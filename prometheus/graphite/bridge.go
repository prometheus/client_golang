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

// Package graphite provides a bridge to push Prometheus metrics to a Graphite
// server.
package graphite

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/prometheus/common/model"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultInterval       = 15 * time.Second
	millisecondsPerSecond = 1000
)

// HandlerErrorHandling defines how a Handler serving metrics will handle
// errors.
type HandlerErrorHandling int

// These constants cause handlers serving metrics to behave as described if
// errors are encountered.
const (
	// Ignore errors and try to push as many metrics to Graphite as possible.
	ContinueOnError HandlerErrorHandling = iota

	// Abort the push to Graphite upon the first error encountered.
	AbortOnError
)

// Config defines the Graphite bridge config.
type Config struct {
	// Whether to use Graphite tags or not. Defaults to false. Whitespace,
	// control characters, and characters reserved by the Graphite tag syntax
	// in tag names and values are replaced with "_".
	UseTags bool

	// The url to push data to. Required.
	URL string

	// The prefix for the pushed Graphite metrics. Defaults to empty string.
	Prefix string

	// The interval to use for pushing data to Graphite. Defaults to 15 seconds.
	Interval time.Duration

	// The timeout for connecting to Graphite and for writing metrics after
	// connecting. Defaults to 15 seconds.
	Timeout time.Duration

	// The Gatherer to use for metrics. Defaults to prometheus.DefaultGatherer.
	Gatherer prometheus.Gatherer

	// The logger that messages are written to. Defaults to no logging.
	Logger Logger

	// ErrorHandling defines how errors are handled. Note that errors are
	// logged regardless of the configured ErrorHandling provided Logger
	// is not nil.
	ErrorHandling HandlerErrorHandling
}

// Bridge pushes metrics to the configured Graphite server.
type Bridge struct {
	useTags  bool
	url      string
	prefix   string
	interval time.Duration
	timeout  time.Duration

	errorHandling HandlerErrorHandling
	logger        Logger

	g prometheus.Gatherer
}

// Logger is the minimal interface Bridge needs for logging. Note that
// log.Logger from the standard library implements this interface, and it is
// easy to implement by custom loggers, if they don't do so already anyway.
type Logger interface {
	Println(v ...any)
}

// NewBridge returns a pointer to a new Bridge struct.
func NewBridge(c *Config) (*Bridge, error) {
	b := &Bridge{}

	b.useTags = c.UseTags

	if c.URL == "" {
		return nil, errors.New("missing URL")
	}
	b.url = c.URL

	if c.Gatherer == nil {
		b.g = prometheus.DefaultGatherer
	} else {
		b.g = c.Gatherer
	}

	if c.Logger != nil {
		b.logger = c.Logger
	}

	if c.Prefix != "" {
		b.prefix = c.Prefix
	}

	var z time.Duration
	if c.Interval == z {
		b.interval = defaultInterval
	} else {
		b.interval = c.Interval
	}

	if c.Timeout == z {
		b.timeout = defaultInterval
	} else {
		b.timeout = c.Timeout
	}

	b.errorHandling = c.ErrorHandling

	return b, nil
}

// Run starts the event loop that pushes Prometheus metrics to Graphite at the
// configured interval.
func (b *Bridge) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := b.Push(); err != nil && b.logger != nil {
				b.logger.Println("error pushing to Graphite:", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// Push pushes Prometheus metrics to the configured Graphite server.
func (b *Bridge) Push() error {
	mfs, err := b.g.Gather()
	if err != nil {
		switch b.errorHandling {
		case AbortOnError:
			return err
		case ContinueOnError:
			if b.logger != nil {
				b.logger.Println("continue on error:", err)
			}
		default:
			panic("unrecognized error handling value")
		}
	}

	if len(mfs) == 0 {
		return nil
	}

	conn, err := net.DialTimeout("tcp", b.url, b.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetWriteDeadline(time.Now().Add(b.timeout)); err != nil {
		return err
	}

	return writeMetrics(conn, mfs, b.useTags, b.prefix, model.Now())
}

type graphiteLabel struct {
	name  string
	value string
}

type graphiteLabels []graphiteLabel

func (g graphiteLabels) Len() int { return len(g) }

func (g graphiteLabels) Swap(i, j int) { g[i], g[j] = g[j], g[i] }

func (g graphiteLabels) Less(i, j int) bool {
	return concatLess(g[i].name, g[i].value, g[j].name, g[j].value)
}

func concatLess(aName, aValue, bName, bValue string) bool {
	n := len(aName) + 1 + len(aValue)
	bn := len(bName) + 1 + len(bValue)
	if bn < n {
		n = bn
	}
	for i := range n {
		ab := concatAt(aName, aValue, i)
		bb := concatAt(bName, bValue, i)
		if ab != bb {
			return ab < bb
		}
	}
	return len(aName)+1+len(aValue) < len(bName)+1+len(bValue)
}

func concatAt(name, value string, i int) byte {
	if i < len(name) {
		return name[i]
	}
	if i == len(name) {
		return ' '
	}
	return value[i-len(name)-1]
}

func writeMetrics(w io.Writer, mfs []*dto.MetricFamily, useTags bool, prefix string, now model.Time) error {
	for _, mf := range mfs {
		switch mf.GetType() {
		case dto.MetricType_COUNTER, dto.MetricType_GAUGE, dto.MetricType_SUMMARY, dto.MetricType_UNTYPED, dto.MetricType_HISTOGRAM, dto.MetricType_GAUGE_HISTOGRAM:
		default:
			return fmt.Errorf("expfmt.extractSamples: unknown metric family type %v", mf.GetType())
		}
	}

	buf := bufio.NewWriter(w)
	for _, mf := range mfs {
		if err := writeFamily(buf, mf, useTags, prefix, now); err != nil {
			return err
		}
	}
	return nil
}

func writeFamily(buf *bufio.Writer, mf *dto.MetricFamily, useTags bool, prefix string, now model.Time) error {
	name := mf.GetName()
	switch mf.GetType() {
	case dto.MetricType_COUNTER:
		for _, m := range mf.GetMetric() {
			if m.Counter == nil {
				continue
			}
			if err := writeSample(buf, prefix, name, m, nil, m.Counter.GetValue(), useTags, now); err != nil {
				return err
			}
		}
	case dto.MetricType_GAUGE:
		for _, m := range mf.GetMetric() {
			if m.Gauge == nil {
				continue
			}
			if err := writeSample(buf, prefix, name, m, nil, m.Gauge.GetValue(), useTags, now); err != nil {
				return err
			}
		}
	case dto.MetricType_UNTYPED:
		for _, m := range mf.GetMetric() {
			if m.Untyped == nil {
				continue
			}
			if err := writeSample(buf, prefix, name, m, nil, m.Untyped.GetValue(), useTags, now); err != nil {
				return err
			}
		}
	case dto.MetricType_SUMMARY:
		for _, m := range mf.GetMetric() {
			if m.Summary == nil {
				continue
			}
			for _, q := range m.Summary.GetQuantile() {
				extra := []graphiteLabel{{name: string(model.QuantileLabel), value: strconv.FormatFloat(q.GetQuantile(), 'g', -1, 64)}}
				if err := writeSample(buf, prefix, name, m, extra, q.GetValue(), useTags, now); err != nil {
					return err
				}
			}
			if err := writeSample(buf, prefix, name+"_sum", m, nil, m.Summary.GetSampleSum(), useTags, now); err != nil {
				return err
			}
			if err := writeSample(buf, prefix, name+"_count", m, nil, float64(m.Summary.GetSampleCount()), useTags, now); err != nil {
				return err
			}
		}
	case dto.MetricType_HISTOGRAM, dto.MetricType_GAUGE_HISTOGRAM:
		for _, m := range mf.GetMetric() {
			if m.Histogram == nil {
				continue
			}
			infSeen := false
			for _, bucket := range m.Histogram.GetBucket() {
				if isPositiveInf(bucket.GetUpperBound()) {
					infSeen = true
				}
				v := bucket.GetCumulativeCountFloat()
				if v <= 0 {
					v = float64(bucket.GetCumulativeCount())
				}
				extra := []graphiteLabel{{name: string(model.BucketLabel), value: strconv.FormatFloat(bucket.GetUpperBound(), 'g', -1, 64)}}
				if err := writeSample(buf, prefix, name+"_bucket", m, extra, v, useTags, now); err != nil {
					return err
				}
			}
			if err := writeSample(buf, prefix, name+"_sum", m, nil, m.Histogram.GetSampleSum(), useTags, now); err != nil {
				return err
			}
			count := m.Histogram.GetSampleCountFloat()
			if count <= 0 {
				count = float64(m.Histogram.GetSampleCount())
			}
			if err := writeSample(buf, prefix, name+"_count", m, nil, count, useTags, now); err != nil {
				return err
			}
			if !infSeen {
				extra := []graphiteLabel{{name: string(model.BucketLabel), value: "+Inf"}}
				if err := writeSample(buf, prefix, name+"_bucket", m, extra, count, useTags, now); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isPositiveInf(v float64) bool {
	return math.IsInf(v, +1)
}

func writeSample(buf *bufio.Writer, prefix, name string, m *dto.Metric, extra []graphiteLabel, value float64, useTags bool, now model.Time) error {
	for _, c := range prefix {
		if _, err := buf.WriteRune(c); err != nil {
			return err
		}
	}
	if prefix != "" {
		if err := buf.WriteByte('.'); err != nil {
			return err
		}
	}
	if err := writeSanitized(buf, name); err != nil {
		return err
	}
	labels := m.GetLabel()
	if useTags {
		if err := writeTagLabels(buf, labels, extra); err != nil {
			return err
		}
	} else if err := writeDotLabels(buf, labels, extra); err != nil {
		return err
	}
	ts := now
	if m.TimestampMs != nil {
		ts = model.TimeFromUnixNano(*m.TimestampMs * 1000000)
	}
	if err := writeTail(buf, value, int64(ts)/millisecondsPerSecond); err != nil {
		return err
	}
	return buf.Flush()
}

func writeTail(buf *bufio.Writer, value float64, ts int64) error {
	if err := buf.WriteByte(' '); err != nil {
		return err
	}
	var tmp [32]byte
	if _, err := buf.Write(strconv.AppendFloat(tmp[:0], value, 'g', -1, 64)); err != nil {
		return err
	}
	if err := buf.WriteByte(' '); err != nil {
		return err
	}
	if _, err := buf.Write(strconv.AppendInt(tmp[:0], ts, 10)); err != nil {
		return err
	}
	return buf.WriteByte('\n')
}

func writeTagLabels(buf *bufio.Writer, labels []*dto.LabelPair, extra []graphiteLabel) error {
	for _, label := range labels {
		if err := writeOneTag(buf, label.GetName(), label.GetValue()); err != nil {
			return err
		}
	}
	for _, label := range extra {
		if err := writeOneTag(buf, label.name, label.value); err != nil {
			return err
		}
	}
	return nil
}

func writeOneTag(buf *bufio.Writer, name, value string) error {
	if _, err := buf.WriteRune(';'); err != nil {
		return err
	}
	if _, err := buf.WriteString(sanitizeTag(name, ";!^=")); err != nil {
		return err
	}
	if _, err := buf.WriteRune('='); err != nil {
		return err
	}
	v := sanitizeTag(value, ";")
	if strings.HasPrefix(v, "~") {
		v = "_" + v[1:]
	}
	_, err := buf.WriteString(v)
	return err
}

func writeDotLabels(buf *bufio.Writer, labels []*dto.LabelPair, extra []graphiteLabel) error {
	pairs := make(graphiteLabels, 0, len(labels)+len(extra))
	for _, label := range labels {
		pairs = append(pairs, graphiteLabel{name: label.GetName(), value: label.GetValue()})
	}
	pairs = append(pairs, extra...)
	sort.Sort(pairs)
	for _, label := range pairs {
		if err := buf.WriteByte('.'); err != nil {
			return err
		}
		if err := writeSanitized(buf, label.name); err != nil {
			return err
		}
		if err := buf.WriteByte('.'); err != nil {
			return err
		}
		if err := writeSanitized(buf, label.value); err != nil {
			return err
		}
	}
	return nil
}

// sanitizeTag replaces whitespace, control characters, and the given
// forbidden characters with '_'. This prevents a tag name or value from
// breaking the plaintext protocol line, e.g. by injecting a newline.
func sanitizeTag(s, forbidden string) string {
	return strings.Map(func(c rune) rune {
		if unicode.IsSpace(c) || unicode.IsControl(c) || strings.ContainsRune(forbidden, c) {
			return '_'
		}
		return c
	}, s)
}

func writeSanitized(buf *bufio.Writer, s string) error {
	prevUnderscore := false

	for _, c := range s {
		c = replaceInvalidRune(c)
		if c == '_' {
			if prevUnderscore {
				continue
			}
			prevUnderscore = true
		} else {
			prevUnderscore = false
		}
		if _, err := buf.WriteRune(c); err != nil {
			return err
		}
	}

	return nil
}

func replaceInvalidRune(c rune) rune {
	if c == ' ' {
		return '.'
	}
	// !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == ':' || c == '-' || (c >= '0' && c <= '9'))
	if (c < 'a' || c > 'z') &&
		(c < 'A' || c > 'Z') &&
		c != '_' &&
		c != ':' &&
		c != '-' &&
		(c < '0' || c > '9') {
		return '_'
	}
	return c
}
