/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	. "github.com/matttproud/gocheck"
	"testing"
)

type S struct{}

var _ = Suite(&S{})

func TestMetrics(t *testing.T) {
	TestingT(t)
}
