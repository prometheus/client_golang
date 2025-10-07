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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/prometheus/common/model"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	writev2 "github.com/prometheus/client_golang/exp/api/remote/genproto/v2"
	"github.com/prometheus/client_golang/exp/internal/github.com/efficientgo/core/backoff"
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
	protos []WriteMessageType

	mockCode *int
	mockErr  error
}

func (m *mockStorage) Store(req *http.Request, msgType WriteMessageType) (*WriteResponse, error) {
	w := NewWriteResponse()
	if m.mockErr != nil {
		if m.mockCode != nil {
			w.SetStatusCode(*m.mockCode)
		}
		return w, m.mockErr
	}

	// Read the request body
	serializedRequest, err := io.ReadAll(req.Body)
	if err != nil {
		w.SetStatusCode(http.StatusBadRequest)
		return w, err
	}

	// This test expects v2 only
	r := &writev2.Request{}
	if err := proto.Unmarshal(serializedRequest, r); err != nil {
		w.SetStatusCode(http.StatusInternalServerError)
		return w, err
	}
	m.v2Reqs = append(m.v2Reqs, r)
	m.protos = append(m.protos, msgType)

	// Set stats in response headers
	w.Add(stats(r))
	w.SetStatusCode(http.StatusNoContent)

	return w, nil
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
	s.confirmed = true
	for _, ts := range req.Timeseries {
		s.Samples += len(ts.Samples)
		s.Histograms += len(ts.Histograms)
		s.Exemplars += len(ts.Exemplars)
	}
	return s
}

func TestRemoteAPI_Write_WithHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tLogger := slog.Default()
		mStore := &mockStorage{}
		srv := httptest.NewServer(NewWriteHandler(mStore, MessageTypes{WriteV2MessageType}, WithWriteHandlerLogger(tLogger)))
		t.Cleanup(srv.Close)

		client, err := NewAPI(srv.URL,
			WithAPIHTTPClient(srv.Client()),
			WithAPILogger(tLogger),
			WithAPIPath("api/v1/write"),
		)
		if err != nil {
			t.Fatal(err)
		}

		req := testV2()
		s, err := client.Write(context.Background(), WriteV2MessageType, req)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(stats(req), s, cmpopts.IgnoreUnexported(WriteResponseStats{})); diff != "" {
			t.Fatal("unexpected stats", diff)
		}
		if len(mStore.v2Reqs) != 1 {
			t.Fatal("expected 1 v2 request stored, got", mStore.v2Reqs)
		}
		if diff := cmp.Diff(req, mStore.v2Reqs[0], protocmp.Transform()); diff != "" {
			t.Fatal("unexpected request received", diff)
		}
	})

	t.Run("storage error", func(t *testing.T) {
		tLogger := slog.Default()
		mockCode := http.StatusInternalServerError
		mStore := &mockStorage{
			mockErr:  errors.New("storage error"),
			mockCode: &mockCode,
		}
		srv := httptest.NewServer(NewWriteHandler(mStore, MessageTypes{WriteV2MessageType}, WithWriteHandlerLogger(tLogger)))
		t.Cleanup(srv.Close)

		client, err := NewAPI(srv.URL,
			WithAPIHTTPClient(srv.Client()),
			WithAPILogger(tLogger),
			WithAPIPath("api/v1/write"),
			WithAPIBackoff(backoff.Config{
				Min:        1 * time.Second,
				Max:        1 * time.Second,
				MaxRetries: 2,
			}),
		)
		if err != nil {
			t.Fatal(err)
		}

		req := testV2()
		_, err = client.Write(context.Background(), WriteV2MessageType, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "storage error") {
			t.Fatalf("expected error to contain 'storage error', got %v", err)
		}
	})

	t.Run("retry callback invoked on retries", func(t *testing.T) {
		tLogger := slog.Default()
		mockCode := http.StatusInternalServerError
		mStore := &mockStorage{
			mockErr:  errors.New("storage error"),
			mockCode: &mockCode,
		}
		srv := httptest.NewServer(NewWriteHandler(mStore, MessageTypes{WriteV2MessageType}, WithWriteHandlerLogger(tLogger)))
		t.Cleanup(srv.Close)

		// Track retry callback invocations
		var retryCount int

		client, err := NewAPI(srv.URL,
			WithAPIHTTPClient(srv.Client()),
			WithAPILogger(tLogger),
			WithAPIPath("api/v1/write"),
			WithAPIBackoff(backoff.Config{
				Min:        1 * time.Millisecond,
				Max:        1 * time.Millisecond,
				MaxRetries: 3,
			}),
			WithAPIRetryCallback(func() {
				retryCount++
			}),
		)
		if err != nil {
			t.Fatal(err)
		}

		req := testV2()
		_, err = client.Write(context.Background(), WriteV2MessageType, req)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Verify callback was invoked for each retry
		expectedRetries := 3
		if retryCount != expectedRetries {
			t.Fatalf("expected %d retry callback invocations, got %d", expectedRetries, retryCount)
		}
	})

	t.Run("retry callback not invoked on success", func(t *testing.T) {
		tLogger := slog.Default()
		mStore := &mockStorage{}
		srv := httptest.NewServer(NewWriteHandler(mStore, MessageTypes{WriteV2MessageType}, WithWriteHandlerLogger(tLogger)))
		t.Cleanup(srv.Close)

		callbackInvoked := false
		client, err := NewAPI(srv.URL,
			WithAPIHTTPClient(srv.Client()),
			WithAPILogger(tLogger),
			WithAPIPath("api/v1/write"),
			WithAPIRetryCallback(func() {
				callbackInvoked = true
			}),
		)
		if err != nil {
			t.Fatal(err)
		}

		req := testV2()
		_, err = client.Write(context.Background(), WriteV2MessageType, req)
		if err != nil {
			t.Fatal(err)
		}

		// Verify callback was not invoked for successful request
		if callbackInvoked {
			t.Fatal("retry callback should not be invoked on successful request")
		}
	})
}
