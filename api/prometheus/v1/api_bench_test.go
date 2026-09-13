// Copyright 2019 The Prometheus Authors
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
package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/prometheus/common/model"
)

func generateData(timeseries, datapoints int) (floatMatrix, histogramMatrix model.Matrix) {
	for i := 0; i < timeseries; i++ {
		lset := map[model.LabelName]model.LabelValue{
			model.MetricNameLabel: model.LabelValue("timeseries_" + strconv.Itoa(i)),
			"foo":                 "bar",
		}
		now := model.Time(1677587274055)
		floats := make([]model.SamplePair, datapoints)
		histograms := make([]model.SampleHistogramPair, datapoints)

		for x := datapoints; x > 0; x-- {
			f := float64(x)
			floats[x-1] = model.SamplePair{
				// Set the time back assuming a 15s interval. Since this is used for
				// Marshal/Unmarshal testing the actual interval doesn't matter.
				Timestamp: now.Add(time.Second * -15 * time.Duration(x)),
				Value:     model.SampleValue(f),
			}
			histograms[x-1] = model.SampleHistogramPair{
				Timestamp: now.Add(time.Second * -15 * time.Duration(x)),
				Histogram: &model.SampleHistogram{
					Count: model.FloatString(13.5 * f),
					Sum:   model.FloatString(.1 * f),
					Buckets: model.HistogramBuckets{
						{
							Boundaries: 1,
							Lower:      -4870.992343051145,
							Upper:      -4466.7196729968955,
							Count:      model.FloatString(1 * f),
						},
						{
							Boundaries: 1,
							Lower:      -861.0779292198035,
							Upper:      -789.6119426088657,
							Count:      model.FloatString(2 * f),
						},
						{
							Boundaries: 1,
							Lower:      -558.3399591246119,
							Upper:      -512,
							Count:      model.FloatString(3 * f),
						},
						{
							Boundaries: 0,
							Lower:      2048,
							Upper:      2233.3598364984477,
							Count:      model.FloatString(1.5 * f),
						},
						{
							Boundaries: 0,
							Lower:      2896.3093757400984,
							Upper:      3158.4477704354626,
							Count:      model.FloatString(2.5 * f),
						},
						{
							Boundaries: 0,
							Lower:      4466.7196729968955,
							Upper:      4870.992343051145,
							Count:      model.FloatString(3.5 * f),
						},
					},
				},
			}
		}

		fss := &model.SampleStream{
			Metric: model.Metric(lset),
			Values: floats,
		}
		hss := &model.SampleStream{
			Metric:     model.Metric(lset),
			Histograms: histograms,
		}

		floatMatrix = append(floatMatrix, fss)
		histogramMatrix = append(histogramMatrix, hss)
	}
	return floatMatrix, histogramMatrix
}

func BenchmarkSamplesJsonSerialization(b *testing.B) {
	for _, timeseriesCount := range []int{10, 100, 1000} {
		b.Run("series="+strconv.Itoa(timeseriesCount), func(b *testing.B) {
			for _, datapointCount := range []int{10, 100, 1000} {
				b.Run("dp="+strconv.Itoa(datapointCount), func(b *testing.B) {
					floats, histograms := generateData(timeseriesCount, datapointCount)

					floatBytes, err := json.Marshal(floats)
					if err != nil {
						b.Fatalf("Error marshaling: %v", err)
					}
					histogramBytes, err := json.Marshal(histograms)
					if err != nil {
						b.Fatalf("Error marshaling: %v", err)
					}

					b.Run("op=marshal", func(b *testing.B) {
						b.Run("type=floats", func(b *testing.B) {
							b.Run("encoder=json", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									if _, err := json.Marshal(floats); err != nil {
										b.Fatal(err)
									}
								}
							})
							if supportsJSONv2 {
								b.Run("encoder=jsonv2", func(b *testing.B) {
									b.ReportAllocs()
									for b.Loop() {
										if _, err := jsonv2Marshal(floats); err != nil {
											b.Fatal(err)
										}
									}
								})
							}
							b.Run("encoder=jsoniter", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									if _, err := jsoniter.Marshal(floats); err != nil {
										b.Fatal(err)
									}
								}
							})
						})
						b.Run("type=histograms", func(b *testing.B) {
							b.Run("encoder=json", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									if _, err := json.Marshal(histograms); err != nil {
										b.Fatal(err)
									}
								}
							})
							if supportsJSONv2 {
								b.Run("encoder=jsonv2", func(b *testing.B) {
									b.ReportAllocs()
									for b.Loop() {
										if _, err := jsonv2Marshal(histograms); err != nil {
											b.Fatal(err)
										}
									}
								})
							}
							b.Run("encoder=jsoniter", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									if _, err := jsoniter.Marshal(histograms); err != nil {
										b.Fatal(err)
									}
								}
							})
						})
					})

					b.Run("op=unmarshal", func(b *testing.B) {
						b.Run("type=floats", func(b *testing.B) {
							b.Run("encoder=json", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									var m model.Matrix
									if err := json.Unmarshal(floatBytes, &m); err != nil {
										b.Fatal(err)
									}
								}
							})
							if supportsJSONv2 {
								b.Run("encoder=jsonv2", func(b *testing.B) {
									b.ReportAllocs()
									for b.Loop() {
										var m model.Matrix
										if err := jsonv2Unmarshal(floatBytes, &m); err != nil {
											b.Fatal(err)
										}
									}
								})
							}
							b.Run("encoder=jsoniter", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									var m model.Matrix
									if err := jsoniter.Unmarshal(floatBytes, &m); err != nil {
										b.Fatal(err)
									}
								}
							})
						})
						b.Run("type=histograms", func(b *testing.B) {
							b.Run("encoder=json", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									var m model.Matrix
									if err := json.Unmarshal(histogramBytes, &m); err != nil {
										b.Fatal(err)
									}
								}
							})
							if supportsJSONv2 {
								b.Run("encoder=jsonv2", func(b *testing.B) {
									b.ReportAllocs()
									for b.Loop() {
										var m model.Matrix
										if err := jsonv2Unmarshal(histogramBytes, &m); err != nil {
											b.Fatal(err)
										}
									}
								})
							}
							b.Run("encoder=jsoniter", func(b *testing.B) {
								b.ReportAllocs()
								for b.Loop() {
									var m model.Matrix
									if err := jsoniter.Unmarshal(histogramBytes, &m); err != nil {
										b.Fatal(err)
									}
								}
							})
						})
					})
				})
			}
		})
	}
}

func BenchmarkAPIResponse(b *testing.B) {
	type apiResponseTest struct {
		name string
		data []byte
	}

	var testcases []apiResponseTest
	addTestcase := func(name string, v any) {
		data, err := json.Marshal(v)
		if err != nil {
			b.Fatal(err)
		}
		testcases = append(testcases, apiResponseTest{
			name: name,
			data: data,
		})
	}

	addTestcase("AlertsResult", AlertsResult{Alerts: []Alert{{
		ActiveAt:    time.Unix(1, 0),
		Annotations: model.LabelSet{"key": "value"},
		Labels:      model.LabelSet{"key": "value"},
		State:       AlertStateFiring,
		Value:       "somevalue",
	}}})
	addTestcase("AlertManagersResult", AlertManagersResult{
		Active:  []AlertManager{{URL: "https://example.com"}},
		Dropped: []AlertManager{{URL: "https://example.com"}},
	})
	addTestcase("ConfigResult", ConfigResult{
		YAML: "somekey: somevalue",
	})
	addTestcase("FlagsResult", FlagsResult{"key": "value"})
	addTestcase("BuildinfoResult", BuildinfoResult{
		Version:   "1.0.0",
		Revision:  "v12",
		Branch:    "dev",
		BuildUser: "default",
		BuildDate: "2026-01-02",
		GoVersion: "1.26.6",
	})
	addTestcase("RuntimeinfoResult", RuntimeinfoResult{})
	addTestcase("model.LabelValues", model.LabelValues{"value1", "value2"})
	addTestcase("SnapshotResult", SnapshotResult{Name: "name"})
	addTestcase("TargetsResult", TargetsResult{
		Active:  []ActiveTarget{},
		Dropped: []DroppedTarget{},
	})
	addTestcase("[]MetricMetadata", []MetricMetadata{{
		Target: map[string]string{"key": "value"},
		Metric: "mymetric",
		Type:   MetricTypeGauge,
		Help:   "help text",
		Unit:   "unit",
	}})
	addTestcase("map[string][]Metadata", map[string][]Metadata{
		"default": {{
			Type: "default",
			Help: "help text",
			Unit: "unit",
		}},
	})
	addTestcase("TSDBResult", TSDBResult{
		HeadStats: TSDBHeadStats{
			NumSeries:     1000,
			NumLabelPairs: 1000,
			ChunkCount:    10,
			MinTime:       1,
			MaxTime:       1000,
		},
		SeriesCountByMetricName:     []Stat{{Name: "statname", Value: 12345}},
		LabelValueCountByLabelName:  []Stat{{Name: "statname", Value: 12345}},
		MemoryInBytesByLabelName:    []Stat{{Name: "statname", Value: 12345}},
		SeriesCountByLabelValuePair: []Stat{{Name: "statname", Value: 12345}},
	})
	addTestcase("TSDBBlocksResult", TSDBBlocksResult{
		Status: "ok",
		Data: TSDBBlocksData{
			Blocks: []TSDBBlocksBlockMetadata{{
				Ulid:    "ulid",
				MinTime: 1,
				MaxTime: 1000,
				Stats: TSDBBlocksStats{
					NumSamples: 1000,
					NumSeries:  1000,
					NumChunks:  1000,
				},
				Compaction: TSDBBlocksCompaction{
					Level:   1234,
					Sources: []string{"sourcea"},
				},
				Version: 1,
			}},
		},
	})
	addTestcase("WalReplayStatus", WalReplayStatus{Min: 1, Max: 1000, Current: 500})
	addTestcase("[]ExemplarQueryResult", []ExemplarQueryResult{{
		SeriesLabels: model.LabelSet{"key": "value"},
		Exemplars: []Exemplar{{
			Labels:    model.LabelSet{"key": "value"},
			Value:     model.SampleValue(1234.567),
			Timestamp: model.Time(1234),
		}},
	}})

	for _, size := range []int{10, 100, 1000} {
		floats, histograms := generateData(size, size)
		addTestcase(fmt.Sprintf("floats-%d", size), floats)
		addTestcase(fmt.Sprintf("histograms-%d", size), histograms)
	}

	for _, tc := range testcases {
		data, err := json.Marshal(apiResponse{Status: "ok", Data: tc.data})
		if err != nil {
			b.Fatal(err)
		}
		b.Log(string(data))
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				var r apiResponse
				if err := json.Unmarshal(data, &r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRuleGroup(b *testing.B) {
	alertingRuleJSON, err := json.Marshal(struct {
		Type         RuleType `json:"type"`
		AlertingRule `json:""`
	}{
		Type: RuleTypeAlerting,
		AlertingRule: AlertingRule{
			Name:        "HighRequestLatency",
			Query:       "job:request_latency_seconds:mean5m{job=\"myjob\"} > 0.5",
			Duration:    600,
			Labels:      model.LabelSet{"severity": "page"},
			Annotations: model.LabelSet{"summary": "High request latency"},
			Alerts: []*Alert{{
				ActiveAt:    time.Now().UTC(),
				Annotations: model.LabelSet{"summary": "High request latency"},
				Labels:      model.LabelSet{"alertname": "HighRequestLatency", "severity": "page"},
				State:       AlertStateFiring,
				Value:       "1e+00",
			}},
			Health:         RuleHealthGood,
			LastError:      "Unknown",
			EvaluationTime: 1,
			LastEvaluation: time.Now().Round(time.Millisecond).UTC(),
			State:          "state",
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	b.Log("alerting:", string(alertingRuleJSON))

	recordingRuleJSON, err := json.Marshal(struct {
		Type          RuleType `json:"type"`
		RecordingRule `json:""`
	}{
		Type: RuleTypeRecording,
		RecordingRule: RecordingRule{
			Name:           "job:http_inprogress_requests:sum",
			Query:          "sum(http_inprogress_requests) by (job)",
			Labels:         model.LabelSet{"severity": "page"},
			Health:         RuleHealthGood,
			LastError:      "Unknown",
			EvaluationTime: 1,
			LastEvaluation: time.Now().Round(time.Millisecond).UTC(),
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	b.Log("recording:", string(recordingRuleJSON))

	data := []byte(`{
"name":"myname","file":"myfile","interval":0.0000005,"rules":[` +
		string(alertingRuleJSON) + strings.Repeat(","+string(alertingRuleJSON), 100) + strings.Repeat(","+string(recordingRuleJSON), 100) +
		`]}`)

	b.Run("streaming", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := json.NewDecoder(bytes.NewReader(data)).Decode(&RuleGroup{}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if err := json.Unmarshal(data, &RuleGroup{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkQueryResult(b *testing.B) {
	scalarData, err := json.Marshal(queryResult{
		Type:   model.ValScalar,
		Result: &model.Scalar{Value: 2, Timestamp: model.TimeFromUnix(1234)},
	})
	if err != nil {
		b.Fatal(err)
	}

	vectorData, err := json.Marshal(queryResult{
		Type:   model.ValVector,
		Result: model.Vector{genSample(), genSample(), genSample(), genSample(), genSample(), genSample(), genSample(), genSample(), genSample(), genSample()},
	})
	if err != nil {
		b.Fatal(err)
	}

	floatMatrix, histogramMatrix := generateData(10, 10)
	floatData, err := json.Marshal(queryResult{
		Type:   model.ValMatrix,
		Result: floatMatrix,
	})
	if err != nil {
		b.Fatal(err)
	}

	histogramData, err := json.Marshal(queryResult{
		Type:   model.ValMatrix,
		Result: histogramMatrix,
	})
	if err != nil {
		b.Fatal(err)
	}

	b.Run("scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var q queryResult
			if err := q.UnmarshalJSON(scalarData); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("vector", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var q queryResult
			if err := q.UnmarshalJSON(vectorData); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("matrix-float", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var q queryResult
			if err := q.UnmarshalJSON(floatData); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("matrix-histogram", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var q queryResult
			if err := q.UnmarshalJSON(histogramData); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func genSample() *model.Sample {
	return &model.Sample{
		Metric: model.Metric{
			"name": "test_metric",
		},
		Histogram: genSampleHistogram(),
		Timestamp: 1234567,
	}
}

func genSampleHistogram() *model.SampleHistogram {
	return &model.SampleHistogram{
		Count: 6,
		Sum:   3897,
		Buckets: model.HistogramBuckets{
			{
				Boundaries: 1,
				Lower:      -4870.992343051145,
				Upper:      -4466.7196729968955,
				Count:      1,
			},
			{
				Boundaries: 1,
				Lower:      -861.0779292198035,
				Upper:      -789.6119426088657,
				Count:      1,
			},
			{
				Boundaries: 1,
				Lower:      -558.3399591246119,
				Upper:      -512,
				Count:      1,
			},
			{
				Boundaries: 0,
				Lower:      2048,
				Upper:      2233.3598364984477,
				Count:      1,
			},
			{
				Boundaries: 0,
				Lower:      2896.3093757400984,
				Upper:      3158.4477704354626,
				Count:      1,
			},
			{
				Boundaries: 0,
				Lower:      4466.7196729968955,
				Upper:      4870.992343051145,
				Count:      1,
			},
		},
	}
}
