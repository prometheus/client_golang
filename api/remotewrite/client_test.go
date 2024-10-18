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
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
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
			expected: defaultBackoff,
		},
		{
			name:     "retry-after not provided",
			tInput:   "", // Expected layout is http.TimeFormat, hence an error.
			expected: defaultBackoff,
		},
	}
	for _, c := range tc {
		require.Equal(t, c.expected, retryAfterDuration(c.tInput), c.name)
	}
}
