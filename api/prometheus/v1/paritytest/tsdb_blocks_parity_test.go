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

// Package paritytest checks that the v1 API client's TSDB block types
// stay in sync with the prometheus/prometheus types they are
// hand-copied from. The client types are copies (importing
// prometheus/prometheus from the main module would be near-circular,
// since Prometheus imports client_golang), so nothing ties them to
// upstream at compile time.
//
// The check is deliberately reflection-only: it diffs the struct
// definitions field by field (JSON names, tags, and field types)
// without exercising Prometheus's block machinery, so it depends only
// on the exported type definitions -- the thing it guards. The module
// is separate so prometheus/prometheus stays out of client_golang's
// module graph, and its pinned prometheus version is
// dependabot-managed so upstream type changes fail the Parity workflow
// on the bump PR.
package paritytest

import (
	"reflect"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/prometheus/tsdb"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// pairs maps each upstream block type to its client copy. Every named
// struct reachable from these types must have its own entry, or
// canonicalType fails the test asking for one; without that guard a
// struct field added upstream would compare by name only and its
// fields would go unchecked.
var pairs = []struct {
	name             string
	upstream, client any
}{
	{"BlockMeta", tsdb.BlockMeta{}, v1.TSDBBlockMeta{}},
	{"BlockStats", tsdb.BlockStats{}, v1.TSDBBlockStats{}},
	{"BlockMetaCompaction", tsdb.BlockMetaCompaction{}, v1.TSDBBlockMetaCompaction{}},
	{"BlockDesc", tsdb.BlockDesc{}, v1.TSDBBlockDesc{}},
}

// TestTSDBBlockTypeParity diffs the upstream and client struct
// definitions field by field, catching added, removed, renamed, or
// retyped fields and omitempty drift.
func TestTSDBBlockTypeParity(t *testing.T) {
	for _, pair := range pairs {
		t.Run(pair.name, func(t *testing.T) {
			require.Equal(t, jsonFields(t, pair.upstream), jsonFields(t, pair.client),
				"serialized fields of %T and %T have drifted", pair.upstream, pair.client)
		})
	}
}

// fieldShape is one serialized struct field as compared across the
// two modules: the full json tag and the canonicalized field type.
type fieldShape struct {
	Tag  string
	Type string
}

// jsonFields maps each serialized field of v's type from its JSON name
// to its shape. Keying on JSON names keeps upstream Go-level renames
// that leave the wire format alone from failing the guard.
func jsonFields(t *testing.T, v any) map[string]fieldShape {
	t.Helper()
	typ := reflect.TypeOf(v)
	fields := make(map[string]fieldShape, typ.NumField())
	for i := range typ.NumField() {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		// Untagged exported fields still serialize (under the Go name).
		tag, ok := f.Tag.Lookup("json")
		require.True(t, ok, "%s.%s has no json tag", typ, f.Name)
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			name = f.Name
		}
		fields[name] = fieldShape{Tag: tag, Type: canonicalType(t, f.Type)}
	}
	return fields
}

// canonicalType renders a field type so that the deliberate
// representation differences between the modules compare equal: the
// client maps ulid.ULID to string (same JSON encoding), and prefixes
// upstream struct names with TSDB. Named structs compare by canonical
// name only; their fields are covered by their own entry in pairs,
// which is required to exist.
func canonicalType(t *testing.T, typ reflect.Type) string {
	t.Helper()
	switch {
	case typ == reflect.TypeOf(ulid.ULID{}):
		return "string"
	case typ.Kind() == reflect.Slice:
		return "[]" + canonicalType(t, typ.Elem())
	case typ.Kind() == reflect.Struct && typ.Name() != "":
		name := strings.TrimPrefix(typ.Name(), "TSDB")
		for _, pair := range pairs {
			if pair.name == name {
				return name
			}
		}
		require.Failf(t, "unpaired struct type", "%s has no entry in pairs; add one so its fields are checked", typ)
		return ""
	default:
		return typ.String()
	}
}
