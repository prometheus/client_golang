// Copyright The Prometheus Authors
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

import "testing"

// TestConstrainLabelsIsolatesCallerMap reproduces the root cause of #1951:
// without label constraints, constrainLabels used to return the caller's map.
// Mutations to that map between hashing and metric creation can then desync the
// hash from the stored label values.
func TestConstrainLabelsIsolatesCallerMap(t *testing.T) {
	desc := NewDesc("test_metric", "help", []string{"l1", "l2"}, nil)

	original := Labels{"l1": "a", "l2": "b"}
	got := constrainLabels(desc, original)
	defer putLabelsToPool(got)

	original["l1"] = "mutated"

	if got["l1"] != "a" {
		t.Fatalf("constrainLabels returned a live view of the caller map; got l1=%q, want %q", got["l1"], "a")
	}
	if got["l2"] != "b" {
		t.Fatalf("unexpected l2 value: got %q, want %q", got["l2"], "b")
	}
}
