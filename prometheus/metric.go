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
	"bytes"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"sort"
	"strings"
	"code.google.com/p/goprotobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

// Metric models any sort of telemetric data you wish to export to Prometheus.
type Metric interface {
	// Desc returns the descriptor for the Metric. This method idempotently
	// returns the same immutable descriptor throughout the lifetime of the
	// Metric.
	Desc() *Desc
	// Write encodes the Metric into a "Metric" Protocol Buffer data
	// transmission object.
	//
	// Implementers of custom Metric types must observe concurrency safety
	// as reads of this metric may occur at any time, and any blocking
	// occurs at the expense of total performance of rendering all
	// registered metrics.  Ideally Metric implementations should support
	// concurrent readers.
	//
	// The Prometheus client library attempts to minimize memory allocations
	// and will provide a pre-existing reset dto.Metric pointer. Prometheus
	// recycles the returned value, so Metric implementations should not
	// keep any reference to it.  Prometheus will never invoke Write with a
	// value.
	Write(*dto.Metric)
}

// Desc is a the descriptor for all Prometheus metrics. It is essentially the
// immutable meta-data for a metric. (Any mutations to Desc instances will be
// performed internally by the prometheus package. Users will only ever set
// field values at initialization time.)
//
// Upon registration, Prometheus automatically materializes fully-qualified
// metric names by joining Namespace, Subsystem, and Name with "_". It is
// mandatory to provide a non-empty strings for Name and Help. All other fields
// are optional and may be left at their zero values.
//
// Descriptors registered with the same registry have to fulfill certain
// consistency and uniqueness criteria if they share the same fully-qualified
// name. (Take into account that you may end up with the same fully-qualified
// even with different settings for Namespace, Subsystem, and Name.) Descriptors
// that share a fully-qualified name must also have the same Type, the same
// Help, and the same label names (aka label dimensions) in each, PresetLabels
// and VariableLabels, but they must differ in the values of the PresetLabels.
type Desc struct {
	Namespace string
	Subsystem string
	Name      string

	// Help provides some helpful information about this metric.
	Help string

	// PresetLabels are labels with a fixed value.
	PresetLabels map[string]string
	// VariableLabels contains names of labels for which the metric
	// maintains variable values.
	VariableLabels []string

	// The DTO type this metric will encode to (the zero value is
	// MetricType_COUNTER).
	Type dto.MetricType

	// canonName is materialized from Namespace, Subsystem, and Name.
	canonName string
	// id is a hash of the values of the PresetLabels and canonName. This
	// must be unique among all registered descriptors and can therefore be
	// used as an identifier of the descriptor.
	id uint64
	// dimHash is a hash of the label names (preset and variable), the Type
	// and the Help string. Each Desc with the same canonName must have the
	// same dimHash.
	dimHash uint64
	// presetLabelPairs contains precalculated DTO label pairs based on
	// PresetLabels.
	presetLabelPairs []*dto.LabelPair
}

var (
	errEmptyName = errors.New("may not have empty name")
	errEmptyHelp = errors.New("may not have empty help")

	errNoDesc = errors.New("metric collector has no metric descriptors")

	errInconsistentCardinality = errors.New("inconsistent label cardinality")
	errLabelsForSimpleMetric   = errors.New("tried to create a simple metric with variable labels")
	errNoLabelsForVecMetric    = errors.New("tried to create a vector metric without variable labels")

	errEmptyLabelName = errors.New("empty label name")
	errDuplLabelName  = errors.New("duplicate label name")
)

// build is called upon registration. It materializes cannonName and calculates
// the various hashes.
func (d *Desc) build() error {
	if d.Name == "" {
		return errEmptyName
	}
	if d.Help == "" {
		return errEmptyHelp
	}
	switch {
	case d.Namespace != "" && d.Subsystem != "":
		d.canonName = strings.Join([]string{d.Namespace, d.Subsystem, d.Name}, "_")
		break
	case d.Namespace != "":
		d.canonName = strings.Join([]string{d.Namespace, d.Name}, "_")
		break
	case d.Subsystem != "":
		d.canonName = strings.Join([]string{d.Subsystem, d.Name}, "_")
		break
	default:
		d.canonName = d.Name
	}
	// TODO: Check that the resulting canonName and all the label names
	// (preset and variable) are valid identifiers in the Prometheus
	// expression language.

	// labelValues contain the label values of preset labels (in order of
	// their sorted label names) plus the canonName (at position 0).
	labelValues := make([]string, 1, len(d.PresetLabels)+1)
	labelValues[0] = d.canonName
	labelNames := make([]string, 0, len(d.PresetLabels)+len(d.VariableLabels))
	labelNameSet := map[string]struct{}{}
	// First add only the preset label names and sort them...
	for labelName := range d.PresetLabels {
		if labelName == "" {
			return errEmptyLabelName
		}
		labelNames = append(labelNames, labelName)
		labelNameSet[labelName] = struct{}{}
	}
	sort.Strings(labelNames)
	// ... so that we can now add preset label values in the order of their names.
	for _, labelName := range labelNames {
		labelValues = append(labelValues, d.PresetLabels[labelName])
	}
	// Now add the variable label names, but prefix them with something that
	// cannot be in a regular label name. That prevents matching the label
	// dimension with a different mix between preset and variable labels.
	for _, labelName := range d.VariableLabels {
		if labelName == "" {
			return errEmptyLabelName
		}
		labelNames = append(labelNames, "$"+labelName)
		labelNameSet[labelName] = struct{}{}
	}
	if len(labelNames) != len(labelNameSet) {
		return errDuplLabelName
	}
	h := fnv.New64a()
	var b bytes.Buffer // To copy string contents into, avoiding []byte allocations.
	for _, val := range labelValues {
		b.Reset()
		b.WriteString(val)
		h.Write(b.Bytes())
	}
	d.id = h.Sum64()
	// Sort labelNames so that order doesn't matter for the hash.
	sort.Strings(labelNames)
	// Now hash together (in this order) the type, the help string, and the
	// sorted label names.
	h.Reset()
	binary.Write(h, binary.BigEndian, d.Type)
	b.Reset()
	b.WriteString(d.Help)
	h.Write(b.Bytes())
	for _, labelName := range labelNames {
		b.Reset()
		b.WriteString(labelName)
		h.Write(b.Bytes())
	}
	d.dimHash = h.Sum64()

	d.presetLabelPairs = make([]*dto.LabelPair, 0, len(d.PresetLabels))
	for n, v := range d.PresetLabels {
		d.presetLabelPairs = append(d.presetLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(lpSorter(d.presetLabelPairs))

	return nil
}

type lpSorter []*dto.LabelPair

func (s lpSorter) Len() int {
	return len(s)
}

func (s lpSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s lpSorter) Less(i, j int) bool {
	return s[i].GetName() < s[j].GetName()
}

type hashSorter []uint64

func (s hashSorter) Len() int {
	return len(s)
}

func (s hashSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s hashSorter) Less(i, j int) bool {
	return s[i] < s[j]
}
