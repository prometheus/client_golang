// Copyright 2014 Prometheus Team
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

package prometheus

import (
	"encoding/json"
	"expvar"
	"fmt"
	"sort"

	dto "github.com/prometheus/client_model/go"
)

// ExpvarCollector collects metrics from the expvar interface. It provides a
// quick way to expose numeric values that are already exported via expvar as
// Prometheus metrics. Note that the data models of expvar and Prometheus are
// fundamentally different, and that the ExpvarCollector is inherently
// slow. Thus, the ExvarCollector is probably great for experiments and
// prototying, but you should seriously consider a more direct implementation of
// Prometheus metrics for monitoring a production systems.
type ExpvarCollector struct {
	exports map[string]*Desc
	descs   []*Desc
}

// NewExpvarCollector returns a newly allocated ExpvarCollector that still has
// to be registered with the Prometheus registry.
//
// The exports map has the following meaning:
//
// The keys in the map correspond to expvar keys, i.e. for every expvar key you
// want to export as Prometheus metric, you need an entry in the exports
// map. The descriptor mapped to each key describes how to export the expvar
// value. It defines name, help string, and type of the Prometheus metric
// proxying the expvar value. The type can be anything but a summary.
//
// For descriptors without variable labels, the expvar value must be a number or
// a bool. The number is then directly exported as the Prometheus sample
// value. (For a bool, 'false' translates to 0 and 'true' to 1). Expvar values
// that are not numbers or bools are silently ignored.
//
// If the descriptor has one variable label, the expvar value must be an expvar
// map. The keys in the expvar map become the various values of the one
// Prometheus label. The values in the expvar map must be numbers or bools again
// as above.
//
// For descriptors with more than one variable lable, the expvar must be a
// nested expvar map, i.e. where the values of the topmost map are maps again
// etc. until a depth is reached that corresponds to the number of labels. The
// leaves of that structure must be numbers or bools as above to serve as the
// sample values.
//
// Example:
//
// expvar exports the following map:
// "http-request-count": {"200": {"POST": 11, "GET": 212}, "404": {"POST": 3, "GET": 13}}
//
// The following descriptor would be suitable to convert that expvar map into a
// Prometheus metric:
// desc := &prometheus.Desc{
//     Name: "http_request_count",
//     Help: "Number of HTTP requests.",
//     Type:  dto.MetricType_COUNTER,
//     VariableLabels: []string{"code", "method"},
// }
//
// Then call the function like this:
// expvarColl, err := prometheus.NewExpvarCollector(map[string]*prometheus.Desc{"http-request-count": desc})
//
// Finally register the new collector:
// _, err := prometheus.Register(expvarColl)
func NewExpvarCollector(exports map[string]*Desc) (*ExpvarCollector, error) {
	descs := make([]*Desc, 0, len(exports))
	for _, desc := range exports {
		if desc.Type == dto.MetricType_SUMMARY {
			return nil, fmt.Errorf("descriptor %+v contains type summary", desc)
		}
		descs = append(descs, desc)
	}
	return &ExpvarCollector{
		exports: exports,
		descs:   descs,
	}, nil
}

// MustNewExpvarCollector is a version of NewExpvarCollector that panics where
// NewExpvarCollector would have returned an error.
func MustNewExpvarCollector(exports map[string]*Desc) *ExpvarCollector {
	e, err := NewExpvarCollector(exports)
	if err != nil {
		panic(err)
	}
	return e
}

func (e *ExpvarCollector) DescribeMetrics() []*Desc {
	return e.descs
}

func (e *ExpvarCollector) CollectMetrics() []Metric {
	metrics := make([]Metric, 0, len(e.exports))
	for name, desc := range e.exports {
		var m Metric
		expVar := expvar.Get(name)
		if expVar == nil {
			continue
		}
		var v interface{}
		labels := make([]string, len(desc.VariableLabels))
		if err := json.Unmarshal([]byte(expVar.String()), &v); err == nil {

			var processValue func(v interface{}, i int)
			processValue = func(v interface{}, i int) {
				if i >= len(labels) {
					dims := append(make([]string, 0, len(labels)), labels...)
					switch v := v.(type) {
					case float64:
						m = MustNewConstMetric(desc, v, dims...)
					case bool:
						if v {
							m = MustNewConstMetric(desc, 1, dims...)
						} else {
							m = MustNewConstMetric(desc, 0, dims...)
						}
					default:
						return
					}
					metrics = append(metrics, m)
					return
				}
				vm, ok := v.(map[string]interface{})
				if !ok {
					return
				}
				lvs := make([]string, 0, len(vm))
				for lv := range vm {
					lvs = append(lvs, lv)
				}
				sort.Strings(lvs)
				for _, lv := range lvs {
					labels[i] = lv
					processValue(vm[lv], i+1)
				}
			}

			processValue(v, 0)
		}
	}
	return metrics
}
