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
)

const (
	acceptEncodingHeader     = "Accept-Encoding"
	authorization            = "Authorization"
	authorizationHeader      = "WWW-Authenticate"
	authorizationHeaderValue = "Basic"
	contentEncodingHeader    = "Content-Encoding"
	contentTypeHeader        = "Content-Type"
	gzipAcceptEncodingValue  = "gzip"
	gzipContentEncodingValue = "gzip"
	jsonContentType          = "application/json"
	jsonSuffix               = ".json"
)

var (
	abortOnMisuse             bool
	debugRegistration         bool
	useAggressiveSanityChecks bool
)

// container represents a top-level registered metric that encompasses its
// static metadata.
type container struct {
	BaseLabels map[string]string `json:"baseLabels"`
	Docstring  string            `json:"docstring"`
	Metric     Metric            `json:"metric"`
	name       string
}

type registry struct {
	mutex               sync.RWMutex
	signatureContainers map[string]container
}

// Registry is a registrar where metrics are listed.
//
// In most situations, using DefaultRegistry is sufficient versus creating one's
// own.
type Registry interface {
	// Register a metric with a given name.  Name should be globally unique.
	Register(name, docstring string, baseLabels map[string]string, metric Metric) error
	// Create a http.HandlerFunc that is tied to a Registry such that requests
	// against it generate a representation of the housed metrics.
	Handler() http.HandlerFunc
	// This is a legacy version of Handler and is deprecated.  Please stop
	// using.
	YieldExporter() http.HandlerFunc
}

// This builds a new metric registry.  It is not needed in the majority of
// cases.
func NewRegistry() Registry {
	return registry{
		signatureContainers: make(map[string]container),
	}
}

// Associate a Metric with the DefaultRegistry.
func Register(name, docstring string, baseLabels map[string]string, metric Metric) error {
	return DefaultRegistry.Register(name, docstring, baseLabels, metric)
}

// Implements json.Marshaler
func (r registry) MarshalJSON() (_ []byte, err error) {
	metrics := make([]interface{}, 0, len(r.signatureContainers))

	keys := make([]string, 0, len(metrics))
	for key := range r.signatureContainers {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		metrics = append(metrics, r.signatureContainers[key])
	}

	return json.Marshal(metrics)
}

// isValidCandidate returns true if the candidate is acceptable for use.  In the
// event of any apparent incorrect use it will report the problem, invalidate
// the candidate, or outright abort.
func (r registry) isValidCandidate(name string, baseLabels map[string]string) (signature string, err error) {
	if len(name) == 0 {
		err = fmt.Errorf("unnamed metric named with baseLabels %s is invalid", baseLabels)

		if abortOnMisuse {
			panic(err)
		} else if debugRegistration {
			log.Println(err)
		}
	}

	if _, contains := baseLabels[nameLabel]; contains {
		err = fmt.Errorf("metric named %s with baseLabels %s contains reserved label name %s in baseLabels", name, baseLabels, nameLabel)

		if abortOnMisuse {
			panic(err)
		} else if debugRegistration {
			log.Println(err)
		}

		return
	}

	baseLabels[nameLabel] = name
	signature = labelsToSignature(baseLabels)

	if _, contains := r.signatureContainers[signature]; contains {
		err = fmt.Errorf("metric named %s with baseLabels %s is already registered", name, baseLabels)
		if abortOnMisuse {
			panic(err)
		} else if debugRegistration {
			log.Println(err)
		}

		return
	}

	if useAggressiveSanityChecks {
		for _, container := range r.signatureContainers {
			if container.name == name {
				err = fmt.Errorf("metric named %s with baseLabels %s is already registered as %s and risks causing confusion", name, baseLabels, container.BaseLabels)
				if abortOnMisuse {
					panic(err)
				} else if debugRegistration {
					log.Println(err)
				}

				return
			}
		}
	}

	return
}

func (r registry) Register(name, docstring string, baseLabels map[string]string, metric Metric) (err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if baseLabels == nil {
		baseLabels = map[string]string{}
	}

	signature, err := r.isValidCandidate(name, baseLabels)
	if err != nil {
		return
	}

	r.signatureContainers[signature] = container{
		BaseLabels: baseLabels,
		Docstring:  docstring,
		Metric:     metric,
		name:       name,
	}

	return
}

// YieldBasicAuthExporter creates a http.HandlerFunc that is protected by HTTP's
// basic authentication.
func (register registry) YieldBasicAuthExporter(username, password string) http.HandlerFunc {
	// XXX: Work with Daniel to get this removed from the library, as it is really
	//      superfluous and can be much more elegantly accomplished via
	//      delegation.
	exporter := register.YieldExporter()

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

func (registry registry) YieldExporter() http.HandlerFunc {
	log.Println("Registry.YieldExporter is deprecated in favor of Registry.Handler.")

	return registry.Handler()
}

func (registry registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer requestLatencyAccumulator(time.Now())

		requestCount.Increment(nil)
		url := r.URL

		if strings.HasSuffix(url.Path, jsonSuffix) {
			header := w.Header()
			header.Set(ProtocolVersionHeader, APIVersion)
			header.Set(contentTypeHeader, jsonContentType)

			writer := decorateWriter(r, w)

			if closer, ok := writer.(io.Closer); ok {
				defer closer.Close()
			}

			json.NewEncoder(writer).Encode(registry)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func init() {
	flag.BoolVar(&abortOnMisuse, FlagNamespace+"abortonmisuse", false, "abort if a semantic misuse is encountered (bool).")
	flag.BoolVar(&debugRegistration, FlagNamespace+"debugregistration", false, "display information about the metric registration process (bool).")
	flag.BoolVar(&useAggressiveSanityChecks, FlagNamespace+"useaggressivesanitychecks", false, "perform expensive validation of metrics (bool).")
}
