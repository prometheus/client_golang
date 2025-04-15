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
	"context"
	"time"

	"github.com/prometheus/common/model"
)

type FakeAPI struct {
	// FakeAPI is a mock API for testing purposes.
	// It implements the API interface and provides fake data for testing.
	ExpectedAlertsResult []*Alert
	ExpectedAlertsError  error

	ExpectedAlertManagersResult AlertManagersResult
	ExpectedAlertManagersError  error

	ExpectedCleanTombstonesError error

	ExpectedConfigResult ConfigResult
	ExpectedConfigError  error

	ExpectedDeleteSeriesError error

	ExpectedFlagsResult FlagsResult
	ExpectedFlagsError  error

	ExpectedLabelNamesResult   []string
	ExpectedLabelNamesWarnings Warnings
	ExpectedLabelNamesError    error

	ExpectedLabelValuesResult   model.LabelValues
	ExpectedLabelValuesWarnings Warnings
	ExpectedLabelValuesError    error

	ExpectedQueryResult   model.Value
	ExpectedQueryWarnings Warnings
	ExpectedQueryError    error

	ExpectedQueryRangeResult   model.Value
	ExpectedQueryRangeWarnings Warnings
	ExpectedQueryRangeError    error

	ExpectedQueryExemplarsResult []ExemplarQueryResult
	ExpectedQueryExemplarsError  error

	ExpectedBuildinfoResult BuildinfoResult
	ExpectedBuildinfoError  error

	ExpectedRuntimeinfoResult RuntimeinfoResult
	ExpectedRuntimeinfoError  error

	ExpectedSeriesResult   []model.LabelSet
	ExpectedSeriesWarnings Warnings
	ExpectedSeriesError    error

	ExpectedSnapshotResult SnapshotResult
	ExpectedSnapshotError  error

	ExpectedRulesResult RulesResult
	ExpectedRulesError  error

	ExpectedTargetsResult TargetsResult
	ExpectedTargetsError  error

	ExpectedTargetsMetadataResult []MetricMetadata
	ExpectedTargetsMetadataError  error

	ExpectedMetadataResult map[string][]Metadata
	ExpectedMetadataError  error

	ExpectedTSDBResult TSDBResult
	ExpectedTSDBError  error

	ExpectedWalReplayResult WalReplayStatus
	ExpectedWalReplayError  error
}

func (f *FakeAPI) Alerts(ctx context.Context) ([]*Alert, error) {
	return f.ExpectedAlertsResult, f.ExpectedAlertsError
}

func (f *FakeAPI) AlertManagers(ctx context.Context) (AlertManagersResult, error) {
	return f.ExpectedAlertManagersResult, f.ExpectedAlertManagersError
}

func (f *FakeAPI) CleanTombstones(ctx context.Context) error {
	return f.ExpectedCleanTombstonesError
}

func (f *FakeAPI) Config(ctx context.Context) (ConfigResult, error) {
	return f.ExpectedConfigResult, f.ExpectedConfigError
}

func (f *FakeAPI) DeleteSeries(ctx context.Context, matches []string, startTime, endTime time.Time) error {
	return f.ExpectedDeleteSeriesError
}

func (f *FakeAPI) Flags(ctx context.Context) (FlagsResult, error) {
	return f.ExpectedFlagsResult, f.ExpectedFlagsError
}

func (f *FakeAPI) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...Option) ([]string, Warnings, error) {
	return f.ExpectedLabelNamesResult, f.ExpectedLabelNamesWarnings, f.ExpectedLabelNamesError
}

func (f *FakeAPI) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...Option) (model.LabelValues, Warnings, error) {
	return f.ExpectedLabelValuesResult, f.ExpectedLabelValuesWarnings, f.ExpectedLabelValuesError
}

func (f *FakeAPI) Query(ctx context.Context, query string, ts time.Time, opts ...Option) (model.Value, Warnings, error) {
	return f.ExpectedQueryResult, f.ExpectedQueryWarnings, f.ExpectedQueryError
}

func (f *FakeAPI) QueryRange(ctx context.Context, query string, r Range, opts ...Option) (model.Value, Warnings, error) {
	return f.ExpectedQueryRangeResult, f.ExpectedQueryRangeWarnings, f.ExpectedQueryRangeError
}

func (f *FakeAPI) QueryExemplars(ctx context.Context, query string, startTime, endTime time.Time) ([]ExemplarQueryResult, error) {
	return f.ExpectedQueryExemplarsResult, f.ExpectedQueryExemplarsError
}

func (f *FakeAPI) Buildinfo(ctx context.Context) (BuildinfoResult, error) {
	return f.ExpectedBuildinfoResult, f.ExpectedBuildinfoError
}

func (f *FakeAPI) Runtimeinfo(ctx context.Context) (RuntimeinfoResult, error) {
	return f.ExpectedRuntimeinfoResult, f.ExpectedRuntimeinfoError
}

func (f *FakeAPI) Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...Option) ([]model.LabelSet, Warnings, error) {
	return f.ExpectedSeriesResult, f.ExpectedSeriesWarnings, f.ExpectedSeriesError
}

func (f *FakeAPI) Snapshot(ctx context.Context, skipHead bool) (SnapshotResult, error) {
	return f.ExpectedSnapshotResult, f.ExpectedSnapshotError
}

func (f *FakeAPI) Rules(ctx context.Context) (RulesResult, error) {
	return f.ExpectedRulesResult, f.ExpectedRulesError
}

func (f *FakeAPI) Targets(ctx context.Context) (TargetsResult, error) {
	return f.ExpectedTargetsResult, f.ExpectedTargetsError
}

func (f *FakeAPI) TargetsMetadata(ctx context.Context, matchTarget, metric, limit string) ([]MetricMetadata, error) {
	return f.ExpectedTargetsMetadataResult, f.ExpectedTargetsMetadataError
}

func (f *FakeAPI) Metadata(ctx context.Context, metric, limit string) (map[string][]Metadata, error) {
	return f.ExpectedMetadataResult, f.ExpectedMetadataError
}

func (f *FakeAPI) TSDB(ctx context.Context, opts ...Option) (TSDBResult, error) {
	return f.ExpectedTSDBResult, f.ExpectedTSDBError
}

func (f *FakeAPI) WalReplay(ctx context.Context) (WalReplayStatus, error) {
	return f.ExpectedWalReplayResult, f.ExpectedWalReplayError
}
