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

//go:build interactive
// +build interactive

package internal

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"github.com/efficientgo/e2e"
	e2edb "github.com/efficientgo/e2e/db"
	e2einteractive "github.com/efficientgo/e2e/interactive"
	e2emon "github.com/efficientgo/e2e/monitoring"
	"github.com/efficientgo/e2e/monitoring/promconfig"
	sdconfig "github.com/efficientgo/e2e/monitoring/promconfig/discovery/config"
	"github.com/efficientgo/e2e/monitoring/promconfig/discovery/targetgroup"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
)

func TestPlayground(t *testing.T) {
	// NOTE: Only one run at the time will work due to static ports used.

	e, err := e2e.New(e2e.WithVerbose())
	testutil.Ok(t, err)
	t.Cleanup(e.Close)

	// Setup in-memory Jaeger as our tracing backend.
	jaeger := e2emon.AsInstrumented(e.Runnable("tracing").
		WithPorts(
			map[string]int{
				"http.front":   16686,
				"grpc.otlp":    4317,
				"http.metrics": 14269,
			}).
		Init(e2e.StartOptions{
			Image:   "jaegertracing/all-in-one:1.35",
			EnvVars: map[string]string{"COLLECTOR_OTLP_ENABLED": "true"},
		}), "http.metrics")
	testutil.Ok(t, e2e.StartAndWaitReady(jaeger))

	prom := e2edb.NewPrometheus(e, "prometheus", e2edb.WithImage("prom/prometheus:v2.43.0-stringlabels"))
	testutil.Ok(t, e2e.StartAndWaitReady(prom))

	// Write config file for whatsup app.
	c := Config{
		PrometheusAddr:     "http://" + prom.Endpoint("http"),
		TraceEndpoint:      jaeger.Endpoint("grpc.otlp"),
		TraceSamplingRatio: 1,
	}
	b, err := yaml.Marshal(c)
	testutil.Ok(t, err)
	testutil.Ok(t, os.WriteFile("../whatsup.yaml", b, os.ModePerm))

	testutil.Ok(t, prom.SetConfig(prometheusConfig(map[string]string{
		"prometheus": prom.InternalEndpoint("http"),
		"jaeger":     jaeger.InternalEndpoint("http.metrics"),
		"whatsup":    whatsupAddr(fmt.Sprintf("host.docker.internal:%v", WhatsupPort)),
	})))
	// Due to VM based docker setups (e.g. MacOS), file sharing can be slower - do more sighups just in case (noops if all good)
	prom.Exec(e2e.NewCommand("kill", "-SIGHUP", "1"))
	prom.Exec(e2e.NewCommand("kill", "-SIGHUP", "1"))

	// Best effort.
	fmt.Println(e2einteractive.OpenInBrowser(convertToExternal("http://" + jaeger.Endpoint("http.front"))))
	fmt.Println(e2einteractive.OpenInBrowser(convertToExternal("http://" + prom.Endpoint("http"))))
	testutil.Ok(t, e2einteractive.RunUntilEndpointHitWithPort(19920))
}

func convertToExternal(endpoint string) string {
	a := os.Getenv("HOSTADDR")
	if a == "" {
		return endpoint
	}
	// YOLO, fix and test.
	return fmt.Sprintf("%v:%v", a, strings.Split(endpoint, ":")[2])
}

func prometheusConfig(jobToScrapeTargetAddress map[string]string) promconfig.Config {
	h, _ := os.Hostname()
	cfg := promconfig.Config{
		GlobalConfig: promconfig.GlobalConfig{
			ExternalLabels: map[model.LabelName]model.LabelValue{"prometheus": model.LabelValue(h)},
			ScrapeInterval: model.Duration(15 * time.Second),
		},
	}

	for job, s := range jobToScrapeTargetAddress {
		scfg := &promconfig.ScrapeConfig{
			JobName:                job,
			ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{},
		}

		g := &targetgroup.Group{
			Targets: []model.LabelSet{map[model.LabelName]model.LabelValue{
				model.AddressLabel: model.LabelValue(s),
			}},
		}
		scfg.ServiceDiscoveryConfig.StaticConfigs = append(scfg.ServiceDiscoveryConfig.StaticConfigs, g)
		cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, scfg)
	}
	return cfg
}
