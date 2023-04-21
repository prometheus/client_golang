// Copyright 2023 The Prometheus Authors
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

//go:build interactive
// +build interactive

package internal

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/efficientgo/core/testutil"
)

func TestAcceptance(t *testing.T) {
	resp, err := http.Get(whatsupAddr(fmt.Sprintf("http://localhost:%v", WhatsupPort)) + "/metrics")
	testutil.Ok(t, err)
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	testutil.Ok(t, err)

	metrics := string(b)

	gotErr := false
	for _, tcase := range []struct {
		expectMetricName string
		expectMetricType string
		expectExemplar   bool // TODO
	}{
		{
			expectMetricName: "whatsup_queries_handled_total",
			expectMetricType: "counter",
		},
		{
			expectMetricName: "whatsup_last_response_elements",
			expectMetricType: "gauge",
		},
		{
			expectMetricName: "build_info",
			expectMetricType: "gauge",
		},
		{
			expectMetricName: "whatsup_queries_duration_seconds",
			expectMetricType: "histogram",
		},
		{
			expectMetricName: "go_goroutines",
			expectMetricType: "gauge",
		},
	} {
		if !t.Run(fmt.Sprintf("Metric %v type %v with exemplar %v", tcase.expectMetricName, tcase.expectMetricType, tcase.expectExemplar), func(t *testing.T) {
			// Yolo.
			expLine := fmt.Sprintf("# TYPE %v %v", tcase.expectMetricName, tcase.expectMetricType)
			if strings.Index(metrics, expLine) == -1 {
				t.Error(expLine, "not found!")
			}
			// TODO(bwplotka): Check all properly.
		}) {
			gotErr = true
		}
	}

	if gotErr {
		fmt.Println("Got this response from ", fmt.Sprintf("http://localhost:%v", WhatsupPort), ":", metrics)
	}

}
