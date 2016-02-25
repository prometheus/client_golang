// Copyright 2015 The Prometheus Authors
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

package registry

import (
	"os"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

// TODO: These are from the old code. Vet!
const (
	// Constants for object pools.
	numBufs           = 4
	numMetricFamilies = 1000
	numMetrics        = 10000

	// Capacity for the channel to collect metrics and descriptors.
	capMetricChan = 1000
	capDescChan   = 10
)

func init() {
	MustRegister(Default, collectors.NewProcessCollector(os.Getpid(), ""))
	MustRegister(Default, collectors.NewGoCollector())
}

// Default is the default registry implicitly used by the top-level functions in
// the prometheus package. It is using the default value of Opts and has a
// ProcessCollector and a GoCollector pre-registered.
var Default Registry = New(Opts{})

func New(opts Opts) Registry {
	return &registry{}
}

type registry struct {
}

func (r *registry) Register(prometheus.Collector) error {
	return nil // TODO
}

func (r *registry) Unregister(prometheus.Collector) bool {
	return false // TODO
}

func (r *registry) Collect(names map[string]struct{}) <-chan struct {
	*dto.MetricFamily
	error
} {
	return nil // TODO
}
