/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package maths

import (
	. "github.com/matttproud/gocheck"
	"math"
	"reflect"
)

type isNaNChecker struct {
	*CheckerInfo
}

/*
This piece provides a simple tester for the gocheck testing library to
ascertain if a value is not-a-number.
*/
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
