// Copyright 2024 The Prometheus Authors
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

package remote

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/prometheus/client_golang/api"
	writev2 "github.com/prometheus/client_golang/exp/api/remote/genproto/v2"
)

func TestRetryAfterDuration(t *testing.T) {
	tc := []struct {
		name     string
		tInput   string
		expected model.Duration
	}{
		{
			name:     "seconds",
			tInput:   "120",
			expected: model.Duration(time.Second * 120),
		},
		{
			name:     "date-time default",
			tInput:   time.RFC1123, // Expected layout is http.TimeFormat, hence an error.
			expected: 0,
		},
		{
			name:     "retry-after not provided",
			tInput:   "", // Expected layout is http.TimeFormat, hence an error.
			expected: 0,
		},
	}
	for _, c := range tc {
		if got := retryAfterDuration(c.tInput); got != time.Duration(c.expected) {
			t.Fatal("expected", c.expected, "got", got)
		}
	}
}

type mockStorage struct {
	v2Reqs []*writev2.Request
	protos []WriteProtoFullName

	mockCode *int
	mockErr  error
}

func (m *mockStorage) Store(_ context.Context, msgFullName WriteProtoFullName, serializedRequest []byte) (w WriteResponseStats, code int, _ error) {
	if m.mockErr != nil {
		return w, *m.mockCode, m.mockErr
	}

	// This test expects v2 only.
	r := &writev2.Request{}
	if err := proto.Unmarshal(serializedRequest, r); err != nil {
		return WriteResponseStats{}, http.StatusInternalServerError, err
	}
	m.v2Reqs = append(m.v2Reqs, r)
	m.protos = append(m.protos, msgFullName)
	return stats(r), http.StatusOK, nil
}

func testV2() *writev2.Request {
	s := writev2.NewSymbolTable()
	return &writev2.Request{
		Timeseries: []*writev2.TimeSeries{
			{
				Metadata: &writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
					HelpRef: s.Symbolize("My lovely counter"),
				},
				LabelsRefs: s.SymbolizeLabels([]string{"__name__", "metric1", "foo", "bar1"}, nil),
				Samples: []*writev2.Sample{
					{Value: 1.1, Timestamp: 1214141},
					{Value: 1.5, Timestamp: 1214180},
				},
			},
			{
				Metadata: &writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_COUNTER,
					HelpRef: s.Symbolize("My lovely counter"),
				},
				LabelsRefs: s.SymbolizeLabels([]string{"__name__", "metric1", "foo", "bar2"}, nil),
				Samples: []*writev2.Sample{
					{Value: 1231311, Timestamp: 1214141},
					{Value: 1310001, Timestamp: 1214180},
				},
			},
		},
	}
}

func stats(req *writev2.Request) (s WriteResponseStats) {
	s.Confirmed = true
	for _, ts := range req.Timeseries {
		s.Samples += len(ts.Samples)
		s.Histograms += len(ts.Histograms)
		s.Exemplars += len(ts.Exemplars)
	}
	return s
}

func TestRemoteAPI_Write_WithHandler(t *testing.T) {
	tLogger := slog.Default()
	mStore := &mockStorage{}
	srv := httptest.NewServer(NewRemoteWriteHandler(mStore, WithHandlerLogger(tLogger)))
	t.Cleanup(srv.Close)

	cl, err := api.NewClient(api.Config{
		Address:      srv.URL,
		RoundTripper: srv.Client().Transport,
	})
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewAPI(cl, WithAPILogger(tLogger), WithAPIPath("api/v1/write"))
	if err != nil {
		t.Fatal(err)
	}

	req := testV2()
	s, err := client.Write(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(stats(req), s); diff != "" {
		t.Fatal("unexpected stats", diff)
	}
	if len(mStore.v2Reqs) != 1 {
		t.Fatal("expected 1 v2 request stored, got", mStore.v2Reqs)
	}
	if diff := cmp.Diff(req, mStore.v2Reqs[0], protocmp.Transform()); diff != "" {
		t.Fatal("unexpected request received", diff)
	}
}
