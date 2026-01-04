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

package zstd

import (
	"bytes"
	"io"
	"testing"

	kpzstd "github.com/klauspost/compress/zstd"

	prominternal "github.com/prometheus/client_golang/prometheus/promhttp/internal"
)

func TestZstdRegistersWriter(t *testing.T) {
	if prominternal.NewZstdWriter == nil {
		t.Fatal("internal.NewZstdWriter is nil; zstd support not registered")
	}
}

func TestZstdWriterRoundTrip(t *testing.T) {
	if prominternal.NewZstdWriter == nil {
		t.Fatal("internal.NewZstdWriter is nil; zstd support not registered")
	}

	var buf bytes.Buffer
	w, closeFn, err := prominternal.NewZstdWriter(&buf)
	if err != nil {
		t.Fatalf("NewZstdWriter returned error: %v", err)
	}
	if closeFn == nil {
		t.Fatal("closeFn is nil")
	}

	in := []byte("hello zstd")
	if _, err := w.Write(in); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	closeFn()

	r, err := kpzstd.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("zstd NewReader failed: %v", err)
	}
	defer r.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("round-trip mismatch: got %q, want %q", out, in)
	}
}
