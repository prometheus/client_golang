// Copyright 2024 The Prometheus Authors
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

//go:build go1.21
// +build go1.21

package prometheus

func goRuntimeEnvVarsMetrics() runtimeEnvVarsMetrics {
	return runtimeEnvVarsMetrics{
		{
			desc: NewDesc(
				"go_gogc_percent",
				"Value of GOGC (percentage).",
				nil, nil,
			),
			origMetricName: "/gc/gogc:percent",
		},
		{
			desc: NewDesc(
				"go_gomemlimit",
				"Value of GOMEMLIMIT (bytes).",
				nil, nil,
			),
			origMetricName: "/gc/gomemlimit:bytes",
		},
		{
			desc: NewDesc(
				"go_gomaxprocs",
				"Value of GOMAXPROCS, i.e number of usable threads.",
				nil, nil,
			),
			origMetricName: "/sched/gomaxprocs:threads",
		},
	}
}
