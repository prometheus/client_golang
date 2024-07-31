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
	"flag"
	"log"
	"net/http"
	httppprof "net/http/pprof"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/efficientgo/core/errors"
	"github.com/oklog/run"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	addr = flag.String("listen-address", ":9999", "The address to listen on for HTTP requests.")
)

func main() {
	flag.Parse()

	if err := runMain(*addr); err != nil {
		// Use %+v for github.com/efficientgo/core/errors error to print with stack.
		log.Fatalf("Error: %+v", errors.Wrapf(err, "%s", flag.Arg(0)))
	}
}

func runMain(addr string) (err error) {
	// Create registry for Prometheus metrics.
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics( // Metrics from Go runtime.
			collectors.GoRuntimeMetricsRule{
				Matcher: regexp.MustCompile("/sched/latencies:seconds"), // One more recommended metric on top of the default.
			},
		)),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // Metrics about the current UNIX process.
	)

	m := http.NewServeMux()

	// Create HTTP handler for Prometheus metrics.
	m.Handle("/metrics", promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	))

	// Debug profiling endpoints.
	m.HandleFunc("/debug/pprof/", httppprof.Index)
	m.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
	m.HandleFunc("/debug/pprof/profile", httppprof.Profile)
	m.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)

	s := &scenarios{}
	m.HandleFunc("/break/", func(w http.ResponseWriter, r *http.Request) {
		if err := s.SetFromParam(strings.TrimPrefix(r.URL.Path, "/break/"), true); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	m.HandleFunc("/fix/", func(w http.ResponseWriter, r *http.Request) {
		if err := s.SetFromParam(strings.TrimPrefix(r.URL.Path, "/fix/"), false); err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	srv := http.Server{Addr: addr, Handler: m}
	g := &run.Group{}
	{
		g.Add(func() error {
			log.Println("Starting HTTP server", "addr", addr)
			if err := srv.ListenAndServe(); err != nil {
				return errors.Wrap(err, "starting web server")
			}
			return nil
		}, func(error) {
			if err := srv.Close(); err != nil {
				log.Println("Error: Failed to stop web server", "err", err)
			}
		})
	}
	g.Add(run.SignalHandler(context.Background(), syscall.SIGINT, syscall.SIGTERM))
	addContextNotCanceledGroup(g, reg, func() bool { return s.IsEnabled(contextNotCanceled) })
	addGoroutineJumpGroup(g, func() bool { return s.IsEnabled(goroutineJump) })
	return g.Run()
}

func doOp(ctx context.Context) int64 {
	wg := sync.WaitGroup{}
	wg.Add(10)
	var sum int64
	for i := 0; i < 10; i++ {
		atomic.StoreInt64(&sum, int64(fib(ctx, 1e5)))
		wg.Done()
	}
	wg.Wait()
	return sum
}

func fib(ctx context.Context, n int) int {
	if n <= 1 {
		return n
	}
	var n2, n1 = 0, 1
	for i := 2; i <= n; i++ {
		if ctx.Err() != nil {
			return -1
		}
		n2, n1 = n1, n1+n2
	}
	return n1
}
