// Copyright 2019 The Prometheus Authors
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

// +build go1.7

package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	//"github.com/prometheus/common/model" // will need later
	goclient "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/prometheus/web"
)

var (
	mux    *http.ServeMux
	server *httptest.Server
)

func setup() func() {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
	return func() {
		server.Close()
	}
}

// HTTP Handlers
func getAlertManagersHandler_Response() http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"activeAlertManagers": []map[string]string{
				{
					"url": "http://127.0.0.1:9091/api/v1/alerts",
				},
			},
			"droppedAlertManagers": []map[string]string{
				{
					"url": "http://127.0.0.1:9092/api/v1/alerts",
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(response)
		w.Write(b)
	}
	return http.HandlerFunc(hf)
}

func getAlertManagersHandler_Error() http.Handler {
	hf := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server Error", 500)
	}
	return http.HandlerFunc(hf)
}

type apiTest_ struct {
	do      func(h *httpAPI) (interface{}, error)
	handler http.Handler
	muxPath string
	res     interface{}
	err     error
}

func TestAPIs_(t *testing.T) {

	doAlertManagers := func(h *httpAPI) (interface{}, error) {
		return h.AlertManagers(context.Background())
	}

	queryTests := []apiTest_{
		{
			do:      doAlertManagers,
			handler: getAlertManagersHandler_Response(),
			muxPath: "/api/v1/alertmanagers",
			res: AlertManagersResult{
				Active: []AlertManager{
					{
						URL: "http://127.0.0.1:9091/api/v1/alerts",
					},
				},
				Dropped: []AlertManager{
					{
						URL: "http://127.0.0.1:9092/api/v1/alerts",
					},
				},
			},
		},
		{
			do:      doAlertManagers,
			handler: getAlertManagersHandler_Error(),
			muxPath: "/api/v1/alertmanagers",
			err:     errors.New("invalid character 'S' looking for beginning of value"),
		},
	}

	for i, test := range queryTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {

			tearDown := setup()
			defer tearDown()
			config := goclient.Config{Address: server.URL}
			client, err := goclient.NewClient(config)

			promAPI := &httpAPI{
				client: client,
			}

			mux.Handle(test.muxPath, test.handler)
			res, err := test.do(promAPI)

			if test.err != nil {
				if err == nil {
					t.Fatalf("expected error %q but got none", test.err)
				}
				if err.Error() != test.err.Error() {
					t.Errorf("unexpected error: want %s, got %s", test.err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if !reflect.DeepEqual(res, test.res) {
				t.Errorf("unexpected result: want %v, got %v", test.res, res)
			}
		})
	}
}
