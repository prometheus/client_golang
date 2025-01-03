// Copyright (c) Bartłomiej Płotka @bwplotka
// Licensed under the Apache License 2.0.

// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Copyright 2024 Prometheus Team
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

package writev2

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func requireEqual(t testing.TB, expected, got any) {
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Fatal(diff)
	}
}

func TestSymbolsTable(t *testing.T) {
	s := NewSymbolTable()
	requireEqual(t, []string{""}, s.Symbols())
	requireEqual(t, uint32(0), s.Symbolize(""))
	requireEqual(t, []string{""}, s.Symbols())

	requireEqual(t, uint32(1), s.Symbolize("abc"))
	requireEqual(t, []string{"", "abc"}, s.Symbols())

	requireEqual(t, uint32(2), s.Symbolize("__name__"))
	requireEqual(t, []string{"", "abc", "__name__"}, s.Symbols())

	requireEqual(t, uint32(3), s.Symbolize("foo"))
	requireEqual(t, []string{"", "abc", "__name__", "foo"}, s.Symbols())

	s.Reset()
	requireEqual(t, []string{""}, s.Symbols())
	requireEqual(t, uint32(0), s.Symbolize(""))

	requireEqual(t, uint32(1), s.Symbolize("__name__"))
	requireEqual(t, []string{"", "__name__"}, s.Symbols())

	requireEqual(t, uint32(2), s.Symbolize("abc"))
	requireEqual(t, []string{"", "__name__", "abc"}, s.Symbols())

	ls := []string{"__name__", "qwer", "zxcv", "1234"}
	encoded := s.SymbolizeLabels(ls, nil)
	requireEqual(t, []uint32{1, 3, 4, 5}, encoded)
	decoded := DesymbolizeLabels(encoded, s.Symbols(), nil)
	requireEqual(t, ls, decoded)

	// Different buf.
	ls = []string{"__name__", "qwer", "zxcv2222", "1234"}
	encoded = s.SymbolizeLabels(ls, []uint32{1, 3, 4, 5})
	requireEqual(t, []uint32{1, 3, 6, 5}, encoded)
}
