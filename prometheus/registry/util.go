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

import "github.com/prometheus/client_golang/prometheus/metric"

func MustRegister(r Registry, c metric.Collector) {
	if err := r.Register(c); err != nil {
		panic(err)
	}
}

func RegisterOrGet(r Registry, c metric.Collector) (metric.Collector, error) {
	if err := r.Register(c); err != nil {
		if are, ok := err.(AlreadyRegisteredError); ok {
			return are.ExistingCollector, nil
		}
		return nil, err
	}
	return c, nil
}

func MustRegisterOrGet(r Registry, c metric.Collector) metric.Collector {
	existing, err := RegisterOrGet(r, c)
	if err != nil {
		panic(err)
	}
	return existing
}
