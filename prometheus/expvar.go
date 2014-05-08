// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
// TODO: Describe usage. No summaries! Maps!
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
						m, _ = NewStaticMetric(desc, v, dims...)
					case bool:
						if v {
							m, _ = NewStaticMetric(desc, 1, dims...)
						} else {
							m, _ = NewStaticMetric(desc, 0, dims...)
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
