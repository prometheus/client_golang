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
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"golang.org/x/net/context"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultInterval               = 15 * time.Second
	millisecondsPerSecond float64 = 1000
)

// HandlerErrorHandling defines how a Handler serving metrics will handle
// errors.
type HandlerErrorHandling int

// These constants cause handlers serving metrics to behave as described if
// errors are encountered.
const (
	// Abort the push to Graphite upon the first error encountered.
	AbortOnError HandlerErrorHandling = iota

	// Ignore errors and try to push as many metrics to Graphite as possible.
	ContinueOnError
)

// Config defines the Graphite bridge config.
type Config struct {
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

	// The logger that messages are written to. Defaults to log.Base().
	Logger Logger

	// ErrorHandling defines how errors are handled. Note that errors are
	// logged regardless of the configured ErrorHandling provided Logger
	// is not nil.
	ErrorHandling HandlerErrorHandling
}

// Bridge pushes metrics to the configured Graphite server.
type Bridge struct {
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
	Printf(v ...interface{})
}

// NewBridge returns a pointer to a new Bridge struct.
func NewBridge(c *Config) (*Bridge, error) {
	b := &Bridge{}

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
			if err := b.Push(); err != nil {
				if b.logger != nil {
					b.logger.Printf("%v", err)
				}
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
				b.logger.Printf("continue on error: %v", err)
			}
		}
	}

	conn, err := net.DialTimeout("tcp", b.url, b.timeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = writeMetrics(conn, mfs, b.prefix, model.Now())
	if err != nil {
		return err
	}

	return err
}

func writeMetrics(w io.Writer, mfs []*dto.MetricFamily, prefix string, now model.Time) (int, error) {
	vec := expfmt.ExtractSamples(&expfmt.DecodeOptions{
		Timestamp: now,
	}, mfs...)

	var total int
	for _, s := range vec {
		n, err := fmt.Fprintf(w, "%s %g %.3f\n", strings.Join([]string{sanitize(prefix), toMetric(s.Metric)}, "."), s.Value, float64(s.Timestamp)/millisecondsPerSecond)
		if err != nil {
			return 0, err
		}
		total += n
	}

	return total, nil
}

func toMetric(m model.Metric) string {
	metricName, hasName := m[model.MetricNameLabel]
	numLabels := len(m) - 1
	if !hasName {
		numLabels = len(m)
	}

	labelStrings := make([]string, 0, numLabels)
	for label, value := range m {
		if label != model.MetricNameLabel {
			labelStrings = append(labelStrings, fmt.Sprintf("%s.%s", sanitize(string(label)), sanitize(string(value))))
		}
	}

	switch numLabels {
	case 0:
		if hasName {
			return sanitize(string(metricName))
		}
		return ""
	default:
		sort.Strings(labelStrings)
		return fmt.Sprintf("%s.%s", sanitize(string(metricName)), strings.Join(labelStrings, "."))
	}
}

var (
	reInvalidChars       = regexp.MustCompile("[^a-zA-Z0-9_-]")
	reRepeatedUnderscore = regexp.MustCompile("_{2,}")
)

func sanitize(s string) string {
	return strings.Trim(
		strings.ToLower(
			reRepeatedUnderscore.ReplaceAllString(
				reInvalidChars.ReplaceAllString(s, "_"), "_"),
		), "_")
}
