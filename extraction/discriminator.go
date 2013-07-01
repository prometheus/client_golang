// Copyright 2013 Prometheus Team
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

package extraction

import (
	"fmt"
	"mime"
	"net/http"
)

// ProcessorForRequestHeader interprets a HTTP request header to determine
// what Processor should be used for the given input.  If no acceptable
// Processor can be found, an error is returned.
func ProcessorForRequestHeader(header http.Header) (Processor, error) {
	if header == nil {
		return nil, fmt.Errorf("Received illegal and nil header.")
	}

	mediatype, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("Invalid Content-Type header %q: %s", header.Get("Content-Type"), err)
	}
	switch mediatype {
	case "application/vnd.google.protobuf":
		if params["proto"] != "io.prometheus.client.MetricFamily" {
			return nil, fmt.Errorf("Unrecognized Protocol Message %s", params["proto"])
		}
		if params["encoding"] != "delimited" {
			return nil, fmt.Errorf("Unsupported Encoding %s", params["encoding"])
		}
		return MetricFamilyProcessor, nil

	case "application/json":
		var prometheusApiVersion string

		if params["schema"] == "prometheus/telemetry" && params["version"] != "" {
			prometheusApiVersion = params["version"]
		} else {
			prometheusApiVersion = header.Get("X-Prometheus-API-Version")
		}

		switch prometheusApiVersion {
		case "0.0.2":
			return Processor002, nil
		case "0.0.1":
			return Processor001, nil
		default:
			return nil, fmt.Errorf("Unrecognized API version %s", prometheusApiVersion)
		}
	default:
		return nil, fmt.Errorf("Unsupported media type %q, expected %q", mediatype, "application/json")
	}
}
