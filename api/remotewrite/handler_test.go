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

package remotewrite

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	writev1 "github.com/prometheus/client_golang/api/remotewrite/genproto/v1"
	writev2 "github.com/prometheus/client_golang/api/remotewrite/genproto/v2"
	"google.golang.org/protobuf/testing/protocmp"
)

type mockStore struct {
	v1Reqs []*writev1.WriteRequest
	v2Reqs []*writev2.Request
	protos []ProtoMsg

	mockCode *int
	mockErr  error
}

func (m *mockStore) StoreV1(_ context.Context, r *writev1.WriteRequest) (_ *WriteResponseStats, code *int, _ error) {
	m.v1Reqs = append(m.v1Reqs, r)
	return nil, m.mockCode, m.mockErr
}

func (m *mockStore) StoreV2(_ context.Context, r *writev2.Request) (_ *WriteResponseStats, code *int, _ error) {
	m.v2Reqs = append(m.v2Reqs, r)
	return nil, m.mockCode, m.mockErr
}

func (m *mockStore) Store(_ context.Context, proto ProtoMsg, serializedRequest []byte) (w WriteResponseStats, code int, _ error) {
	// For test purposes we send JSON encoded write response to populate.
	if err := json.Unmarshal(serializedRequest, &w); err != nil {
		return w, http.StatusInternalServerError, err
	}

	m.protos = append(m.protos, proto)

	if m.mockErr != nil {
		return w, *m.mockCode, m.mockErr
	}
	return w, http.StatusOK, nil
}

func testV1() *writev1.WriteRequest {
	return &writev1.WriteRequest{
		Timeseries: []*writev1.TimeSeries{
			{
				Labels: []*writev1.Label{
					{Name: "__name__", Value: "metric1"},
					{Name: "foo", Value: "bar1"},
				},
				Samples: []*writev1.Sample{
					{Value: 1.1, Timestamp: 1214141},
					{Value: 1.5, Timestamp: 1214180},
				},
			},
			{
				Labels: []*writev1.Label{
					{Name: "__name__", Value: "metric1"},
					{Name: "foo", Value: "bar2"},
				},
				Samples: []*writev1.Sample{
					{Value: 1231311, Timestamp: 1214141},
					{Value: 1310001, Timestamp: 1214180},
				},
			},
		},
	}
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

func TestClientHandler_EncodingDecoding(t *testing.T) {
	tLogger := slog.Default()
	mStore := &mockStore{}
	srv := httptest.NewServer(NewHandler(tLogger, NewDecodingStore(mStore)))
	t.Cleanup(srv.Close)

	client := NewClient(tLogger, srv.URL, nil, SnappyBlockCompression, "yolo", false)
	eClient := NewEncodingClient(client)

	t.Run(string(ProtoMsgV1), func(t *testing.T) {
		req := testV1()
		s, err := eClient.WriteV1(context.Background(), req, nil)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(WriteResponseStats{}.AddV1(req), s); diff != "" {
			t.Fatal("unexpected stats", diff)
		}
		if len(mStore.v1Reqs) != 1 {
			t.Fatal("expected 1 request stored, got", mStore.v1Reqs)
		}
		if diff := cmp.Diff(req, mStore.v1Reqs[0], protocmp.Transform()); diff != "" {
			t.Fatal("unexpected request received", diff)
		}
	})
	t.Run(string(ProtoMsgV2), func(t *testing.T) {
		req := testV2()
		s, err := eClient.WriteV2(context.Background(), req, nil)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(WriteResponseStats{}.AddV2(req), s); diff != "" {
			t.Fatal("unexpected stats", diff)
		}
		if len(mStore.v2Reqs) != 1 {
			t.Fatal("expected 1 request stored, got", mStore.v1Reqs)
		}
		if diff := cmp.Diff(req, mStore.v2Reqs[0], protocmp.Transform()); diff != "" {
			t.Fatal("unexpected request received", diff)
		}
	})
}
