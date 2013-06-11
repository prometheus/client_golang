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
	"time"

	"github.com/prometheus/client_golang/model"
)

const (
	// The label name prefix to prepend if a synthetic label is already present
	// in the exported metrics.
	ExporterLabelPrefix model.LabelName = "exporter_"

	// The label name indicating the metric name of a timeseries.
	MetricNameLabel = "name"

	// The label name indicating the job from which a timeseries was scraped.
	JobLabel = "job"
)

// ProcessOptions dictates how the interpreted stream should be rendered for
// consumption.
type ProcessOptions struct {
	// Timestamp is added to each value interpreted from the stream.
	Timestamp time.Time

	// BaseLabels are labels that are accumulated onto each sample, if any.
	BaseLabels model.LabelSet
}

// Processor is responsible for decoding the actual message responses from
// stream into a format that can be consumed with the end result written
// to the results channel.
type Processor interface {
	// ProcessSingle treats the input as a single self-contained message body and
	// transforms it accordingly.  It has no support for streaming.
	ProcessSingle(in io.Reader, out chan<- *Result, o *ProcessOptions) error
}

// Helper function to convert map[string]string into LabelSet.
//
// NOTE: This should be deleted when support for go 1.0.3 is removed; 1.1 is
//       smart enough to unmarshal JSON objects into LabelSet directly.
func labelSet(labels map[string]string) model.LabelSet {
	labelset := make(model.LabelSet, len(labels))

	for k, v := range labels {
		labelset[model.LabelName(k)] = model.LabelValue(v)
	}

	return labelset
}

// Helper function to merge a target's base labels ontop of the labels of an
// exported sample. If a label is already defined in the exported sample, we
// assume that we are scraping an intermediate exporter and attach
// "exporter_"-prefixes to Prometheus' own base labels.
func mergeTargetLabels(entityLabels, targetLabels model.LabelSet) model.LabelSet {
	if targetLabels == nil {
		targetLabels = model.LabelSet{}
	}

	result := model.LabelSet{}

	for label, value := range entityLabels {
		result[label] = value
	}

	for label, labelValue := range targetLabels {
		if _, exists := result[label]; exists {
			result[ExporterLabelPrefix+label] = labelValue
		} else {
			result[label] = labelValue
		}
	}
	return result
}

// Result encapsulates the outcome from processing samples from a source.
type Result struct {
	Err     error
	Samples model.Samples
}

// A basic interface only useful in testing contexts for dispensing the time
// in a controlled manner.
type instantProvider interface {
	// The current instant.
	Now() time.Time
}

// Clock is a simple means for fluently wrapping around standard Go timekeeping
// mechanisms to enhance testability without compromising code readability.
//
// It is sufficient for use on bare initialization.  A provider should be
// set only for test contexts.  When not provided, it emits the current
// system time.
type clock struct {
	// The underlying means through which time is provided, if supplied.
	Provider instantProvider
}

// Emit the current instant.
func (t *clock) Now() time.Time {
	if t.Provider == nil {
		return time.Now()
	}

	return t.Provider.Now()
}
