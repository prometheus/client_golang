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
	//"errors"
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
	"github.com/prometheus/prometheus/storage"
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
	do  func() (interface{}, error)
	res interface{}
	err error
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
	testTime := time.Now()

	// prepare arguments for creating the new v1api
	var samplePrometheusCfg = config.Config{}
	var sampleFlagMap = map[string]string{}
	suite, err := promql.NewTest(t, "")
	if err != nil {
		t.Fatal(err)
	}
	defer suite.Close()
	if err := suite.Run(); err != nil {
		t.Fatal(err)
	}
	var algr rulesRetrieverMock
	algr.testing = t
	algr.AlertingRules()
	algr.RuleGroups()
	db := func() apiv1.TSDBAdmin {
		return &tsdb.DB{}
	}
	logger := log.NewNopLogger()
	str := testStorage{
		q: testQuerier{},
	}

	// create api
	api := apiv1.NewAPI(
		suite.QueryEngine(),
		str,
		testTargetRetriever{},
		testAlertmanagerRetriever{},
		func() config.Config { return samplePrometheusCfg },
		sampleFlagMap,
		func(f http.HandlerFunc) http.HandlerFunc { return f },
		db,
		true,
		logger,
		algr,
		0,
		0,
		regexp.MustCompile(".*"),
	)

	// register the router with the api
	router := route.New()
	api.Register(router)
	mux.Handle("/", http.StripPrefix(apiPrefix, router))

	// do methods for test table
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
			do: doQuery("2", testTime),
			res: &model.Scalar{
				Value:     2,
				Timestamp: model.TimeFromUnixNano(testTime.UnixNano()),
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
			do:  doLabelValues("mylabel"),
			res: model.LabelValues{"val1", "val2"},
		},
	}

	for i, test := range queryTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {

			res, err := test.do()

			if test.err != nil {
				if err == nil {
					t.Fatalf("expected error %q but got none", test.err)
				}
				if err.Error() != test.err.Error() {
					t.Errorf("unexpected error: want %s, got %s", test.err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			// TODO add the err api check

			if !reflect.DeepEqual(res, test.res) {
				t.Errorf("unexpected result: want %v, got %v", test.res, res)
			}
		})
	}
}

type testStorage struct {
	q storage.Querier
}
type testQuerier struct{}
type testTargetRetriever struct{}
type testAlertmanagerRetriever struct{}
type rulesRetrieverMock struct {
	testing *testing.T
}

func (t testStorage) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	return t.q, nil
}

func (t testQuerier) LabelValues(name string) ([]string, error) {
	return []string{"val1", "val2"}, nil
}

func (t testQuerier) Close() error {
	return nil
}

func (t testQuerier) LabelNames() ([]string, error) {
	return []string{"val1", "val2"}, nil
}

func (t testQuerier) Select(params *storage.SelectParams, matchers ...*labels.Matcher) (storage.SeriesSet, storage.Warnings, error) {
	var seriesSet storage.SeriesSet
	var warnings storage.Warnings
	return seriesSet, warnings, nil
}

func (t testTargetRetriever) TargetsActive() map[string][]*scrape.Target {
	return map[string][]*scrape.Target{
		"test": {
			scrape.NewTarget(
				labels.FromMap(map[string]string{
					model.SchemeLabel:      "http",
					model.AddressLabel:     "example.com:8080",
					model.MetricsPathLabel: "/metrics",
					model.JobLabel:         "test",
				}),
				nil,
				url.Values{},
			),
		},
		"blackbox": {
			scrape.NewTarget(
				labels.FromMap(map[string]string{
					model.SchemeLabel:      "http",
					model.AddressLabel:     "localhost:9115",
					model.MetricsPathLabel: "/probe",
					model.JobLabel:         "blackbox",
				}),
				nil,
				url.Values{"target": []string{"example.com"}},
			),
		},
	}
}

func (t testTargetRetriever) TargetsDropped() map[string][]*scrape.Target {
	return map[string][]*scrape.Target{
		"blackbox": {
			scrape.NewTarget(
				nil,
				labels.FromMap(map[string]string{
					model.AddressLabel:     "http://dropped.example.com:9115",
					model.MetricsPathLabel: "/probe",
					model.SchemeLabel:      "http",
					model.JobLabel:         "blackbox",
				}),
				url.Values{},
			),
		},
	}
}

func (m rulesRetrieverMock) AlertingRules() []*rules.AlertingRule {
	expr1, err := promql.ParseExpr(`absent(test_metric3) != 1`)
	if err != nil {
		m.testing.Fatalf("unable to parse alert expression: %s", err)
	}
	expr2, err := promql.ParseExpr(`up == 1`)
	if err != nil {
		m.testing.Fatalf("Unable to parse alert expression: %s", err)
	}

	rule1 := rules.NewAlertingRule(
		"test_metric3",
		expr1,
		time.Second,
		labels.Labels{},
		labels.Labels{},
		true,
		log.NewNopLogger(),
	)
	rule2 := rules.NewAlertingRule(
		"test_metric4",
		expr2,
		time.Second,
		labels.Labels{},
		labels.Labels{},
		true,
		log.NewNopLogger(),
	)
	var r []*rules.AlertingRule
	r = append(r, rule1)
	r = append(r, rule2)
	return r
}

func (m rulesRetrieverMock) RuleGroups() []*rules.Group {
	var ar rulesRetrieverMock
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

	recordingExpr, err := promql.ParseExpr(`vector(1)`)
	if err != nil {
		m.testing.Fatalf("unable to parse alert expression: %s", err)
	}
	recordingRule := rules.NewRecordingRule("recording-rule-1", recordingExpr, labels.Labels{})
	r = append(r, recordingRule)

	group := rules.NewGroup("grp", "/path/to/file", time.Second, r, false, opts)
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
