// Copyright 2021 The Prometheus Authors
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

package collectors

import (
	"database/sql"
	"runtime"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestDBStatsCollector(t *testing.T) {
	reg := prometheus.NewRegistry()
	db := new(sql.DB)
	opts := DBStatsCollectorOpts{DriverName: "test"}
	if err := reg.Register(NewDBStatsCollector(db, opts)); err != nil {
		t.Fatal(err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}

	names := []string{
		"go_test_db_stats_max_open_connections",
		"go_test_db_stats_open_connections",
		"go_test_db_stats_in_use_connections",
		"go_test_db_stats_idle_connections",
		"go_test_db_stats_wait_count_total",
		"go_test_db_stats_wait_duration_seconds_total",
		"go_test_db_stats_max_idle_closed_total",
		"go_test_db_stats_max_lifetime_closed_total",
	}
	if runtime.Version() >= "go1.15" {
		names = append(names, "go_test_db_stats_max_idle_time_closed_total")
	}
	type result struct {
		found bool
	}
	results := make(map[string]result)
	for _, name := range names {
		results[name] = result{found: false}
	}
	for _, mf := range mfs {
		for _, name := range names {
			if name == mf.GetName() {
				results[name] = result{found: true}
				break
			}
		}
	}

	for name, result := range results {
		if !result.found {
			t.Errorf("%s not found", name)
		}
	}
}
