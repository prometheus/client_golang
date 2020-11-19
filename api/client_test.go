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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestConfig(t *testing.T) {
	c := Config{}
	if c.roundTripper() != DefaultRoundTripper {
		t.Fatalf("expected default roundtripper for nil RoundTripper field")
	}
}

func TestHeaders(t *testing.T) {
	headerContents := "Bearer 09q38c91203498c124c12"

	// Initialize mock http server so we don't have to depend on external things for unit tests
	// This mock responds with a json payload containing all request headers
	requestMock := func(w http.ResponseWriter, r *http.Request) {
		t.Logf("request: %v", r)
		j, _ := json.Marshal(map[string]http.Header{"headers": r.Header})
		_, _ = w.Write(j)
	}
	handler := http.NewServeMux()
	handler.HandleFunc("/headers", requestMock)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	// Set up client with mock server address
	ep, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Any headers added to the client will be automatically added to all requests
	hclient := &httpClient{
		endpoint: ep,
		headers:  map[string]string{"Authorization": headerContents},
		client:   http.Client{Transport: DefaultRoundTripper},
	}

	// Query the mock server
	reqUrl := hclient.URL("/headers", nil)
	req, _ := http.NewRequest(http.MethodGet, reqUrl.String(), nil)
	ctx := context.Background()
	_, bytes, err := hclient.Do(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	// Parse response & validate the headers got set
	var responseJson map[string]http.Header
	err = json.Unmarshal(bytes, &responseJson)
	if err != nil {
		t.Fatal(err)
	}
	if responseJson["headers"]["Authorization"][0] != headerContents {
		t.Errorf("expected %s, got: %s", headerContents, responseJson["headers"]["Authorization"])
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
