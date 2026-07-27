// Copyright 2026 The Prometheus Authors
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

package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/prometheus/common/model"
)

// apiClientDoer is the interface the generated client needs for transport.
// The standard api.Client from client_golang satisfies this interface.
type apiClientDoer interface {
	Do(context.Context, *http.Request) (*http.Response, []byte, error)
}

// Client is a higher-level OpenAPI-generated client for the Prometheus HTTP API.
// It wraps the raw generated client with model.Value decoding and convenience methods.
type APIClient struct {
	gen *ClientWithResponses
}

// NewAPIClient creates a new APIClient that uses the provided apiClient for transport.
// The apiClient must implement Do(ctx, *http.Request) (*http.Response, []byte, error).
func NewAPIClient(apiClient apiClientDoer) *APIClient {
	genTransport := &transport{client: apiClient}
	genClient, _ := NewClientWithResponses("http://localhost", WithHTTPClient(genTransport))
	return &APIClient{gen: genClient}
}

// transport adapts the client_golang api.Client to oapi-codegen's HttpRequestDoer.
type transport struct {
	client apiClientDoer
}

func (t *transport) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	resp, body, err := t.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// ── Query Types ────────────────────────────────────────────────────────

// QueryOptions configures optional parameters for query requests.
type QueryOptions struct {
	Timeout       string // e.g. "30s"
	LookbackDelta string // e.g. "5m"
	Stats         string // "all" to enable stats
	Limit         uint64
}

// QueryResult holds the result of an instant or range query.
type QueryResult struct {
	Value    model.Value
	Warnings []string
	Infos    []string
}

// QueryError is an error returned by the Prometheus API.
type QueryError struct {
	Type string
	Msg  string
}

func (e *QueryError) Error() string { return fmt.Sprintf("%s: %s", e.Type, e.Msg) }

// ── Range Type ────────────────────────────────────────────────────────

// QueryRange represents a sliced time range for range queries.
type QueryRange struct {
	Start, End model.Time
	Step       string // e.g. "15s"
}

// ── Query Methods ──────────────────────────────────────────────────────

// InstantQuery performs an instant query using the OpenAPI generated client.
func (c *APIClient) InstantQuery(ctx context.Context, query string, ts model.Time, opts QueryOptions) (*QueryResult, error) {
	params := &GetInstantQueryParams{
		Query: query,
	}
	if ts != 0 {
		t := float32(float64(ts) / 1000)
		params.Time = &t
	}
	applyQueryOpts(params, nil, opts)

	resp, err := c.gen.GetInstantQueryWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	qr := resp.JSON200
	if qr.Status == "error" {
		return nil, queryErr(qr)
	}

	return parseQueryResult(qr.Data, qr.Warnings, qr.Infos)
}

// RangeQuery performs a range query using the OpenAPI generated client.
func (c *APIClient) RangeQuery(ctx context.Context, query string, r QueryRange, opts QueryOptions) (*QueryResult, error) {
	params := &GetRangeQueryParams{
		Query: query,
		Start: float32(float64(r.Start) / 1000),
		End:   float32(float64(r.End) / 1000),
		Step:  r.Step,
	}
	applyQueryOpts(nil, params, opts)

	resp, err := c.gen.GetRangeQueryWithResponse(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("query_range: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, parseErrorResponse(resp.HTTPResponse, resp.Body)
	}

	qr := resp.JSON200
	if qr.Status == "error" {
		return nil, queryErr(qr)
	}

	return parseQueryResult(qr.Data, qr.Warnings, qr.Infos)
}

// LabelNames returns label names using the OpenAPI generated client.
func (c *APIClient) LabelNames(ctx context.Context, matches []string, startTime, endTime model.Time) (model.LabelNames, []string, []string, error) {
	params := &GetLabelNamesParams{}
	if len(matches) > 0 {
		params.Match = &matches
	}
	if startTime != 0 {
		t := float32(float64(startTime) / 1000)
		params.Start = &t
	}
	if endTime != 0 {
		t := float32(float64(endTime) / 1000)
		params.End = &t
	}

	resp, err := c.gen.GetLabelNamesWithResponse(ctx, params)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("labels: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, nil, nil, fmt.Errorf("unexpected status %d: %s", resp.HTTPResponse.StatusCode, string(resp.Body))
	}

	sr := resp.JSON200
	if sr.Status == "error" {
		return nil, nil, nil, fmt.Errorf("labels: server returned status=error")
	}

	var warnings, infos []string
	if sr.Warnings != nil {
		warnings = *sr.Warnings
	}
	if sr.Infos != nil {
		infos = *sr.Infos
	}

	return strSliceToLabelNames(sr.Data), warnings, infos, nil
}

// LabelValues returns label values for a given label name.
func (c *APIClient) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime model.Time) (model.LabelValues, []string, []string, error) {
	params := &GetLabelValuesParams{}
	if len(matches) > 0 {
		params.Match = &matches
	}
	if startTime != 0 {
		t := float32(float64(startTime) / 1000)
		params.Start = &t
	}
	if endTime != 0 {
		t := float32(float64(endTime) / 1000)
		params.End = &t
	}

	resp, err := c.gen.GetLabelValuesWithResponse(ctx, label, params)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("label values: %w", err)
	}
	if resp.JSON200 == nil {
		return nil, nil, nil, fmt.Errorf("unexpected status %d: %s", resp.HTTPResponse.StatusCode, string(resp.Body))
	}

	sr := resp.JSON200
	if sr.Status == "error" {
		return nil, nil, nil, fmt.Errorf("label values: server returned status=error")
	}

	var warnings, infos []string
	if sr.Warnings != nil {
		warnings = *sr.Warnings
	}
	if sr.Infos != nil {
		infos = *sr.Infos
	}

	return strSliceToLabelValues(sr.Data), warnings, infos, nil
}

// ── Access to the generated client ─────────────────────────────────────

// Generated returns the raw generated oapi-codegen ClientWithResponses,
// for use with endpoints not yet wrapped by the high-level API.
func (c *APIClient) Generated() *ClientWithResponses {
	return c.gen
}

// ── Helpers ───────────────────────────────────────────────────────────

func applyQueryOpts(getParams *GetInstantQueryParams, rangeParams *GetRangeQueryParams, opts QueryOptions) {
	if getParams != nil {
		if opts.Timeout != "" {
			getParams.Timeout = &opts.Timeout
		}
		if opts.LookbackDelta != "" {
			getParams.LookbackDelta = &opts.LookbackDelta
		}
		if opts.Stats != "" {
			getParams.Stats = &opts.Stats
		}
	}
	if rangeParams != nil {
		if opts.Timeout != "" {
			rangeParams.Timeout = &opts.Timeout
		}
		if opts.LookbackDelta != "" {
			rangeParams.LookbackDelta = &opts.LookbackDelta
		}
		if opts.Stats != "" {
			rangeParams.Stats = &opts.Stats
		}
	}
}

func parseErrorResponse(resp *http.Response, body []byte) error {
	if resp == nil {
		return fmt.Errorf("empty response")
	}
	var errResp struct {
		Status    string `json:"status"`
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil && errResp.Status == "error" {
		return &QueryError{Type: errResp.ErrorType, Msg: errResp.Error}
	}
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}

func queryErr(qr *QueryResponse) *QueryError {
	errType := ""
	errMsg := ""
	if qr.ErrorType != nil {
		errType = *qr.ErrorType
	}
	if qr.Error != nil {
		errMsg = *qr.Error
	}
	return &QueryError{Type: errType, Msg: errMsg}
}

func parseQueryResult(data QueryData, warnings, infos *[]string) (*QueryResult, error) {
	val, err := dataToModelValue(data)
	if err != nil {
		return nil, err
	}

	var w, i []string
	if warnings != nil {
		w = *warnings
	}
	if infos != nil {
		i = *infos
	}

	return &QueryResult{Value: val, Warnings: w, Infos: i}, nil
}

// dataToModelValue converts the generated QueryData to a model.Value.
// Since the OpenAPI spec represents query results as generic objects,
// this round-trips through JSON to leverage model's existing unmarshalers.
func strSliceToLabelNames(ss []string) model.LabelNames {
	if ss == nil {
		return nil
	}
	ln := make(model.LabelNames, len(ss))
	for i, s := range ss {
		ln[i] = model.LabelName(s)
	}
	return ln
}

func strSliceToLabelValues(ss []string) model.LabelValues {
	if ss == nil {
		return nil
	}
	lv := make(model.LabelValues, len(ss))
	for i, s := range ss {
		lv[i] = model.LabelValue(s)
	}
	return lv
}

func dataToModelValue(data QueryData) (model.Value, error) {
	b, err := json.Marshal(data.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal query result: %w", err)
	}

	switch data.ResultType {
	case "scalar":
		var sv model.Scalar
		if err := json.Unmarshal(b, &sv); err != nil {
			return nil, fmt.Errorf("unmarshal scalar: %w", err)
		}
		return &sv, nil
	case "vector":
		var vv model.Vector
		if err := json.Unmarshal(b, &vv); err != nil {
			return nil, fmt.Errorf("unmarshal vector: %w", err)
		}
		return vv, nil
	case "matrix":
		var mv model.Matrix
		if err := json.Unmarshal(b, &mv); err != nil {
			return nil, fmt.Errorf("unmarshal matrix: %w", err)
		}
		return mv, nil
	default:
		return nil, fmt.Errorf("unknown resultType: %q", data.ResultType)
	}
}
