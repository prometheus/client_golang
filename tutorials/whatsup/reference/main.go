// Copyright 2023 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	httppprof "net/http/pprof"
	"os"
	"syscall"
	"time"

	"github.com/bwplotka/tracing-go/tracing"
	"github.com/bwplotka/tracing-go/tracing/exporters/otlp"
	tracinghttp "github.com/bwplotka/tracing-go/tracing/http"
	"github.com/efficientgo/core/errcapture"
	"github.com/efficientgo/core/errors"
	"github.com/oklog/run"
	"github.com/prometheus/common/model"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/tutorials/whatsup/internal"
)

func main() {
	opts, err := internal.ParseOptions(os.Args)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	if err := runMain(opts); err != nil {
		// Use %+v for github.com/efficientgo/core/errors error to print with stack.
		log.Fatalf("Error: %+v", errors.Wrapf(err, "%s", flag.Arg(0)))
	}
}

func runMain(opts internal.Config) (err error) {
	// Create tracer, so the application can be instrumented with traces.
	var exporter tracing.ExporterBuilder
	switch opts.TraceEndpoint {
	case "stdout":
		exporter = tracing.NewWriterExporter(os.Stdout)
	default:
		exporter = otlp.Exporter(opts.TraceEndpoint, otlp.WithInsecure())
	}
	tracer, closeFn, err := tracing.NewTracer(
		exporter,
		tracing.WithSampler(tracing.TraceIDRatioBasedSampler(opts.TraceSamplingRatio)),
		tracing.WithServiceName("client_golang-tutorial:whatsup"),
	)
	if err != nil {
		return err
	}
	defer errcapture.Do(&err, closeFn, "close tracers")

	// Create registry for Prometheus metrics.
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),                                       // Metrics from Go runtime.
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // Metrics about the current UNIX process.
	)

	handled := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Name: "whatsup_queries_handled_total",
		Help: "Number of queries handed.",
	})
	lastNumElems := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Name: "whatsup_last_response_elements",
		Help: "Number of elements in response for the last call.",
	})
	_ = promauto.With(reg).NewGaugeFunc(prometheus.GaugeOpts{
		Name: "build_info",
		Help: "Build information.",
		ConstLabels: map[string]string{
			"version":  "vYOLO",
			"language": "Go 1.20",
			"owner":    "@me",
		},
	}, func() float64 {
		return 1
	})
	handledDuration := promauto.With(reg).NewHistogram(
		prometheus.HistogramOpts{
			Name:    "whatsup_queries_duration_seconds",
			Help:    "Tracks the latencies for calls.",
			Buckets: []float64{0.1, 0.3, 0.6, 1, 3, 6, 9, 20},
		},
	)

	m := http.NewServeMux()
	// Create HTTP handler for Prometheus metrics.
	m.Handle("/metrics", promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics e.g. to support exemplars.
			EnableOpenMetrics: true,
		},
	))

	promClient, err := api.NewClient(api.Config{
		Client:  &http.Client{Transport: instrumentRoundTripper(reg, "prometheus", http.DefaultTransport)},
		Address: opts.PrometheusAddr,
	})
	if err != nil {
		return err
	}
	apiClient := v1.NewAPI(promClient)

	// Create HTTP handler for our whatsup implementation.
	m.HandleFunc(instrumentHandlerFunc(tracer, reg, "/whatsup", whatsUpHandler(
		apiClient,
		handled,
		lastNumElems,
		handledDuration,
	)))

	// Debug profiling endpoints.
	m.HandleFunc("/debug/pprof/", httppprof.Index)
	m.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", httppprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)

	srv := http.Server{Addr: fmt.Sprintf(":%v", internal.WhatsupPort), Handler: m}

	g := &run.Group{}
	g.Add(func() error {
		log.Println("Starting HTTP server", "addr", internal.WhatsupPort)
		if err := srv.ListenAndServe(); err != nil {
			return errors.Wrap(err, "starting web server")
		}
		return nil
	}, func(error) {
		if err := srv.Close(); err != nil {
			log.Println("Error: Failed to stop web server", "err", err)
		}
	})
	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))
	return g.Run()
}

type response struct {
	Error     error `json:",omitempty"`
	Instances []string
}

// whatsUpHandler returns all services that currently monitored by Prometheus.
// It uses prometheus client_golang client code to request PromQL query against given Prometheus server
// to return answer.
func whatsUpHandler(
	apiClient v1.API,
	handled prometheus.Counter,
	lastNumElems prometheus.Gauge,
	handledDuration prometheus.Histogram,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()
		w.Header().Set("Content-Type", "application/json")

		var upResponse model.Value
		if err := tracing.DoInSpan(ctx, "query Prometheus", func(ctx context.Context) error {
			res, warn, err := apiClient.Query(ctx, "up", time.Now())
			if err != nil {
				return err
			}

			if len(warn) > 0 {
				return errors.Newf("got warnings from Prometheus %v", warn)
			}
			upResponse = res
			return nil
		}); err != nil {
			// We return OK status, so browser can render nice result.
			w.WriteHeader(http.StatusOK)
			// NOTE: Pass-through error might be not always safe, sanitize on production.
			b, _ := json.Marshal(response{Error: err})
			_, _ = fmt.Fprintln(w, string(b))
			return
		}

		resp := response{}
		switch r := upResponse.(type) {
		case model.Vector:
			for _, s := range r {
				resp.Instances = append(resp.Instances, string(s.Metric["instance"]))
			}
		}

		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(resp)
		_, _ = fmt.Fprintln(w, string(b))

		lastNumElems.Set(float64(len(resp.Instances)))
		handled.(prometheus.ExemplarAdder).
			AddWithExemplar(1, getExemplarFn(ctx))
		handledDuration.(prometheus.ExemplarObserver).
			ObserveWithExemplar(time.Since(start).Seconds(), getExemplarFn(ctx))
	}
}

func getExemplarFn(ctx context.Context) prometheus.Labels {
	if spanCtx := tracing.GetSpan(ctx); spanCtx.Context().IsSampled() {
		return prometheus.Labels{"traceID": spanCtx.Context().TraceID()}
	}
	return nil
}

func instrumentHandlerFunc(tracer *tracing.Tracer, reg prometheus.Registerer, handlerName string, handler http.Handler) (string, http.HandlerFunc) {
	reg = prometheus.WrapRegistererWith(prometheus.Labels{"handler": handlerName}, reg)

	requestDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Tracks the latencies for HTTP requests.",
			Buckets: []float64{0.1, 0.3, 0.6, 1, 3, 6, 9, 20},
		},
		[]string{"method", "code"},
	)
	requestSize := promauto.With(reg).NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_request_size_bytes",
			Help: "Tracks the size of HTTP requests.",
		},
		[]string{"method", "code"},
	)
	requestsTotal := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Tracks the number of HTTP requests.",
		}, []string{"method", "code"},
	)
	responseSize := promauto.With(reg).NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "http_response_size_bytes",
			Help: "Tracks the size of HTTP responses.",
		},
		[]string{"method", "code"},
	)

	base := promhttp.InstrumentHandlerRequestSize(
		requestSize,
		promhttp.InstrumentHandlerCounter(
			requestsTotal,
			promhttp.InstrumentHandlerResponseSize(
				responseSize,
				promhttp.InstrumentHandlerDuration(
					requestDuration,
					http.HandlerFunc(func(writer http.ResponseWriter, r *http.Request) {
						handler.ServeHTTP(writer, r)
					}),
					promhttp.WithExemplarFromContext(getExemplarFn),
				),
			),
			promhttp.WithExemplarFromContext(getExemplarFn),
		),
	)

	// Wrap with tracing. This will be visited as a first middleware.
	base = tracinghttp.NewMiddleware(tracer).WrapHandler(handlerName, base)
	return handlerName, base.ServeHTTP
}

func instrumentRoundTripper(reg prometheus.Registerer, clientName string, rt http.RoundTripper) http.RoundTripper {
	reg = prometheus.WrapRegistererWith(prometheus.Labels{"client": clientName}, reg)

	requestDuration := promauto.With(reg).NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_client_request_duration_seconds",
			Help:    "Tracks the latencies for HTTP requests.",
			Buckets: []float64{0.1, 0.3, 0.6, 1, 3, 6, 9, 20},
		},
		[]string{"method", "code"},
	)
	requestsTotal := promauto.With(reg).NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_client_requests_total",
			Help: "Tracks the number of HTTP requests.",
		}, []string{"method", "code"},
	)
	responseInflight := promauto.With(reg).NewGauge(
		prometheus.GaugeOpts{
			Name: "http_client_requests_inflight",
			Help: "Tracks the number of client requests currently in progress.",
		},
	)

	var base http.RoundTripper
	base = promhttp.InstrumentRoundTripperCounter(
		requestsTotal,
		promhttp.InstrumentRoundTripperInFlight(
			responseInflight,
			promhttp.InstrumentRoundTripperDuration(requestDuration, rt, promhttp.WithExemplarFromContext(getExemplarFn)),
		),
		promhttp.WithExemplarFromContext(getExemplarFn),
	)

	// Wrap with tracing. This will be visited as a first middleware.
	return tracinghttp.NewTripperware().WrapRoundTipper(clientName, base)
}
