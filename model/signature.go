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

package model

import (
	"hash/fnv"
	"sort"
)

// cache the signature of an empty label set.
var emptyLabelSignature = fnv.New64a().Sum64()

// LabelsToSignature provides a way of building a unique signature
// (i.e., fingerprint) for a given label set sequence.
func LabelsToSignature(labels map[string]string) uint64 {
	if len(labels) == 0 {
		return emptyLabelSignature
	}

	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}

	sort.Strings(names)

	hasher := fnv.New64a()

	for _, name := range names {
		hasher.Write([]byte(name))
		hasher.Write([]byte(labels[name]))
	}

	return hasher.Sum64()
}

// LabelValuesToSignature provides a way of building a unique signature
// (i.e., fingerprint) for a given set of label's values.
func LabelValuesToSignature(labels map[string]string) uint64 {
	if len(labels) == 0 {
		return emptyLabelSignature
	}

	names := make([]string, 0, len(labels))
	for name := range labels {
		names = append(names, name)
	}

	sort.Strings(names)

	hasher := fnv.New64a()

	for _, name := range names {
		hasher.Write([]byte(labels[name]))
	}

	return hasher.Sum64()
}
