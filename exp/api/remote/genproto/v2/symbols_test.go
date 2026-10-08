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
	decoded, err := DesymbolizeLabels(encoded, s.Symbols(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	requireEqual(t, ls, decoded)

	// Different buf.
	ls = []string{"__name__", "qwer", "zxcv2222", "1234"}
	encoded = s.SymbolizeLabels(ls, []uint32{1, 3, 4, 5})
	requireEqual(t, []uint32{1, 3, 6, 5}, encoded)
}

func TestDesymbolizeLabelsInvalidInput(t *testing.T) {
	// Label references come from a remote-write request, so malformed input has
	// to be reported as an error rather than panicking the receiver.
	for _, tcase := range []struct {
		name      string
		labelRefs []uint32
		symbols   []string
	}{
		{name: "odd number of refs", labelRefs: []uint32{1}, symbols: []string{"", "a"}},
		{name: "odd number of refs, longer", labelRefs: []uint32{1, 1, 1}, symbols: []string{"", "a"}},
		{name: "name ref out of range", labelRefs: []uint32{9, 1}, symbols: []string{"", "a"}},
		{name: "value ref out of range", labelRefs: []uint32{1, 9}, symbols: []string{"", "a"}},
		{name: "refs against empty symbols", labelRefs: []uint32{0, 0}, symbols: nil},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			if _, err := DesymbolizeLabels(tcase.labelRefs, tcase.symbols, nil); err == nil {
				t.Fatal("expected an error, got none")
			}
		})
	}
}
