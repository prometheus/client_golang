package v1

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/common/model"
)

func assertEqual(t *testing.T, a, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%v != %v", a, b)
	}
}

func TestFakeAPI_Query(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		expectedResult   model.Value
		expectedWarnings Warnings
		expectedError    error
	}{
		{
			name:           "Valid query",
			query:          "up == 1",
			expectedResult: &model.String{Value: "1"},
		},
		{
			name:             "Query with no results, warning present",
			query:            "up == 0",
			expectedResult:   nil,
			expectedWarnings: Warnings{"Warning: No data found for query, check if the time range is correct"},
			expectedError:    nil,
		},
		{
			name:          "Error query",
			query:         "invalid_query",
			expectedError: fmt.Errorf("mock error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup FakeAPI
			fakeAPI := &FakeAPI{
				ExpectedQueryResult:   tt.expectedResult,
				ExpectedQueryWarnings: tt.expectedWarnings,
				ExpectedQueryError:    tt.expectedError,
			}

			result, warnings, err := fakeAPI.Query(context.Background(), tt.query, time.Now())
			assertEqual(t, tt.expectedResult, result)
			assertEqual(t, tt.expectedWarnings, warnings)
			assertEqual(t, tt.expectedError, err)
		})
	}
}

func TestFakeAPI_LabelNames(t *testing.T) {
	tests := []struct {
		name             string
		matches          []string
		expectedLabels   []string
		expectedWarnings Warnings
		expectedError    error
	}{
		{
			name:             "Valid label names",
			matches:          []string{"up"},
			expectedLabels:   []string{"label1", "label2"},
			expectedWarnings: nil,
			expectedError:    nil,
		},
		{
			name:             "Error in label names",
			matches:          []string{"error"},
			expectedLabels:   nil,
			expectedWarnings: nil,
			expectedError:    fmt.Errorf("mock label error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup FakeAPI
			fakeAPI := &FakeAPI{
				ExpectedLabelNamesResult:   tt.expectedLabels,
				ExpectedLabelNamesWarnings: tt.expectedWarnings,
				ExpectedLabelNamesError:    tt.expectedError,
			}

			result, warnings, err := fakeAPI.LabelNames(context.Background(), tt.matches, time.Now(), time.Now())
			assertEqual(t, tt.expectedLabels, result)
			assertEqual(t, tt.expectedWarnings, warnings)
			assertEqual(t, tt.expectedError, err)
		})
	}
}
