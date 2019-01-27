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

// +build go1.7

package v1

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/go-kit/kit/log"
	goclient "github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/util/testutil"
	apiv1 "github.com/prometheus/prometheus/web/api/v1"
	"github.com/prometheus/tsdb"
)

var (
	mux    *http.ServeMux
	server *httptest.Server
)

func setup() func() {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)
	return func() {
		server.Close()
	}
}

type apiTest_ struct {
	do     func() (interface{}, error)
	res    interface{}
	err    error
	apierr *Error
}

func TestAPIs_(t *testing.T) {
	tearDown := setup()
	defer tearDown()

	// setup golang api client
	client, err := goclient.NewClient(goclient.Config{Address: server.URL})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	promAPI := NewAPI(client)
	testTime := time.Unix(0, 0)

	// prepare arguments for creating the new v1api
	var samplePrometheusCfg = config.Config{}
	var sampleFlagMap = map[string]string{
		"alertmanager.notification-queue-capacity": "10000",
		"alertmanager.timeout":                     "10s",
		"log.level":                                "info",
		"query.lookback-delta":                     "5m",
		"query.max-concurrency":                    "20",
	}
	suite, err := promql.NewTest(t, `
	load 10s
		up{job="prometheus", instance="localhost:9090"} 0+100x100
		test_metric1{foo="val1",} 0+100x100
		test_metric1{foo="val2",} 1+0x100
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer suite.Close()
	if err := suite.Run(); err != nil {
		t.Fatal(err)
	}
	rr := testRulesRetriever{
		testing: t,
	}
	db := func() apiv1.TSDBAdmin {
		return &tsdb.DB{}
	}
	logger := log.NewNopLogger()

	// create v1 api
	api := apiv1.NewAPI(
		suite.QueryEngine(),
		suite.Storage(),
		testTargetRetriever{},
		testAlertmanagerRetriever{},
		func() config.Config { return samplePrometheusCfg },
		sampleFlagMap,
		func(f http.HandlerFunc) http.HandlerFunc { return f },
		db,
		true,
		logger,
		rr,
		0,
		0,
		regexp.MustCompile(".*"),
	)

	// register the router with the api
	router := route.New()
	api.Register(router)
	mux.Handle("/", http.StripPrefix(apiPrefix, router))

	// do methods for API interfaces
	doAlertManagers := func() func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.AlertManagers(context.Background())
		}
	}

	doQuery := func(q string, ts time.Time) func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.Query(context.Background(), q, ts)
		}
	}

	doQueryRange := func(q string, rng Range) func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.QueryRange(context.Background(), q, rng)
		}
	}

	doLabelValues := func(label string) func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.LabelValues(context.Background(), label)
		}
	}

	doSeries := func(matcher []string, startTime time.Time, endTime time.Time) func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.Series(context.Background(), matcher, startTime, endTime)
		}
	}

	doRules := func() func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.Rules(context.Background())
		}
	}

	doTargets := func() func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.Targets(context.Background())
		}
	}

	doConfig := func() func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.Config(context.Background())
		}
	}

	doFlags := func() func() (interface{}, error) {
		return func() (interface{}, error) {
			return promAPI.Flags(context.Background())
		}
	}

	//doCleanTombstones := func() func() (interface{}, error) {
	//	return func() (interface{}, error) {
	//		return nil, promAPI.CleanTombstones(context.Background())
	//	}
	//}
	//doDeleteSeries := func(matcher string, startTime time.Time, endTime time.Time) func() (interface{}, error) {
	//	return func() (interface{}, error) {
	//		return nil, promAPI.DeleteSeries(context.Background(), []string{matcher}, startTime, endTime)
	//	}
	//}
	//doSnapshot := func(skipHead bool) func() (interface{}, error) {
	//	return func() (interface{}, error) {
	//		return promAPI.Snapshot(context.Background(), skipHead)
	//	}
	//}

	queryTests := []apiTest_{
		{
			do: doAlertManagers(),
			res: AlertManagersResult{
				Active: []AlertManager{
					{
						URL: "http://127.0.0.1:9091/api/v1/alerts",
					},
				},
				Dropped: []AlertManager{
					{
						URL: "http://127.0.0.1:9092/api/v1/alerts",
					},
				},
			},
		},
		{
			do: doConfig(),
			res: ConfigResult{
				YAML: "global: {}\n",
			},
		},
		{
			do: doFlags(),
			res: FlagsResult{
				"alertmanager.notification-queue-capacity": "10000",
				"alertmanager.timeout":                     "10s",
				"log.level":                                "info",
				"query.lookback-delta":                     "5m",
				"query.max-concurrency":                    "20",
			},
		},
		{
			do: doQuery("2", testTime),
			res: &model.Scalar{
				Value:     2,
				Timestamp: model.TimeFromUnixNano(testTime.UnixNano()),
			},
		},
		{
			do: doRules(),
			res: RulesResult{
				Groups: []RuleGroup{
					{
						Name:     "example",
						File:     "/rules.yml",
						Interval: 1,
						Rules: []interface{}{
							AlertingRule{
								Alerts: []*Alert{},
								Annotations: model.LabelSet{
									"summary": "High request latency",
								},
								Labels: model.LabelSet{
									"severity": "page",
								},
								Duration:  600,
								Health:    RuleHealthUnknown,
								Name:      "HighRequestLatency",
								Query:     "job:request_latency_seconds:mean5m{job=\"myjob\"} > 0.5",
								LastError: "",
							},
							RecordingRule{
								Health:    RuleHealthUnknown,
								Name:      "job:http_inprogress_requests:sum",
								Query:     "sum by(job) (http_inprogress_requests)",
								LastError: "",
							},
						},
					},
				},
			},
		},
		{
			do: doTargets(),
			res: TargetsResult{
				Active: []ActiveTarget{
					{
						DiscoveredLabels: map[string]string{},
						Labels: model.LabelSet{
							"job": "prometheus",
						},
						ScrapeURL: "http://127.0.0.1:9090/metrics",
						Health:    HealthUnknown,
					},
				},
				Dropped: []DroppedTarget{
					{
						DiscoveredLabels: map[string]string{
							"__address__":      "127.0.0.1:9100",
							"__metrics_path__": "/metrics",
							"__scheme__":       "http",
							"job":              "node",
						},
					},
				},
			},
		},
		{
			do: doQueryRange("2", Range{
				Start: testTime.Add(-time.Minute),
				End:   testTime,
				Step:  time.Minute,
			}),
			res: model.Matrix{
				{
					Metric: model.Metric{},
					Values: []model.SamplePair{
						{
							Value:     2,
							Timestamp: model.TimeFromUnixNano(testTime.Add(-time.Minute).UnixNano()),
						},
						{
							Value:     2,
							Timestamp: model.TimeFromUnixNano(testTime.UnixNano()),
						},
					},
				},
			},
		},
		{
			do:  doLabelValues("foo"),
			res: model.LabelValues{"val1", "val2"},
		},
		{
			do: doSeries([]string{"up"}, testTime.Add(-time.Minute), testTime.Add(time.Minute)),
			res: []model.LabelSet{
				model.LabelSet{
					"__name__": "up",
					"job":      "prometheus",
					"instance": "localhost:9090",
				},
			},
		},
		// Error Tests
		{
			do: doLabelValues("invalid-label-name"),
			apierr: &Error{
				Type: ErrBadData,
				Msg:  "invalid label name: \"invalid-label-name\"",
			},
		},
		{
			do: doQuery("", testTime),
			apierr: &Error{
				Type: ErrBadData,
				Msg:  "parse error at char 1: no expression found in input",
			},
		},
	}

	for i, test := range queryTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {

			res, err := test.do()

			if test.err != nil || test.apierr != nil {
				if err == nil {
					t.Fatalf("expected error %q but got none", test.err)
				}
				if apiErr, ok := err.(*Error); ok {
					if !reflect.DeepEqual(apiErr, test.apierr) {
						t.Errorf("API Error:\nrecieved %q\nexpected %q", apiErr, test.apierr.Error())
					}
				} else {
					if err.Error() != test.err.Error() {
						t.Errorf("Other Error:\nrecieved %q\nexpected %q", err, test.err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if !reflect.DeepEqual(res, test.res) {
				t.Errorf("unexpected result:\nexpected %v\nrecieved %v", test.res, res)
			}
		})
	}
}

type testTargetRetriever struct{}
type testAlertmanagerRetriever struct{}
type testRulesRetriever struct {
	testing *testing.T
}

func (t testTargetRetriever) TargetsActive() map[string][]*scrape.Target {
	return map[string][]*scrape.Target{
		"prometheus": {
			scrape.NewTarget(
				labels.FromMap(map[string]string{
					model.SchemeLabel:      "http",
					model.AddressLabel:     "127.0.0.1:9090",
					model.MetricsPathLabel: "/metrics",
					model.JobLabel:         "prometheus",
				}),
				nil,
				url.Values{},
			),
		},
	}
}

func (t testTargetRetriever) TargetsDropped() map[string][]*scrape.Target {
	return map[string][]*scrape.Target{
		"node": {
			scrape.NewTarget(
				nil,
				labels.FromMap(map[string]string{
					model.SchemeLabel:      "http",
					model.AddressLabel:     "127.0.0.1:9100",
					model.MetricsPathLabel: "/metrics",
					model.JobLabel:         "node",
				}),
				url.Values{},
			),
		},
	}
}

func (m testRulesRetriever) AlertingRules() []*rules.AlertingRule {
	expr1, err := promql.ParseExpr(`job:request_latency_seconds:mean5m{job="myjob"} > 0.5`)
	if err != nil {
		m.testing.Fatalf("unable to parse alert expression: %s", err)
	}
	rule1 := rules.NewAlertingRule(
		"HighRequestLatency",
		expr1,
		time.Second*600,
		labels.FromMap(map[string]string{
			"severity": "page",
		}),
		labels.FromMap(map[string]string{
			"summary": "High request latency",
		}),
		true,
		log.NewNopLogger(),
	)
	var r []*rules.AlertingRule
	r = append(r, rule1)
	return r
}

func (m testRulesRetriever) RuleGroups() []*rules.Group {
	var ar testRulesRetriever
	arules := ar.AlertingRules()
	storage := testutil.NewStorage(m.testing)
	defer storage.Close()

	engineOpts := promql.EngineOpts{
		Logger:        nil,
		Reg:           nil,
		MaxConcurrent: 10,
		MaxSamples:    10,
		Timeout:       100 * time.Second,
	}

	engine := promql.NewEngine(engineOpts)
	opts := &rules.ManagerOptions{
		QueryFunc:  rules.EngineQueryFunc(engine, storage),
		Appendable: storage,
		Context:    context.Background(),
		Logger:     log.NewNopLogger(),
	}

	var r []rules.Rule

	for _, alertrule := range arules {
		r = append(r, alertrule)
	}

	recordingExpr, err := promql.ParseExpr(`sum(http_inprogress_requests) by (job)`)
	if err != nil {
		m.testing.Fatalf("unable to parse alert expression: %s", err)
	}
	recordingRule := rules.NewRecordingRule("job:http_inprogress_requests:sum", recordingExpr, labels.Labels{})
	r = append(r, recordingRule)

	group := rules.NewGroup("example", "/rules.yml", time.Second, r, false, opts)
	return []*rules.Group{group}
}

func (t testAlertmanagerRetriever) Alertmanagers() []*url.URL {
	return []*url.URL{
		{
			Scheme: "http",
			Host:   "127.0.0.1:9091",
			Path:   "/api/v1/alerts",
		},
	}
}

func (t testAlertmanagerRetriever) DroppedAlertmanagers() []*url.URL {
	return []*url.URL{
		{
			Scheme: "http",
			Host:   "127.0.0.1:9092",
			Path:   "/api/v1/alerts",
		},
	}
}
