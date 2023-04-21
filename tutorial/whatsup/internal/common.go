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

package internal

import (
	"flag"
	"os"

	"gopkg.in/yaml.v2"
)

const WhatsupPort = "99"

var (
	WhatsAppFlags = flag.NewFlagSet("whatsapp", flag.ExitOnError)

	prometheusAddr     = WhatsAppFlags.String("prometheus-address", "", "The address of Prometheus to query.")
	traceEndpoint      = WhatsAppFlags.String("trace-endpoint", "", "Optional GRPC OTLP endpoint for tracing backend. Set it to 'stdout' to print traces to the output instead.")
	traceSamplingRatio = WhatsAppFlags.Float64("trace-sampling-ratio", 1.0, "Sampling ratio. Currently 1.0 is the best value to use with exemplars.")
	configFile         = WhatsAppFlags.String("config-file", "./whatsup.yaml", "YAML configuration with same options as flags here. Flags override the configuration items.")
)

type Config struct {
	PrometheusAddr     string  `yaml:"PrometheusAddress,omitempty"`
	TraceEndpoint      string  `yaml:"TraceEndpoint,omitempty"`
	TraceSamplingRatio float64 `yaml:"TraceSamplingRatio,omitempty"`
}

func ParseOptions(args []string) (Config, error) {
	c := Config{}

	if err := WhatsAppFlags.Parse(args); err != nil {
		return c, err
	}

	if *configFile != "" {
		b, err := os.ReadFile(*configFile)
		if err != nil {
			return c, err
		}
		if err := yaml.Unmarshal(b, &c); err != nil {
			return c, err
		}
	}

	if *prometheusAddr != "" {
		c.PrometheusAddr = *prometheusAddr
	}
	if *traceEndpoint != "" {
		c.TraceEndpoint = *traceEndpoint
	}
	if *traceSamplingRatio > 0 {
		c.TraceSamplingRatio = *traceSamplingRatio
	}
	return c, nil
}
