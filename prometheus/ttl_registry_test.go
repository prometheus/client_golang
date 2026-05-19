// Copyright 2014 The Prometheus Authors
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

package prometheus

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestNewTTLRegistryPanicsOnNonPositiveTTL(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for ttl <= 0")
		}
	}()
	NewTTLRegistry(0)
}

func TestTTLRegistryGatherRunsCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ttl := 80 * time.Millisecond
		reg := NewTTLRegistry(ttl)
		vec := reg.NewCounterVec(CounterOpts{
			Name: "ttl_reg_gather",
			Help: "test",
		}, []string{"code"})

		vec.WithLabelValues("200").Add(1)
		if n := collectCount(vec); n != 1 {
			t.Fatalf("expected 1 metric before sleep, got %d", n)
		}

		time.Sleep(ttl + 40*time.Millisecond)

		if _, err := reg.Gather(); err != nil {
			t.Fatal(err)
		}

		if n := collectCount(vec); n != 0 {
			t.Fatalf("expected 0 metrics after Gather cleanup, got %d", n)
		}
	})
}
