// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	. "github.com/matttproud/gocheck"
	"math"
	"reflect"
)

type isNaNChecker struct {
	*CheckerInfo
}

// This piece provides a simple tester for the gocheck testing library to
// ascertain if a value is not-a-number.
var IsNaN Checker = &isNaNChecker{
	&CheckerInfo{Name: "IsNaN", Params: []string{"value"}},
}

func (checker *isNaNChecker) Check(params []interface{}, names []string) (result bool, error string) {
	return isNaN(params[0]), ""
}

func isNaN(obtained interface{}) (result bool) {
	if obtained == nil {
		result = false
	} else {
		switch v := reflect.ValueOf(obtained); v.Kind() {
		case reflect.Float64:
			return math.IsNaN(obtained.(float64))
		}
	}

	return false
}

type valueEqualsChecker struct {
	*CheckerInfo
}

var ValueEquals Checker = &valueEqualsChecker{
	&CheckerInfo{Name: "IsValue", Params: []string{"obtained", "expected"}},
}

func (checker *valueEqualsChecker) Check(params []interface{}, names []string) (result bool, error string) {
	actual := params[0].(*item).Value
	expected := params[1]

	return actual == expected, ""
}

func dupMap(old map[string]string) map[string]string {
	new := make(map[string]string, len(old))

	for k, v := range old {
		new[k] = v
	}

	return new
}

// testSample represents a captured sample
type testSample struct {
	name   string
	value  float64
	labels map[string]string
}

func captureSamples(out *[]testSample) sampleFn {
	return func(name string, value float64, labels map[string]string) error {
		*out = append(*out, testSample{name, value, dupMap(labels)})
		return nil
	}
}
