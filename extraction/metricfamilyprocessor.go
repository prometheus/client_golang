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
	"io"
)

type metricFamilyProcessor struct{}

// MetricFamilyProcessor decodes varint encoded record length-delimited streams
// of io.prometheus.client.MetricFamily.
//
// See http://godoc.org/github.com/matttproud/golang_protobuf_extensions/ext for
// more details.
var MetricFamilyProcessor = new(metricFamilyProcessor)

func (m *metricFamilyProcessor) ProcessSingle(io.Reader, chan<- *Result, *ProcessOptions) error {
	panic("not implemented")
}
