// Copyright 2014 Prometheus Team
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

// Package text contains functions to parse and create the simple and flat
// text-based exchange format.
package text

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

// MetricFamilyToText converts a MetricFamily proto message into text format and
// writes the resulting lines to 'out'. It returns the number of bytes written
// and any error encountered.  This function does not perform checks on the
// content of the metric and label names, i.e. invalid metric or label names
// will result in invalid text format output.
func MetricFamilyToText(in *dto.MetricFamily, out io.Writer) (int, error) {
	var written int

	// Fail-fast checks.
	if len(in.Metric) == 0 {
		return written, fmt.Errorf("MetricFamily has no metrics: %s", in)
	}
	name := in.GetName()
	if name == "" {
		return written, fmt.Errorf("MetricFamily has no name: %s", in)
	}
	if in.Type == nil {
		return written, fmt.Errorf("MetricFamily has no type: %s", in)
	}

	// Comments, first HELP, then TYPE.
	if in.Help != nil {
		n, err := fmt.Fprintf(
			out, "# HELP %s %s\n",
			name, strings.Replace(*in.Help, "\n", `\n`, -1))
		written += n
		if err != nil {
			return written, err
		}
	}
	metricType := in.GetType()
	n, err := fmt.Fprintf(
		out, "# TYPE %s %s\n",
		name, strings.ToLower(metricType.String()),
	)
	written += n
	if err != nil {
		return written, err
	}

	// Finally the samples, one line for each.
	for _, metric := range in.Metric {
		switch metricType {
		case dto.MetricType_COUNTER:
			if metric.Counter == nil {
				return written, fmt.Errorf(
					"expected counter in metric %s", metric,
				)
			}
			n, err = writeSample(
				name, metric, "", "",
				metric.Counter.GetValue(),
				out,
			)
		case dto.MetricType_GAUGE:
			if metric.Gauge == nil {
				return written, fmt.Errorf(
					"expected gauge in metric %s", metric,
				)
			}
			n, err = writeSample(
				name, metric, "", "",
				metric.Gauge.GetValue(),
				out,
			)
		case dto.MetricType_CUSTOM:
			if metric.Custom == nil {
				return written, fmt.Errorf(
					"expected custom in metric %s", metric,
				)
			}
			n, err = writeSample(
				name, metric, "", "",
				metric.Custom.GetValue(),
				out,
			)
		case dto.MetricType_SUMMARY:
			if metric.Summary == nil {
				return written, fmt.Errorf(
					"expected summary in metric %s", metric,
				)
			}
			for _, q := range metric.Summary.Quantile {
				n, err = writeSample(
					name, metric,
					"quantile", fmt.Sprint(q.GetQuantile()),
					q.GetValue(),
					out,
				)
				if err != nil {
					return written, err
				}
				written += n
			}
			n, err = writeSample(
				name+"_sum", metric, "", "",
				metric.Summary.GetSampleSum(),
				out,
			)
			if err != nil {
				return written, err
			}
			written += n
			n, err = writeSample(
				name+"_count", metric, "", "",
				float64(metric.Summary.GetSampleCount()),
				out,
			)
		default:
			return written, fmt.Errorf(
				"unexpected type in metric %s", metric,
			)
		}
		if err != nil {
			return written, err
		}
		written += n
	}
	return written, nil
}

// writeSample writes a single sample in text formet to out, given the metric
// name, the metric proto message itself, optionally an additonal label name and
// value (use empty strings if not required), and the value. The function
// returns the number of bytes written and any error encountered.
func writeSample(
	name string,
	metric *dto.Metric,
	additionalLabelName, additionalLabelValue string,
	value float64,
	out io.Writer,
) (int, error) {
	var written int
	n, err := fmt.Fprint(out, name)
	if err != nil {
		return written, err
	}
	written += n
	n, err = labelPairsToText(
		metric.Label,
		additionalLabelName, additionalLabelValue,
		out,
	)
	if err != nil {
		return written, err
	}
	written += n
	n, err = fmt.Fprintf(out, " %v", value)
	if err != nil {
		return written, err
	}
	written += n
	if metric.TimestampMs != nil {
		n, err = fmt.Fprintf(out, " %v", *metric.TimestampMs)
		if err != nil {
			return written, err
		}
		written += n
	}
	n, err = out.Write([]byte{'\n'})
	if err != nil {
		return written, err
	}
	written += n
	return written, nil
}

// labelPairsToText converts a slice of LabelPair proto messages plus the
// explicitly given additional label pair into text formatted as required by the
// text format and writes it to 'out'. An empty slice in combination with an
// empty string al 'additionalLabelName' results in nothing being
// written. Otherwise, the label pairs are written, escaped as required by the
// text format, and enclosed in '{...}'. The function returns the number of
// bytes written and any error encountered.
func labelPairsToText(
	in []*dto.LabelPair,
	additionalLabelName, additionalLabelValue string,
	out io.Writer,
) (int, error) {
	if len(in) == 0 && additionalLabelName == "" {
		return 0, nil
	}
	var written int
	separator := '{'
	for _, lp := range in {
		n, err := fmt.Fprintf(
			out, `%c%s="%s"`,
			separator, lp.GetName(), escapeLabelValue(lp.GetValue()),
		)
		if err != nil {
			return written, err
		}
		written += n
		separator = ','
	}
	if additionalLabelName != "" {
		n, err := fmt.Fprintf(
			out, `%c%s="%s"`,
			separator, additionalLabelName,
			escapeLabelValue(additionalLabelValue),
		)
		if err != nil {
			return written, err
		}
		written += n
	}
	n, err := out.Write([]byte{'}'})
	if err != nil {
		return written, err
	}
	written += n
	return written, nil
}

// escapeLabelValue replaces '\' by '\\', '"' by '\"', and new line character by '\n'.
func escapeLabelValue(v string) string {
	result := bytes.NewBuffer(make([]byte, 0, len(v)))
	for _, c := range v {
		switch c {
		case '\\':
			result.WriteString(`\\`)
		case '"':
			result.WriteString(`\"`)
		case '\n':
			result.WriteString(`\n`)
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}
