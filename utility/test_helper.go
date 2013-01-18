/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package utility

import (
	. "github.com/matttproud/gocheck"
)

type valueEqualsChecker struct {
	*CheckerInfo
}

var ValueEquals Checker = &valueEqualsChecker{
	&CheckerInfo{Name: "IsValue", Params: []string{"obtained", "expected"}},
}

func (checker *valueEqualsChecker) Check(params []interface{}, names []string) (result bool, error string) {
	actual := params[0].(*Item).Value
	expected := params[1]

	return actual == expected, ""
}
