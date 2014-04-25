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

package model

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
)

// Fingerprint provides a hash-capable representation of a Metric.
type Fingerprint struct {
	// A hashed representation of the underyling entity.  For our purposes, FNV-1A
	// 64-bit is used.
	Hash                           uint64
	FirstCharacterOfFirstLabelName string
	LabelMatterLength              uint
	LastCharacterOfLastLabelValue  string
}

func (f *Fingerprint) String() string {
	return strings.Join([]string{fmt.Sprintf("%020d", f.Hash), f.FirstCharacterOfFirstLabelName, fmt.Sprint(f.LabelMatterLength), f.LastCharacterOfLastLabelValue}, "-")
}

// Less compares first the Hash, then the FirstCharacterOfFirstLabelName, then
// the LabelMatterLength, then the LastCharacterOfLastLabelValue.
func (f *Fingerprint) Less(o *Fingerprint) bool {
	if f.Hash < o.Hash {
		return true
	}
	if f.Hash > o.Hash {
		return false
	}

	if f.FirstCharacterOfFirstLabelName < o.FirstCharacterOfFirstLabelName {
		return true
	}
	if f.FirstCharacterOfFirstLabelName > o.FirstCharacterOfFirstLabelName {
		return false
	}

	if f.LabelMatterLength < o.LabelMatterLength {
		return true
	}
	if f.LabelMatterLength > o.LabelMatterLength {
		return false
	}

	if f.LastCharacterOfLastLabelValue < o.LastCharacterOfLastLabelValue {
		return true
	}
	if f.LastCharacterOfLastLabelValue > o.LastCharacterOfLastLabelValue {
		return false
	}
	return false
}

// Equal uses the same semantics as Less.
func (f *Fingerprint) Equal(o *Fingerprint) bool {
	if f.Hash != o.Hash {
		return false
	}

	if f.FirstCharacterOfFirstLabelName != o.FirstCharacterOfFirstLabelName {
		return false
	}

	if f.LabelMatterLength != o.LabelMatterLength {
		return false
	}

	return f.LastCharacterOfLastLabelValue == o.LastCharacterOfLastLabelValue
}

const rowKeyDelimiter = "-"

// LoadFromString transforms a string representation into a Fingerprint,
// resetting any previous attributes.
func (f *Fingerprint) LoadFromString(s string) {
	components := strings.Split(s, rowKeyDelimiter)
	hash, err := strconv.ParseUint(components[0], 10, 64)
	if err != nil {
		panic(err)
	}
	labelMatterLength, err := strconv.ParseUint(components[2], 10, 0)
	if err != nil {
		panic(err)
	}

	f.Hash = hash
	f.FirstCharacterOfFirstLabelName = components[1]
	f.LabelMatterLength = uint(labelMatterLength)
	f.LastCharacterOfLastLabelValue = components[3]
}

const reservedDelimiter = `"`

// LoadFromMetric decomposes a Metric into this Fingerprint
func (f *Fingerprint) LoadFromMetric(m Metric) {
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

		summer.Write([]byte(labelName))
		summer.Write([]byte(reservedDelimiter))
		summer.Write([]byte(labelValue))
	}

	f.FirstCharacterOfFirstLabelName = firstCharacterOfFirstLabelName
	f.Hash = binary.LittleEndian.Uint64(summer.Sum(nil))
	f.LabelMatterLength = uint(labelMatterLength % 10)
	f.LastCharacterOfLastLabelValue = lastCharacterOfLastLabelValue
}

// Fingerprints represents a collection of Fingerprint subject to a given
// natural sorting scheme. It implements sort.Interface.
type Fingerprints []*Fingerprint

func (f Fingerprints) Len() int {
	return len(f)
}

func (f Fingerprints) Less(i, j int) bool {
	return f[i].Less(f[j])
}

func (f Fingerprints) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// FingerprintSet is a set of Fingerprints.
type FingerprintSet map[Fingerprint]struct{}

// Equal returns true if both sets contain the same elements (and not more).
func (s FingerprintSet) Equal(o FingerprintSet) bool {
	if len(s) != len(o) {
		return false
	}

	for k := range s {
		if _, ok := o[k]; !ok {
			return false
		}
	}

	return true
}

// Intersection returns the elements contained in both sets.
func (s FingerprintSet) Intersection(o FingerprintSet) FingerprintSet {
	myLength, otherLength := len(s), len(o)
	if myLength == 0 || otherLength == 0 {
		return FingerprintSet{}
	}

	subSet := s
	superSet := o

	if otherLength < myLength {
		subSet = o
		superSet = s
	}

	out := FingerprintSet{}

	for k := range subSet {
		if _, ok := superSet[k]; ok {
			out[k] = struct{}{}
		}
	}

	return out
}
