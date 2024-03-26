// Copyright 2022 The Prometheus Authors
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
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/common/expfmt"
)

func BenchmarkSort(b *testing.B) {
	var parser expfmt.TextParser
	text := `
	# HELP node_cpu_guest_seconds_total Seconds the CPUs spent in guests (VMs) for each mode.
	# TYPE node_cpu_guest_seconds_total counter
	node_cpu_guest_seconds_total{cpu="0",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="0",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="1",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="1",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="10",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="10",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="11",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="11",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="12",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="12",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="13",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="13",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="14",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="14",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="15",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="15",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="2",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="2",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="3",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="3",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="4",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="4",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="5",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="5",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="6",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="6",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="7",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="7",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="8",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="8",mode="user"} 0
	node_cpu_guest_seconds_total{cpu="9",mode="nice"} 0
	node_cpu_guest_seconds_total{cpu="9",mode="user"} 0
	`
	notNormalized, _ := parser.TextToMetricFamilies(strings.NewReader(text))
	for i := 0; i < b.N; i++ {
		for _, mf := range notNormalized {
			sort.Sort(MetricSorter(mf.Metric))
		}
	}
}
