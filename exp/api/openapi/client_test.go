package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

// mockDoer implements apiClientDoer with a fixed response.
type mockDoer struct {
	status int
	body   []byte
}

func (m *mockDoer) Do(_ context.Context, _ *http.Request) (*http.Response, []byte, error) {
	return &http.Response{
		StatusCode: m.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, m.body, nil
}

func TestInstantQueryScalar(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":{"resultType":"scalar","result":[1700000000,"42.5"]}}`),
	}
	client := NewAPIClient(mock)
	result, err := client.InstantQuery(context.Background(), "up", 0, QueryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Value == nil {
		t.Fatal("expected non-nil value")
	}
	sv, ok := result.Value.(*model.Scalar)
	if !ok {
		t.Fatalf("expected *model.Scalar, got %T", result.Value)
	}
	if sv.Value != 42.5 {
		t.Errorf("expected value 42.5, got %f", float64(sv.Value))
	}
}

func TestInstantQueryVector(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up","job":"prometheus"},"value":[1700000000,"1"]}]}}`),
	}
	client := NewAPIClient(mock)
	result, err := client.InstantQuery(context.Background(), "up", 0, QueryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vv, ok := result.Value.(model.Vector)
	if !ok {
		t.Fatalf("expected model.Vector, got %T", result.Value)
	}
	if len(vv) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(vv))
	}
	if vv[0].Value != 1.0 {
		t.Errorf("expected value 1.0, got %f", float64(vv[0].Value))
	}
}

func TestInstantQueryMatrix(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"up"},"values":[[1700000000,"1"],[1700000015,"1"]]}]}}`),
	}
	client := NewAPIClient(mock)
	result, err := client.InstantQuery(context.Background(), "up", 0, QueryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mv, ok := result.Value.(model.Matrix)
	if !ok {
		t.Fatalf("expected model.Matrix, got %T", result.Value)
	}
	if len(mv) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(mv))
	}
	if len(mv[0].Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(mv[0].Values))
	}
}

func TestQueryError(t *testing.T) {
	mock := &mockDoer{
		status: 422,
		body:   []byte(`{"status":"error","errorType":"bad_data","error":"invalid parameter"}`),
	}
	client := NewAPIClient(mock)
	_, err := client.InstantQuery(context.Background(), "invalid", 0, QueryOptions{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	qe, ok := err.(*QueryError)
	if !ok {
		t.Fatalf("expected *QueryError, got %T", err)
	}
	if qe.Type != "bad_data" {
		t.Errorf("expected bad_data, got %s", qe.Type)
	}
}

func TestLabelNames(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":["__name__","job","instance"]}`),
	}
	client := NewAPIClient(mock)
	names, warnings, infos, err := client.LabelNames(context.Background(), nil, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warnings != nil {
		t.Errorf("expected nil warnings, got %v", warnings)
	}
	if infos != nil {
		t.Errorf("expected nil infos, got %v", infos)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 label names, got %d", len(names))
	}
}

func TestLabelValues(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":["prometheus","alertmanager","grafana"]}`),
	}
	client := NewAPIClient(mock)
	values, warnings, infos, err := client.LabelValues(context.Background(), "job", nil, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warnings != nil {
		t.Errorf("expected nil warnings, got %v", warnings)
	}
	if infos != nil {
		t.Errorf("expected nil infos, got %v", infos)
	}
	if len(values) != 3 {
		t.Fatalf("expected 3 label values, got %d", len(values))
	}
}

func TestWarningsAndInfos(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":["__name__"],"warnings":["deprecated"],"infos":["note"]}`),
	}
	client := NewAPIClient(mock)
	_, warnings, infos, err := client.LabelNames(context.Background(), nil, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 || warnings[0] != "deprecated" {
		t.Errorf("expected warning 'deprecated', got %v", warnings)
	}
	if len(infos) != 1 || infos[0] != "note" {
		t.Errorf("expected info 'note', got %v", infos)
	}
}

func TestQueryWithWarnings(t *testing.T) {
	mock := &mockDoer{
		status: 200,
		body:   []byte(`{"status":"success","data":{"resultType":"scalar","result":[1700000000,"42.5"]},"warnings":["slow query"],"infos":["cached"]}`),
	}
	client := NewAPIClient(mock)
	result, err := client.InstantQuery(context.Background(), "1+1", 0, QueryOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "slow query" {
		t.Errorf("expected warning 'slow query', got %v", result.Warnings)
	}
	if len(result.Infos) != 1 || result.Infos[0] != "cached" {
		t.Errorf("expected info 'cached', got %v", result.Infos)
	}
	sv, ok := result.Value.(*model.Scalar)
	if !ok || sv.Value != 42.5 {
		t.Fatalf("scalar value mismatch: %v", result.Value)
	}
}

// BenchmarkQueryDecodeVector benchmarks the cost of the generated path:
// json.Unmarshal into generated types -> json.Marshal -> json.Unmarshal into model.Vector.
// 100 series.
func BenchmarkQueryDecodeVector(b *testing.B) {
	streams := make([]struct {
		Metric map[string]string `json:"metric"`
		Value  []interface{}     `json:"value"`
	}, 100)
	for i := 0; i < 100; i++ {
		streams[i] = struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		}{
			Metric: map[string]string{"__name__": "series_" + itoa(i)},
			Value:  []interface{}{float64(1700000000 + i*15), "1.0"},
		}
	}
	dataJSON, _ := json.Marshal(QueryData{
		ResultType: "vector",
		Result:     map[string]interface{}{"result": streams},
	})
	var container map[string]interface{}
	json.Unmarshal(dataJSON, &container)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = dataToModelValue(QueryData{
			ResultType: "vector",
			Result:     container,
		})
	}
}

// BenchmarkQueryDecodeMatrix benchmarks 10 series x 1000 datapoints through
// the round-trip decode path.
func BenchmarkQueryDecodeMatrix(b *testing.B) {
	streams := make([]struct {
		Metric map[string]string `json:"metric"`
		Values [][]interface{}   `json:"values"`
	}, 10)
	now := time.Now()
	for i := 0; i < 10; i++ {
		vals := make([][]interface{}, 1000)
		for j := 0; j < 1000; j++ {
			vals[j] = []interface{}{float64(now.Unix() + int64(j*15)), "1.0"}
		}
		streams[i] = struct {
			Metric map[string]string `json:"metric"`
			Values [][]interface{}   `json:"values"`
		}{
			Metric: map[string]string{"__name__": "series_" + itoa(i)},
			Values: vals,
		}
	}
	dataJSON, _ := json.Marshal(QueryData{
		ResultType: "matrix",
		Result:     map[string]interface{}{"result": streams},
	})
	var container map[string]interface{}
	json.Unmarshal(dataJSON, &container)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = dataToModelValue(QueryData{
			ResultType: "matrix",
			Result:     container,
		})
	}
}

// BenchmarkQueryDecodeHistogram benchmarks histogram sample decoding through
// the round-trip path. 10 series x 100 histogram datapoints.
func BenchmarkQueryDecodeHistogram(b *testing.B) {
	streams := make([]struct {
		Metric     map[string]string `json:"metric"`
		Histograms [][]interface{}   `json:"histograms"`
	}, 10)
	now := time.Now()
	for i := 0; i < 10; i++ {
		pairs := make([][]interface{}, 100)
		for j := 0; j < 100; j++ {
			pairs[j] = []interface{}{
				float64(now.Unix() + int64(j*15)),
				map[string]interface{}{
					"count": "13.5",
					"sum":   "0.1",
					"buckets": []interface{}{
						[]interface{}{float64(1), "-4870.99", "-4466.72", "1"},
					},
				},
			}
		}
		streams[i] = struct {
			Metric     map[string]string `json:"metric"`
			Histograms [][]interface{}   `json:"histograms"`
		}{
			Metric:     map[string]string{"__name__": "series_" + itoa(i)},
			Histograms: pairs,
		}
	}
	dataJSON, _ := json.Marshal(QueryData{
		ResultType: "matrix",
		Result:     map[string]interface{}{"result": streams},
	})
	var container map[string]interface{}
	json.Unmarshal(dataJSON, &container)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = dataToModelValue(QueryData{
			ResultType: "matrix",
			Result:     container,
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
