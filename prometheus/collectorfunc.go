// Copyright 2014 The Prometheus Authors
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

// CollectorFunc is a convenient way to implement a Prometheus Collector
// without interface boilerplate.
// This implementation is based on prometheus.DescribeByCollect method.
// familiarize yourself to it before using
// checkout https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#DescribeByCollect
type CollectorFunc func(chan<- Metric)

// Collect calls the defined CollectorFunc function with the provided Metrics channel
func (f CollectorFunc) Collect(ch chan<- Metric) {
	f(ch)
}

// Describe sends the descriptor information using DescribeByCollect
func (f CollectorFunc) Describe(ch chan<- *Desc) {
	DescribeByCollect(f, ch)
}
