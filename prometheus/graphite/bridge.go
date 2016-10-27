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
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/model"
)

// Config defines the graphite bridge config.
type Config struct {
	// The url to push data to. Required.
	URL string

	// The interval to use for pushing data to Graphite. Defaults to 15 seconds.
	Interval time.Duration

	// The Gatherer to use for metrics. Defaults to prometheus.DefaultGatherer.
	Gatherer prometheus.Gatherer

	// The logger that messages are written to. Defaults to log.Base().
	Logger log.Logger

	// The prefix for your graphite metric. Defaults to empty string.
	Prefix string
}

// Bridge pushes metrics to the configured graphite server.
type Bridge struct {
	url      string
	interval time.Duration
	prefix   string

	g      prometheus.Gatherer
	logger log.Logger
}

// NewBridge returns a pointer to a new Bridge struct.
func NewBridge(c *Config) (*Bridge, error) {
	b := &Bridge{}

	if c.URL == "" {
		return nil, errors.New("graphite bridge: no url given")
	}
	b.url = c.URL

	if c.Gatherer == nil {
		b.g = prometheus.DefaultGatherer
	} else {
		b.g = c.Gatherer
	}

	if c.Logger == nil {
		b.logger = log.Base()
	} else {
		b.logger = c.Logger
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

// Run starts the event loop that pushes Prometheus metrics to Graphite at the
// configured interval.
func (b *Bridge) Run(ctx context.Context) {
	ticker := time.NewTicker(b.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := b.Push(); err != nil {
				b.logger.Errorf("%v", err)
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
	vec := Vector{
		Vector: expfmt.ExtractSamples(&expfmt.DecodeOptions{
			Timestamp: model.Time(now),
		}, mfs...),
		prefix: prefix,
	}

	return bytes.NewBufferString(vec.String()), nil
}

type Vector struct {
	prefix string

	model.Vector
}

func (vec Vector) String() string {
	entries := make([]string, len(vec.Vector))
	for i, s := range vec.Vector {
		// TODO: Should be a better way to add the prefix
		entries[i] = strings.Join([]string{vec.prefix, Sample(*s).String()}, ".")
	}
	return strings.Join(entries, "\n")
}

type Sample model.Sample

func (s Sample) String() string {
	return fmt.Sprintf("%s %g %d",
		Metric(s.Metric),
		s.Value,
		int64(s.Timestamp),
	)
}

type Metric model.Metric

func (m Metric) String() string {
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
