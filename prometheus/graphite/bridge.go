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
		b.prefix = c.Prefix + "."
	}

	var z time.Duration
	if c.Interval == z {
		b.interval = 15 * time.Second
	} else {
		b.interval = c.Interval
	}

	go b.loop()

	return b, nil
}

func (b *Bridge) Stop() {
	b.stopc <- struct{}{}
}

func (b *Bridge) loop() {
	ticker := time.NewTicker(b.interval)
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

	_, err = io.Copy(conn, &buf)
	return err
}

// TODO: Do we want to allow partial writes? i.e., Skip a failed metric, but
// try to write all metrics?
func toReader(mfs []*dto.MetricFamily, prefix string, now int64) (bytes.Buffer, error) {
	// TODO: Snag the buffer pool from promhttp/http.go
	var buf bytes.Buffer

	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			sort.Sort(prometheus.LabelPairSorter(m.GetLabel()))
			_, err := buf.WriteString(prefix + mf.GetName())
			if err != nil {
				return buf, err
			}

			labels := []string{}
			for _, lp := range m.GetLabel() {
				labels = append(labels, sanitize(lp.GetName())+"."+sanitize(lp.GetValue()))
			}

			value := getValue(m)
			// TODO: Do we want to allow partial writes? i.e., do
			// we want to attempt to parse later metrics if an
			// earlier one fails?
			_, err = buf.WriteString(fmt.Sprintf("%s %g %d\n", strings.Join(labels, "."), value, now))
			if err != nil {
				return buf, err
			}
		}
	}

	return buf, nil
}

var re = regexp.MustCompile("[^a-zA-Z0-9_-]")

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
	// How do we deal with the sum and the count and quantile?
	// if m.GetSummary() != nil {
	// 	return *m.GetSummary().GetValue()
	// }
	if m.GetUntyped() != nil {
		return m.GetUntyped().GetValue()
	}
	// How do we deal with the sum and the count and quantile?
	// if m.GetHistogram() != nil {
	// 	return *m.GetHistogram().GetValue()
	// }

	return 0
}
