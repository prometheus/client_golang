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

package promhttp

import (
	"net/http/httptest"
	"testing"
)

func TestResponseWriterDelegatorUnwrap(t *testing.T) {
	w := httptest.NewRecorder()
	rwd := &responseWriterDelegator{ResponseWriter: w}

	if rwd.Unwrap() != w {
		t.Error("unwrapped responsewriter must equal to the original responsewriter")
	}
}
