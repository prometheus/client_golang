// Copyright 2015 The Prometheus Authors
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

package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestConfig(t *testing.T) {
	c := Config{}
	if c.roundTripper() != DefaultRoundTripper {
		t.Fatalf("expected default roundtripper for nil RoundTripper field")
	}
}

func TestClientURL(t *testing.T) {
	tests := []struct {
		address  string
		endpoint string
		args     map[string]string
		expected string
	}{
		{
			address:  "http://localhost:9090",
			endpoint: "/test",
			expected: "http://localhost:9090/test",
		},
		{
			address:  "http://localhost",
			endpoint: "/test",
			expected: "http://localhost/test",
		},
		{
			address:  "http://localhost:9090",
			endpoint: "test",
			expected: "http://localhost:9090/test",
		},
		{
			address:  "http://localhost:9090/prefix",
			endpoint: "/test",
			expected: "http://localhost:9090/prefix/test",
		},
		{
			address:  "https://localhost:9090/",
			endpoint: "/test/",
			expected: "https://localhost:9090/test",
		},
		{
			address:  "http://localhost:9090",
			endpoint: "/test/:param",
			args: map[string]string{
				"param": "content",
			},
			expected: "http://localhost:9090/test/content",
		},
		{
			address:  "http://localhost:9090",
			endpoint: "/test/:param/more/:param",
			args: map[string]string{
				"param": "content",
			},
			expected: "http://localhost:9090/test/content/more/content",
		},
		{
			address:  "http://localhost:9090",
			endpoint: "/test/:param/more/:foo",
			args: map[string]string{
				"param": "content",
				"foo":   "bar",
			},
			expected: "http://localhost:9090/test/content/more/bar",
		},
		{
			address:  "http://localhost:9090",
			endpoint: "/test/:param",
			args: map[string]string{
				"nonexistent": "content",
			},
			expected: "http://localhost:9090/test/:param",
		},
		{
			address:  "http://localhost:9090",
			endpoint: "/test/:foobar",
			args: map[string]string{
				"foo": "content",
			},
			expected: "http://localhost:9090/test/:foobar",
		},
	}

	for _, test := range tests {
		ep, err := url.Parse(test.address)
		if err != nil {
			t.Fatal(err)
		}

		hclient := &httpClient{
			endpoint: ep,
			client:   http.Client{Transport: DefaultRoundTripper},
		}

		u := hclient.URL(test.endpoint, test.args)
		if u.String() != test.expected {
			t.Errorf("unexpected result: got %s, want %s", u, test.expected)
			continue
		}
	}
}

func TestDoContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("partial"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		<-r.Context().Done()
	}))

	defer ts.Close()

	client, err := NewClient(Config{
		Address: ts.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp, body, err := client.Do(ctx, req)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected error %v, got: %v", context.DeadlineExceeded, err)
	}
	if body != nil {
		t.Errorf("expected no body due to cancellation, got: %q", string(body))
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Do did not return promptly on cancellation: took %v", elapsed)
	}

	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
}

func TestDoContextCancellationSlowServer(t *testing.T) {
	serverDone := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-serverDone
	}))
	defer func() {
		close(serverDone)
		ts.Close()
	}()

	client, err := NewClient(Config{
		Address: ts.URL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp, body, err := client.Do(ctx, req)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected error %v, got: %v", context.DeadlineExceeded, err)
	}
	if body != nil {
		t.Errorf("expected no body due to cancellation, got: %q", string(body))
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("client.Do did not return promptly on cancellation: took %v", elapsed)
	}

	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type blockingReadCloser struct {
	readStarted chan struct{}
	closed      chan struct{}
	startOnce   sync.Once
	closeOnce   sync.Once
}

func (b *blockingReadCloser) Read(_ []byte) (int, error) {
	b.startOnce.Do(func() {
		close(b.readStarted)
	})
	<-b.closed
	return 0, io.EOF
}

func (b *blockingReadCloser) Close() error {
	b.closeOnce.Do(func() {
		close(b.closed)
	})
	return nil
}

func TestDoNilContext(t *testing.T) {
	tests := []struct {
		name          string
		cancelRequest bool
		wantBody      string
		wantErr       error
	}{
		{
			name:     "returns response without panic",
			wantBody: "ok",
		},
		{
			name:          "honors request context cancellation",
			cancelRequest: true,
			wantErr:       context.Canceled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{}
			targetURL := "http://example.com"
			var bodyReader *blockingReadCloser
			if tc.cancelRequest {
				// Use a blocking RoundTripper that ignores req.Context() so cancellation
				// is deterministically handled by client.Do after body reading starts.
				bodyReader = &blockingReadCloser{
					readStarted: make(chan struct{}),
					closed:      make(chan struct{}),
				}
				defer bodyReader.Close()
				cfg.RoundTripper = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       bodyReader,
					}, nil
				})
			} else {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, _ = w.Write([]byte("ok"))
				}))
				defer ts.Close()
				targetURL = ts.URL
			}
			cfg.Address = targetURL

			client, err := NewClient(cfg)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			var (
				req  *http.Request
				done = func() {}
			)
			if tc.cancelRequest {
				reqCtx, cancel := context.WithCancel(context.Background())
				done = cancel
				go func() {
					select {
					case <-bodyReader.readStarted:
						cancel()
					case <-reqCtx.Done():
					}
				}()
				req, err = http.NewRequestWithContext(reqCtx, http.MethodGet, targetURL, nil)
			} else {
				req, err = http.NewRequest(http.MethodGet, targetURL, nil)
			}
			defer done()
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			type doResult struct {
				resp *http.Response
				body []byte
				err  error
			}
			resCh := make(chan doResult, 1)
			go func() {
				var nilCtx context.Context
				resp, body, err := client.Do(nilCtx, req)
				resCh <- doResult{resp: resp, body: body, err: err}
			}()

			var res doResult
			select {
			case res = <-resCh:
			case <-time.After(2 * time.Second):
				t.Fatalf("client.Do did not return within 2s after context cancellation")
			}
			if res.resp != nil && res.resp.Body != nil {
				defer res.resp.Body.Close()
			}

			if tc.wantErr != nil {
				if !errors.Is(res.err, tc.wantErr) {
					t.Fatalf("expected error %v, got %v", tc.wantErr, res.err)
				}
				if res.body != nil {
					t.Fatalf("expected nil body on error, got %q", string(res.body))
				}
				return
			}
			if res.err != nil {
				t.Fatalf("unexpected error: %v", res.err)
			}
			if string(res.body) != tc.wantBody {
				t.Fatalf("expected body %q, got %q", tc.wantBody, string(res.body))
			}
		})
	}
}

// Serve any http request with a response of N KB of spaces.
type serveSpaces struct {
	sizeKB int
}

func (t serveSpaces) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	kb := bytes.Repeat([]byte{' '}, 1024)
	for i := 0; i < t.sizeKB; i++ {
		w.Write(kb)
	}
}

func BenchmarkClient(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()

	for _, sizeKB := range []int{4, 50, 1000, 2000} {
		b.Run(fmt.Sprintf("size=%dKB", sizeKB), func(b *testing.B) {
			testServer := httptest.NewServer(serveSpaces{sizeKB})
			defer testServer.Close()

			client, err := NewClient(Config{
				Address: testServer.URL,
			})
			if err != nil {
				b.Fatalf("Failed to initialize client: %v", err)
			}
			url, err := url.Parse(testServer.URL + "/prometheus/api/v1/query?query=up")
			if err != nil {
				b.Fatalf("Failed to parse url: %v", err)
			}
			req := &http.Request{
				URL: url,
			}
			for b.Loop() {
				_, _, err := client.Do(ctx, req)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}
		})
	}
}
