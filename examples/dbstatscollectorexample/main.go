// Copyright 2022 The Prometheus Authors
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

// A minimal example of how to include Prometheus instrumentation for database stats.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")

func main() {
	flag.Parse()

	// Set up an in-memory SQLite DB.
	db, err := sql.Open("sqlite3", ":memory:") // In-memory SQLite database
	if err != nil {
		log.Fatalf("Failed to connect to in-memory database: %v", err)
	}
	defer db.Close()

	// Set connection pool limits to simulate more activity.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Create a new Prometheus registry.
	reg := prometheus.NewRegistry()

	// Create and register the DB stats collector.
	dbStatsCollector := collectors.NewDBStatsCollector(db, "sqlite_in_memory")
	reg.MustRegister(dbStatsCollector)

	// Expose the registered metrics via HTTP.
	http.Handle("/metrics", promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{},
	))

	fmt.Println("Server is running, metrics are available at /metrics")
	log.Fatal(http.ListenAndServe(*addr, nil))
}
