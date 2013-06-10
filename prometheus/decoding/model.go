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

package decoding

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"
)

type Sample struct {
	Metric    Metric
	Value     SampleValue
	Timestamp time.Time
}

func (s *Sample) Equal(o *Sample) bool {
	if !s.Metric.Equal(o.Metric) {
		return false
	}
	if !s.Timestamp.Equal(o.Timestamp) {
		return false
	}
	if !s.Value.Equal(o.Value) {
		return false
	}

	return true
}

type Samples []*Sample

func (s Samples) Len() int {
	return len(s)
}

func (s Samples) Less(i, j int) bool {
	switch {
	case s[i].Metric.Before(s[j].Metric):
		return true
	case s[i].Timestamp.Before(s[j].Timestamp):
		return true
	default:
		return false
	}
}

func (s Samples) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// A LabelSet is a collection of LabelName and LabelValue pairs.  The LabelSet
// may be fully-qualified down to the point where it may resolve to a single
// Metric in the data store or not.  All operations that occur within the realm
// of a LabelSet can emit a vector of Metric entities to which the LabelSet may
// match.
type LabelSet map[LabelName]LabelValue

// Helper function to non-destructively merge two label sets.
func (l LabelSet) Merge(other LabelSet) LabelSet {
	result := make(LabelSet, len(l))

	for k, v := range l {
		result[k] = v
	}

	for k, v := range other {
		result[k] = v
	}

	return result
}

func (l LabelSet) String() string {
	labelStrings := make([]string, 0, len(l))
	for label, value := range l {
		labelStrings = append(labelStrings, fmt.Sprintf("%s='%s'", label, value))
	}

	sort.Strings(labelStrings)

	return fmt.Sprintf("{%s}", strings.Join(labelStrings, ", "))
}

// A LabelValue is an associated value for a LabelName.
type LabelValue string

// A Metric is similar to a LabelSet, but the key difference is that a Metric is
// a singleton and refers to one and only one stream of samples.
type Metric map[LabelName]LabelValue

func (m Metric) Equal(o Metric) bool {
	lFingerprint := &Fingerprint{}
	rFingerprint := &Fingerprint{}

	m.WriteFingerprint(lFingerprint)
	o.WriteFingerprint(rFingerprint)

	return lFingerprint.Equal(rFingerprint)
}

func (m Metric) Before(o Metric) bool {
	lFingerprint := &Fingerprint{}
	rFingerprint := &Fingerprint{}

	m.WriteFingerprint(lFingerprint)
	o.WriteFingerprint(rFingerprint)

	return m.Before(o)
}

func (m Metric) WriteFingerprint(f *Fingerprint) {
	labelLength := len(m)
	labelNames := make([]string, 0, labelLength)

	for labelName := range m {
		labelNames = append(labelNames, string(labelName))
	}

	sort.Strings(labelNames)

	summer := fnv.New64a()
	firstCharacterOfFirstLabelName := ""
	lastCharacterOfLastLabelValue := ""
	labelMatterLength := 0

	for i, labelName := range labelNames {
		labelValue := m[LabelName(labelName)]
		labelNameLength := len(labelName)
		labelValueLength := len(labelValue)
		labelMatterLength += labelNameLength + labelValueLength

		if i == 0 {
			firstCharacterOfFirstLabelName = labelName[0:1]
		}
		if i == labelLength-1 {
			lastCharacterOfLastLabelValue = string(labelValue[labelValueLength-1 : labelValueLength])
		}

		fmt.Fprintf(summer, "%s%s%s", labelName, `"`, labelValue)
	}

	f.firstCharacterOfFirstLabelName = firstCharacterOfFirstLabelName
	f.hash = binary.LittleEndian.Uint64(summer.Sum(nil))
	f.labelMatterLength = uint(labelMatterLength % 10)
	f.lastCharacterOfLastLabelValue = lastCharacterOfLastLabelValue

}

// A SampleValue is a representation of a value for a given sample at a given
// time.
type SampleValue float64

func (v SampleValue) Equal(o SampleValue) bool {
	return v == o
}

func (v SampleValue) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%f"`, v)), nil
}

func (v SampleValue) String() string {
	return fmt.Sprint(float64(v))
}

// Fingerprint provides a hash-capable representation of a Metric.
type Fingerprint struct {
	// A hashed representation of the underyling entity.  For our purposes, FNV-1A
	// 64-bit is used.
	hash                           uint64
	firstCharacterOfFirstLabelName string
	labelMatterLength              uint
	lastCharacterOfLastLabelValue  string
}

func (f *Fingerprint) String() string {
	return f.ToRowKey()
}

// Transforms the Fingerprint into a database row key.
func (f *Fingerprint) ToRowKey() string {
	return strings.Join([]string{fmt.Sprintf("%020d", f.hash), f.firstCharacterOfFirstLabelName, fmt.Sprint(f.labelMatterLength), f.lastCharacterOfLastLabelValue}, "-")
}

func (f *Fingerprint) Hash() uint64 {
	return f.hash
}

func (f *Fingerprint) FirstCharacterOfFirstLabelName() string {
	return f.firstCharacterOfFirstLabelName
}

func (f *Fingerprint) LabelMatterLength() uint {
	return f.labelMatterLength
}

func (f *Fingerprint) LastCharacterOfLastLabelValue() string {
	return f.lastCharacterOfLastLabelValue
}

func (f *Fingerprint) Less(o *Fingerprint) bool {
	if f.hash < o.hash {
		return true
	}
	if f.hash > o.hash {
		return false
	}

	if f.firstCharacterOfFirstLabelName < o.firstCharacterOfFirstLabelName {
		return true
	}
	if f.firstCharacterOfFirstLabelName > o.firstCharacterOfFirstLabelName {
		return false
	}

	if f.labelMatterLength < o.labelMatterLength {
		return true
	}
	if f.labelMatterLength > o.labelMatterLength {
		return false
	}

	if f.lastCharacterOfLastLabelValue < o.lastCharacterOfLastLabelValue {
		return true
	}
	if f.lastCharacterOfLastLabelValue > o.lastCharacterOfLastLabelValue {
		return false
	}
	return false
}

func (f *Fingerprint) Equal(o *Fingerprint) bool {
	if f.Hash() != o.Hash() {
		return false
	}

	if f.FirstCharacterOfFirstLabelName() != o.FirstCharacterOfFirstLabelName() {
		return false
	}

	if f.LabelMatterLength() != o.LabelMatterLength() {
		return false
	}

	return f.LastCharacterOfLastLabelValue() == o.LastCharacterOfLastLabelValue()
}

// A basic interface only useful in testing contexts for dispensing the time
// in a controlled manner.
type instantProvider interface {
	// The current instant.
	Now() time.Time
}

// Time is a simple means for fluently wrapping around standard Go timekeeping
// mechanisms to enhance testability without compromising code readability.
//
// It is sufficient for use on bare initialization.  A provider should be
// set only for test contexts.  When not provided, it emits the current
// system time.
type Time struct {
	// The underlying means through which time is provided, if supplied.
	Provider instantProvider
}

// Emit the current instant.
func (t *Time) Now() time.Time {
	if t.Provider == nil {
		return time.Now()
	}

	return t.Provider.Now()
}
