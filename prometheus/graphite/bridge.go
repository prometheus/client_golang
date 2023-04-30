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
	"net"
	"sort"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultInterval       = 15 * time.Second
	millisecondsPerSecond = 1000
)

// ErrorHandler is a function that handles errors
type ErrorHandler func(err error)

// DefaultErrorHandler skips received errors
var DefaultErrorHandler = func(err error) {}

// Config defines the Graphite bridge config.
type Config struct {
	// Whether to use Graphite tags or not. Defaults to false.
	UseTags bool

	// The url to push data to. Required.
	URL string

	// The prefix for the pushed Graphite metrics. Defaults to empty string.
	Prefix string

	// The interval to use for pushing data to Graphite. Defaults to 15 seconds.
	Interval time.Duration

	// The timeout for pushing metrics to Graphite. Defaults to 15 seconds.
	Timeout time.Duration

	// The Gatherer to use for metrics. Defaults to prometheus.DefaultGatherer.
	Gatherer prometheus.Gatherer

	// ErrorHandler defines how errors are handled.
	ErrorHandler ErrorHandler
}

// Bridge pushes metrics to the configured Graphite server.
type Bridge struct {
	useTags  bool
	url      string
	prefix   string
	interval time.Duration
	timeout  time.Duration

	errorHandler ErrorHandler

	g prometheus.Gatherer
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

	b.errorHandler = c.ErrorHandler

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
			b.errorHandler(b.Push())
		case <-ctx.Done():
			return
		}
	}
}

// Push pushes Prometheus metrics to the configured Graphite server.
func (b *Bridge) Push() error {
	mfs, err := b.g.Gather()
	if err != nil {
		return err
	}

	if len(mfs) == 0 {
		return nil
	}

	conn, err := net.DialTimeout("tcp", b.url, b.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	return writeMetrics(conn, mfs, b.useTags, b.prefix, model.Now())
}

func writeMetrics(w io.Writer, mfs []*dto.MetricFamily, useTags bool, prefix string, now model.Time) error {
	vec, err := expfmt.ExtractSamples(&expfmt.DecodeOptions{
		Timestamp: now,
	}, mfs...)
	if err != nil {
		return err
	}

	buf := bufio.NewWriter(w)
	for _, s := range vec {
		if prefix != "" {
			for _, c := range prefix {
				if _, err := buf.WriteRune(c); err != nil {
					return err
				}
			}
			if err := buf.WriteByte('.'); err != nil {
				return err
			}
		}
		if err := writeMetric(buf, s.Metric, useTags); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(buf, " %g %d\n", s.Value, int64(s.Timestamp)/millisecondsPerSecond); err != nil {
			return err
		}
		if err := buf.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func writeMetric(buf *bufio.Writer, m model.Metric, useTags bool) error {
	metricName, hasName := m[model.MetricNameLabel]
	numLabels := len(m) - 1
	if !hasName {
		numLabels = len(m)
	}

	var err error
	switch numLabels {
	case 0:
		if hasName {
			return writeSanitized(buf, string(metricName))
		}
	default:
		if err = writeSanitized(buf, string(metricName)); err != nil {
			return err
		}
		if useTags {
			return writeTags(buf, m)
		}
		return writeLabels(buf, m, numLabels)
	}
	return nil
}

func writeTags(buf *bufio.Writer, m model.Metric) error {
	for label, value := range m {
		if label != model.MetricNameLabel {
			buf.WriteRune(';')
			if _, err := buf.WriteString(string(label)); err != nil {
				return err
			}
			buf.WriteRune('=')
			if _, err := buf.WriteString(string(value)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeLabels(buf *bufio.Writer, m model.Metric, numLabels int) error {
	labelStrings := make([]string, 0, numLabels)
	for label, value := range m {
		if label != model.MetricNameLabel {
			labelString := string(label) + " " + string(value)
			labelStrings = append(labelStrings, labelString)
		}
	}
	sort.Strings(labelStrings)
	for _, s := range labelStrings {
		if err := buf.WriteByte('.'); err != nil {
			return err
		}
		if err := writeSanitized(buf, s); err != nil {
			return err
		}
	}
	return nil
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
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == ':' || c == '-' || (c >= '0' && c <= '9')) {
		return '_'
	}
	return c
}
