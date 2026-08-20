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
// stay wire-compatible with what Prometheus serves from
// /api/v1/status/tsdb/blocks.
//
// The client types are hand-copied (importing prometheus/prometheus
// from the main module would be near-circular, since Prometheus
// imports client_golang), so nothing ties them to upstream at compile
// time. These tests import both: they generate real blocks with
// Prometheus's own tooling, serialize the metas like the API handler
// does, and require the client types to reproduce that JSON; a
// reflect-based check additionally compares json tags field by field.
// The module is separate so prometheus/prometheus stays out of
// client_golang's module graph, and its pinned prometheus version is
// dependabot-managed so upstream type changes fail the Parity workflow
// on the bump PR.
package paritytest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// baseT is an arbitrary fixed sample timestamp; it keeps the fixture
// deterministic.
const baseT int64 = 1750860000000

// writeBlock writes a block with float and native histogram samples to
// dir and returns its ULID.
func writeBlock(t *testing.T, dir string, seed int64) ulid.ULID {
	t.Helper()
	ctx := context.Background()

	w, err := tsdb.NewBlockWriter(promslog.NewNopLogger(), dir, tsdb.DefaultBlockDuration)
	require.NoError(t, err)
	defer func() { require.NoError(t, w.Close()) }()

	app := w.Appender(ctx)
	for s := range 3 {
		lbls := labels.FromStrings(model.MetricNameLabel, "parity_test_metric", "series", strconv.Itoa(s))
		for i := range 100 {
			_, err = app.Append(0, lbls, baseT+int64(i)*1000, float64(seed)+float64(i))
			require.NoError(t, err)
		}
	}
	// Histogram samples make stats.numHistogramSamples non-zero.
	hLbls := labels.FromStrings(model.MetricNameLabel, "parity_test_histogram")
	for i := range 10 {
		_, err = app.AppendHistogram(0, hLbls, baseT+int64(i)*1000, tsdbutil.GenerateTestHistogram(seed+int64(i)), nil)
		require.NoError(t, err)
	}
	require.NoError(t, app.Commit())

	id, err := w.Flush(ctx)
	require.NoError(t, err)
	return id
}

// TestTSDBBlocksWireParity round-trips real block metas through the
// client types.
func TestTSDBBlocksWireParity(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Compacting two source blocks yields a level-2 block with real
	// sources and parents.
	srcULIDs := []ulid.ULID{writeBlock(t, dir, 0), writeBlock(t, dir, 1000)}
	srcDirs := make([]string, len(srcULIDs))
	for i, id := range srcULIDs {
		srcDirs[i] = filepath.Join(dir, id.String())
	}

	compactor, err := tsdb.NewLeveledCompactor(ctx, nil, promslog.NewNopLogger(),
		[]int64{tsdb.DefaultBlockDuration}, chunkenc.NewPool(), nil)
	require.NoError(t, err)
	compULIDs, err := compactor.Compact(dir, srcDirs, nil)
	require.NoError(t, err)
	require.Len(t, compULIDs, 1)

	// Read the metas back the way the API handler's DB does.
	db, err := tsdb.OpenDBReadOnly(dir, "", promslog.NewNopLogger())
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	readers, err := db.Blocks()
	require.NoError(t, err)

	metas := make([]tsdb.BlockMeta, 0, len(readers)+1)
	for _, r := range readers {
		metas = append(metas, r.Meta())
	}

	// One synthetic meta covers the fields the tooling above doesn't
	// produce (tombstones, deletable/failed, hints).
	synthetic := tsdb.BlockMeta{
		ULID:    ulid.MustParse("01AAAAAAAAAAAAAAAAAAAAAA00"),
		MinTime: baseT - 7200000,
		MaxTime: baseT,
		Stats:   tsdb.BlockStats{NumTombstones: 5},
		Compaction: tsdb.BlockMetaCompaction{
			Level:     3,
			Sources:   []ulid.ULID{ulid.MustParse("01AAAAAAAAAAAAAAAAAAAAAA01")},
			Deletable: true,
			Failed:    true,
			Parents: []tsdb.BlockDesc{
				{ULID: ulid.MustParse("01AAAAAAAAAAAAAAAAAAAAAA01"), MinTime: baseT - 7200000, MaxTime: baseT},
			},
		},
		Version: 1,
	}
	synthetic.Compaction.SetOutOfOrder()
	metas = append(metas, synthetic)

	// Every omitempty field must appear in the fixture, or its parity
	// goes unchecked.
	var sawFloat, sawHistogram, sawTombstones, sawMultiLevel, sawSources, sawParents, sawDeletable, sawFailed, sawHints bool
	for _, m := range metas {
		sawFloat = sawFloat || m.Stats.NumFloatSamples > 0
		sawHistogram = sawHistogram || m.Stats.NumHistogramSamples > 0
		sawTombstones = sawTombstones || m.Stats.NumTombstones > 0
		sawMultiLevel = sawMultiLevel || m.Compaction.Level > 1
		sawSources = sawSources || len(m.Compaction.Sources) > 0
		sawParents = sawParents || len(m.Compaction.Parents) > 0
		sawDeletable = sawDeletable || m.Compaction.Deletable
		sawFailed = sawFailed || m.Compaction.Failed
		sawHints = sawHints || len(m.Compaction.Hints) > 0
	}
	for name, saw := range map[string]bool{
		"numFloatSamples": sawFloat, "numHistogramSamples": sawHistogram,
		"numTombstones": sawTombstones, "level>1": sawMultiLevel,
		"sources": sawSources, "parents": sawParents,
		"deletable": sawDeletable, "failed": sawFailed, "hints": sawHints,
	} {
		require.True(t, saw, "fixture does not exercise %s; parity for it is unchecked", name)
	}

	// serveTSDBBlocks builds this envelope as a map literal, so the key
	// is duplicated here rather than derived from upstream.
	body, err := json.Marshal(map[string][]tsdb.BlockMeta{"blocks": metas})
	require.NoError(t, err)

	var res v1.TSDBBlocksResult
	require.NoError(t, json.Unmarshal(body, &res))
	require.Len(t, res.Blocks, len(metas))

	// Spot-check the semantic mapping on the compacted block.
	var compacted *v1.TSDBBlockMeta
	for i := range res.Blocks {
		if res.Blocks[i].ULID == compULIDs[0].String() {
			compacted = &res.Blocks[i]
		}
	}
	require.NotNil(t, compacted, "compacted block missing from decoded result")
	require.Equal(t, 2, compacted.Compaction.Level)
	wantULIDs := make([]string, len(srcULIDs))
	for i, id := range srcULIDs {
		wantULIDs[i] = id.String()
	}
	require.ElementsMatch(t, wantULIDs, compacted.Compaction.Sources)
	gotParents := make([]string, len(compacted.Compaction.Parents))
	for i, p := range compacted.Compaction.Parents {
		gotParents[i] = p.ULID
	}
	require.ElementsMatch(t, wantULIDs, gotParents)
	require.NotZero(t, compacted.Stats.NumSamples)
	require.NotZero(t, compacted.Stats.NumSeries)

	// Structural JSON equality (key order aside) catches missing fields,
	// renamed keys, and omitempty mismatches.
	remarshaled, err := json.Marshal(res)
	require.NoError(t, err)
	var want, got any
	require.NoError(t, json.Unmarshal(body, &want))
	require.NoError(t, json.Unmarshal(remarshaled, &got))
	require.Equal(t, want, got,
		"client_golang TSDB block types re-marshal differently than prometheus tsdb.BlockMeta: the type definitions have drifted")
}

// TestTSDBBlockTypeParity compares the json tags of the upstream and
// client types field by field. It backstops the wire test, which only
// sees fields its fixture populates: a field upstream adds later would
// be omitempty-absent from both sides of that comparison and pass
// silently.
func TestTSDBBlockTypeParity(t *testing.T) {
	for _, pair := range []struct {
		name     string
		upstream any
		client   any
	}{
		{"BlockMeta", tsdb.BlockMeta{}, v1.TSDBBlockMeta{}},
		{"BlockStats", tsdb.BlockStats{}, v1.TSDBBlockStats{}},
		{"BlockMetaCompaction", tsdb.BlockMetaCompaction{}, v1.TSDBBlockMetaCompaction{}},
		{"BlockDesc", tsdb.BlockDesc{}, v1.TSDBBlockDesc{}},
	} {
		t.Run(pair.name, func(t *testing.T) {
			require.Equal(t, jsonTags(t, pair.upstream), jsonTags(t, pair.client),
				"JSON field sets of %T and %T have drifted", pair.upstream, pair.client)
		})
	}
}

// jsonTags maps each serialized field of v's type from its JSON name to
// its full json tag. Keying on JSON names keeps upstream Go-level
// renames that leave the wire format alone from failing the guard, and
// field types are ignored because the client deliberately maps
// ulid.ULID to string.
func jsonTags(t *testing.T, v any) map[string]string {
	t.Helper()
	typ := reflect.TypeOf(v)
	tags := make(map[string]string, typ.NumField())
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
		tags[name] = tag
	}
	return tags
}
