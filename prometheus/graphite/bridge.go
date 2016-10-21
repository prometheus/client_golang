// Copyright 2016 The Prometheus Authors
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
package graphite

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	dto "github.com/prometheus/client_model/go"
)

// Config defines the graphite bridge config.
type Config struct {
	// The url to push data to. Required.
	URL string

	// The interval to use for pushing data to Graphite. Defaults to 15 seconds.
	Interval time.Duration

	// The Gatherer to use for metrics. Defaults to prometheus.DefaultGatherer.
	Gatherer prometheus.Gatherer

	// The prefix for your graphite metric. Defaults to empty string.
	Prefix string
}

// Bridge pushes metrics to the configured graphite server.
type Bridge struct {
	url      string
	interval time.Duration
	prefix   string
	stopc    chan struct{}

	g prometheus.Gatherer
}

func NewBridge(c *Config) (*Bridge, error) {
	b := &Bridge{
		stopc: make(chan struct{}),
	}

	if c.URL == "" {
		return nil, errors.New("graphite bridge: no url given")
	}
	b.url = c.URL

	if c.Gatherer == nil {
		b.g = prometheus.DefaultGatherer
	} else {
		b.g = c.Gatherer
	}

	if c.Prefix != "" {
		b.prefix = c.Prefix
	}

	var z time.Duration
	if c.Interval == z {
		b.interval = 15 * time.Second
	} else {
		b.interval = c.Interval
	}

	return b, nil
}

// Stop stops the event loop.
func (b *Bridge) Stop() {
	b.stopc <- struct{}{}
}

// Loop starts the event loop that pushes metrics at the configured interval.
func (b *Bridge) Loop() {
	ticker := time.NewTicker(b.interval)
	for {
		select {
		case <-ticker.C:
			if err := b.Push(); err != nil {
				// TODO: Use the right logger.
				log.Printf("%v", err)
			}
		case <-b.stopc:
			return
		}
	}
}

func (b *Bridge) Push() error {
	mfs, err := b.g.Gather()
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	// TODO: Write directly to conn?
	buf, err := toReader(mfs, b.prefix, now)
	if err != nil {
		return err
	}

	// TODO: Should we expose a deadline to the user?
	conn, err := net.Dial("tcp", b.url)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = io.Copy(conn, buf)
	return err
}

// TODO: Do we want to allow partial writes? i.e., Skip a failed metric, but
// try to write all metrics?
func toReader(mfs []*dto.MetricFamily, prefix string, now int64) (*bytes.Buffer, error) {
	// TODO: Snag the buffer pool from promhttp/http.go
	var (
		buf bytes.Buffer
		err error
	)

	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			sort.Sort(prometheus.LabelPairSorter(m.GetLabel()))

			parts := []string{prefix, mf.GetName()}
			for _, lp := range m.GetLabel() {
				parts = append(parts, sanitize(lp.GetName())+"."+sanitize(lp.GetValue()))
			}

			switch mf.GetType() {
			case dto.MetricType_SUMMARY:
				if summary := m.GetSummary(); summary != nil {
					_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(append(parts, "count"), "."), float64(summary.GetSampleCount()), now))
					if err != nil {
						return nil, err
					}
					_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(append(parts, "sum"), "."), summary.GetSampleSum(), now))
					if err != nil {
						return nil, err
					}

					for _, q := range summary.GetQuantile() {
						quantile := fmt.Sprintf("quantile.%g", *q.Quantile*100)
						_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(append(parts, quantile), "."), q.GetValue(), now))
						if err != nil {
							return nil, err
						}
					}
				}
			case dto.MetricType_HISTOGRAM:
				if histogram := m.GetHistogram(); histogram != nil {
					_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(append(parts, "count"), "."), float64(histogram.GetSampleCount()), now))
					if err != nil {
						return nil, err
					}
					_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(append(parts, "sum"), "."), histogram.GetSampleSum(), now))
					if err != nil {
						return nil, err
					}

					for _, b := range histogram.GetBucket() {
						bucket := fmt.Sprintf("bucket.%g", b.GetUpperBound())
						_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(append(parts, bucket), "."), float64(b.GetCumulativeCount()), now))
						if err != nil {
							return nil, err
						}
					}
				}
			default:
				// TODO: Do we want to allow partial writes? i.e., do
				// we want to attempt to parse later metrics if an
				// earlier one fails?
				_, err = buf.WriteString(fmt.Sprintf(graphiteFormatString, strings.Join(parts, "."), getValue(m), now))
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return &buf, nil
}

var re = regexp.MustCompile("[^a-zA-Z0-9_-]")

const graphiteFormatString = "%s %g %d\n"

func sanitize(s string) string {
	return re.ReplaceAllString(s, "_")
}

func getValue(m *dto.Metric) float64 {
	if m.GetGauge() != nil {
		return m.GetGauge().GetValue()
	}
	if m.GetCounter() != nil {
		return m.GetCounter().GetValue()
	}
	if m.GetUntyped() != nil {
		return m.GetUntyped().GetValue()
	}

	return 0
}
