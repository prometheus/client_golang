// Copyright 2025 The Prometheus Authors
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

package fastdto

import (
	dto "github.com/prometheus/client_model/go"
)

type LabelPair struct {
	Name  string `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value" json:"value,omitempty"`
}

func (p LabelPair) GetName() string {
	return p.Name
}

func (p LabelPair) GetValue() string {
	return p.Value
}

// LabelPairSorter implements sort.Interface. It is used to sort a slice of
// LabelPairs
type LabelPairSorter []LabelPair

func (s LabelPairSorter) Len() int {
	return len(s)
}

func (s LabelPairSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s LabelPairSorter) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}

func ToDTOLabelPair(in []LabelPair) []*dto.LabelPair {
	ret := make([]*dto.LabelPair, len(in))
	for i := range in {
		ret[i] = &dto.LabelPair{
			Name:  &(in[i].Name),
			Value: &(in[i].Value),
		}
	}
	return ret
}
