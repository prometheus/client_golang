// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package prometheus

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	dto "github.com/prometheus/client_model/go"

	"code.google.com/p/goprotobuf/proto"

	"github.com/prometheus/client_golang/model"
	"github.com/prometheus/client_golang/text"
	"github.com/prometheus/client_golang/vendor/goautoneg"
)

const (
	authorization            = "Authorization"
	authorizationHeader      = "WWW-Authenticate"
	authorizationHeaderValue = "Basic"

	acceptEncodingHeader     = "Accept-Encoding"
	contentEncodingHeader    = "Content-Encoding"
	contentTypeHeader        = "Content-Type"
	gzipAcceptEncodingValue  = "gzip"
	gzipContentEncodingValue = "gzip"
	jsonContentType          = "application/json"
)

// encoder is a function that writes a dto.MetricFamily to an io.Writer in a
// certain encoding. It returns the number of bytes written and any error
// encountered.  Note that ext.WriteDelimited and text.MetricFamilyToText are
// encoders.
type encoder func(io.Writer, *dto.MetricFamily) (int, error)

// container represents a top-level registered metric that encompasses its
// static metadata.
type container struct {
	BaseLabels map[string]string `json:"baseLabels"`
	Docstring  string            `json:"docstring"`
	Metric     Metric            `json:"metric"`
	name       string
}

type containers []*container

func (c containers) Len() int {
	return len(c)
}

func (c containers) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c containers) Less(i, j int) bool {
	return c[i].name < c[j].name
}

type registry struct {
	mutex                     sync.RWMutex
	signatureContainers       map[uint64]*container
	metricFamilyInjectionHook func() []*dto.MetricFamily
}

// Registry is a registrar where metrics are listed.
//
// In most situations, using DefaultRegistry is sufficient versus creating one's
// own.
type Registry interface {
	// Register a metric with a given name.  Name should be globally unique.
	Register(name, docstring string, baseLabels map[string]string, metric Metric) error
	// SetMetricFamilyInjectionHook sets a function that is called whenever
	// metrics are requested. The MetricsFamily protobufs returned by the
	// function are appended to the delivered metrics. This is a way to
	// directly inject MetricFamily protobufs managed and owned by the
	// caller. The caller has full responsibility. No sanity checks are
	// performed on the returned protobufs. The function must be callable at
	// any time and concurrently. The only thing handled by the Registry is
	// the conversion if metrics are requested in a non-protobuf format. The
	// deprecated JSON format, however, is not supported, i.e. metrics
	// delivered as JSON will not contain the metrics injected by the
	// injection hook.
	SetMetricFamilyInjectionHook(func() []*dto.MetricFamily)
	// Handler creates a http.HandlerFunc. Requests against it generate a
	// representation of the metrics managed by this registry.
	Handler() http.HandlerFunc
	// YieldExporter is a legacy version of Handler and is deprecated.
	// Please stop using.
	YieldExporter() http.HandlerFunc
}

// NewRegistry builds a new metric registry.  It is not needed in the majority
// of cases.
func NewRegistry() Registry {
	return &registry{
		signatureContainers: make(map[uint64]*container),
	}
}

// Register associates a Metric with the DefaultRegistry.
func Register(name, docstring string, baseLabels map[string]string, metric Metric) error {
	return DefaultRegistry.Register(name, docstring, baseLabels, metric)
}

// SetMetricFamilyInjectionHook implements the Registry interface.
func (r *registry) SetMetricFamilyInjectionHook(hook func() []*dto.MetricFamily) {
	r.metricFamilyInjectionHook = hook
}

// MarshalJSON implements json.Marshaler.
func (r *registry) MarshalJSON() ([]byte, error) {
	containers := make(containers, 0, len(r.signatureContainers))

	for _, container := range r.signatureContainers {
		containers = append(containers, container)
	}

	sort.Sort(containers)

	return json.Marshal(containers)
}

// isValidCandidate returns true if the candidate is acceptable for use.  In the
// event of any apparent incorrect use it will report the problem, invalidate
// the candidate, or outright abort.
func (r *registry) isValidCandidate(name string, baseLabels map[string]string) (signature uint64, err error) {
	if len(name) == 0 {
		err = fmt.Errorf("unnamed metric named with baseLabels %s is invalid", baseLabels)

		if *abortOnMisuse {
			panic(err)
		} else if *debugRegistration {
			log.Println(err)
		}
	}

	for label := range baseLabels {
		if strings.HasPrefix(label, model.ReservedLabelPrefix) {
			err = fmt.Errorf("metric named %s with baseLabels %s contains reserved label name %s in baseLabels", name, baseLabels, label)

			if *abortOnMisuse {
				panic(err)
			} else if *debugRegistration {
				log.Println(err)
			}

			return signature, err
		}
	}

	baseLabels[string(model.MetricNameLabel)] = name
	signature = model.LabelsToSignature(baseLabels)

	if _, contains := r.signatureContainers[signature]; contains {
		err = fmt.Errorf("metric named %s with baseLabels %s is already registered", name, baseLabels)
		if *abortOnMisuse {
			panic(err)
		} else if *debugRegistration {
			log.Println(err)
		}

		return signature, err
	}

	if *useAggressiveSanityChecks {
		for _, container := range r.signatureContainers {
			if container.name == name {
				err = fmt.Errorf("metric named %s with baseLabels %s is already registered as %s and risks causing confusion", name, baseLabels, container.BaseLabels)
				if *abortOnMisuse {
					panic(err)
				} else if *debugRegistration {
					log.Println(err)
				}

				return signature, err
			}
		}
	}

	return signature, err
}

func (r *registry) Register(name, docstring string, baseLabels map[string]string, metric Metric) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	labels := map[string]string{}

	if baseLabels != nil {
		for k, v := range baseLabels {
			labels[k] = v
		}
	}

	signature, err := r.isValidCandidate(name, labels)
	if err != nil {
		return err
	}

	r.signatureContainers[signature] = &container{
		BaseLabels: labels,
		Docstring:  docstring,
		Metric:     metric,
		name:       name,
	}

	return nil
}

// YieldBasicAuthExporter creates a http.HandlerFunc that is protected by HTTP's
// basic authentication.
func (r *registry) YieldBasicAuthExporter(username, password string) http.HandlerFunc {
	// XXX: Work with Daniel to get this removed from the library, as it is really
	//      superfluous and can be much more elegantly accomplished via
	//      delegation.
	log.Println("Registry.YieldBasicAuthExporter is deprecated.")
	exporter := r.YieldExporter()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated := false

		if auth := r.Header.Get(authorization); auth != "" {
			base64Encoded := strings.SplitAfter(auth, " ")[1]
			decoded, err := base64.URLEncoding.DecodeString(base64Encoded)
			if err == nil {
				usernamePassword := strings.Split(string(decoded), ":")
				if usernamePassword[0] == username && usernamePassword[1] == password {
					authenticated = true
				}
			}
		}

		if authenticated {
			exporter.ServeHTTP(w, r)
		} else {
			w.Header().Add(authorizationHeader, authorizationHeaderValue)
			http.Error(w, "access forbidden", 401)
		}
	})
}

// decorateWriter annotates the response writer to handle any other behaviors
// that might be beneficial to the client---e.g., GZIP encoding.
func decorateWriter(request *http.Request, writer http.ResponseWriter) io.Writer {
	if !strings.Contains(request.Header.Get(acceptEncodingHeader), gzipAcceptEncodingValue) {
		return writer
	}

	writer.Header().Set(contentEncodingHeader, gzipContentEncodingValue)
	gziper := gzip.NewWriter(writer)

	return gziper
}

func (r *registry) YieldExporter() http.HandlerFunc {
	log.Println("Registry.YieldExporter is deprecated in favor of Registry.Handler.")

	return r.Handler()
}

func (r *registry) dumpPB(w io.Writer, writeEncoded encoder) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	f := new(dto.MetricFamily)
	for _, container := range r.signatureContainers {
		f.Reset()

		f.Name = proto.String(container.name)
		f.Help = proto.String(container.Docstring)

		container.Metric.dumpChildren(f)

		for name, value := range container.BaseLabels {
			if model.LabelName(name) == model.MetricNameLabel {
				// The name is already in MetricFamily.
				continue
				// TODO: Once JSON is history, do not anymore
				// add the __name__ label to BaseLabels and
				// then remove this check.
			}
			p := &dto.LabelPair{
				Name:  proto.String(name),
				Value: proto.String(value),
			}

			for _, child := range f.Metric {
				child.Label = append(child.Label, p)
			}
		}

		writeEncoded(w, f)
	}
}

func (r *registry) dumpExternalPB(w io.Writer, writeEncoded encoder) {
	if r.metricFamilyInjectionHook == nil {
		return
	}
	for _, f := range r.metricFamilyInjectionHook() {
		writeEncoded(w, f)
	}
}

func (r *registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer requestLatencyAccumulator(time.Now())

		requestCount.Increment(nil)
		header := w.Header()

		writer := decorateWriter(req, w)

		if closer, ok := writer.(io.Closer); ok {
			defer closer.Close()
		}

		accepts := goautoneg.ParseAccept(req.Header.Get("Accept"))
		for _, accept := range accepts {
			var enc encoder
			switch {
			case accept.Type == "application" &&
				accept.SubType == "vnd.google.protobuf" &&
				accept.Params["proto"] == "io.prometheus.client.MetricFamily":
				switch accept.Params["encoding"] {
				case "delimited":
					header.Set(contentTypeHeader, DelimitedTelemetryContentType)
					enc = text.WriteProtoDelimited
				case "text":
					header.Set(contentTypeHeader, ProtoTextTelemetryContentType)
					enc = text.WriteProtoText
				case "compact-text":
					header.Set(contentTypeHeader, ProtoCompactTextTelemetryContentType)
					enc = text.WriteProtoCompactText
				default:
					continue
				}
			case accept.Type == "text" &&
				accept.SubType == "plain" &&
				(accept.Params["version"] == "0.0.4" || accept.Params["version"] == ""):
				header.Set(contentTypeHeader, TextTelemetryContentType)
				enc = text.MetricFamilyToText
			default:
				continue
			}
			r.dumpPB(writer, enc)
			r.dumpExternalPB(writer, enc)
			return
		}
		// TODO: Once JSON deprecation is completed, use text format as
		// fall-back.
		header.Set(contentTypeHeader, JSONTelemetryContentType)
		json.NewEncoder(writer).Encode(r)
	}
}

var (
	abortOnMisuse             = flag.Bool(FlagNamespace+"abortonmisuse", false, "abort if a semantic misuse is encountered (bool).")
	debugRegistration         = flag.Bool(FlagNamespace+"debugregistration", false, "display information about the metric registration process (bool).")
	useAggressiveSanityChecks = flag.Bool(FlagNamespace+"useaggressivesanitychecks", false, "perform expensive validation of metrics (bool).")
)
