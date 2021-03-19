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

package prometheus

import "database/sql"

type dbStatsCollector struct {
	db *sql.DB

	maxOpenConnections *Desc

	openConnections  *Desc
	inUseConnections *Desc
	idleConnections  *Desc

	waitCount     *Desc
	waitDuration  *Desc
	maxIdleClosed *Desc
	// maxIdleTimeClosed *Desc // TODO: for 1.15 or later
	maxLifetimeClosed *Desc
}

// DBStatsCollectorOpts defines the behavior of a db stats collector
// created with NewDBStatsCollector.
type DBStatsCollectorOpts struct {
	// DriverName holds the name of driver.
	// It will not used for empty strings.
	DriverName string
}

// NewDBStatsCollector returns a collector that exports metrics about the given *sql.DB.
// See https://golang.org/pkg/database/sql/#DBStats for more information on stats.
func NewDBStatsCollector(db *sql.DB, opts DBStatsCollectorOpts) Collector {
	var fqName func(name string) string
	if opts.DriverName == "" {
		fqName = func(name string) string {
			return "go_db_stats_" + name
		}
	} else {
		fqName = func(name string) string {
			return "go_" + opts.DriverName + "_db_stats_" + name
		}
	}
	return &dbStatsCollector{
		db: db,
		maxOpenConnections: NewDesc(
			fqName("max_open_connections"),
			"Maximum number of open connections to the database.",
			nil, nil,
		),
		openConnections: NewDesc(
			fqName("open_connections"),
			"The number of established connections both in use and idle.",
			nil, nil,
		),
		inUseConnections: NewDesc(
			fqName("in_use_connections"),
			"The number of connections currently in use.",
			nil, nil,
		),
		idleConnections: NewDesc(
			fqName("idle_connections"),
			"The number of idle connections.",
			nil, nil,
		),
		waitCount: NewDesc(
			fqName("wait_count_total"),
			"The total number of connections waited for.",
			nil, nil,
		),
		waitDuration: NewDesc(
			fqName("wait_duration_seconds_total"),
			"The total time blocked waiting for a new connection.",
			nil, nil,
		),
		maxIdleClosed: NewDesc(
			fqName("max_idle_closed_total"),
			"The total number of connections closed due to SetMaxIdleConns.",
			nil, nil,
		),
		maxLifetimeClosed: NewDesc(
			fqName("max_lifetime_closed_total"),
			"The total number of connections closed due to SetConnMaxLifetime.",
			nil, nil,
		),
	}
}

// Describe implements Collector.
func (c *dbStatsCollector) Describe(ch chan<- *Desc) {
	ch <- c.maxOpenConnections
	ch <- c.openConnections
	ch <- c.inUseConnections
	ch <- c.idleConnections
	ch <- c.waitCount
	ch <- c.waitDuration
	ch <- c.maxIdleClosed
	ch <- c.maxLifetimeClosed
}

// Collect implements Collector.
func (c *dbStatsCollector) Collect(ch chan<- Metric) {
	stats := c.db.Stats()
	ch <- MustNewConstMetric(c.maxOpenConnections, GaugeValue, float64(stats.MaxOpenConnections))
	ch <- MustNewConstMetric(c.openConnections, GaugeValue, float64(stats.OpenConnections))
	ch <- MustNewConstMetric(c.inUseConnections, GaugeValue, float64(stats.InUse))
	ch <- MustNewConstMetric(c.idleConnections, GaugeValue, float64(stats.Idle))
	ch <- MustNewConstMetric(c.waitCount, CounterValue, float64(stats.WaitCount))
	ch <- MustNewConstMetric(c.waitDuration, CounterValue, stats.WaitDuration.Seconds())
	ch <- MustNewConstMetric(c.maxIdleClosed, CounterValue, float64(stats.MaxIdleClosed))
	ch <- MustNewConstMetric(c.maxLifetimeClosed, CounterValue, float64(stats.MaxLifetimeClosed))
}
