// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// metrics_test.go provides a test suite for all tests in the metrics package
// hierarchy.  It employs the gocheck framework for test scaffolding.

package metrics

import (
	. "launchpad.net/gocheck"
	"testing"
)

type S struct{}

var _ = Suite(&S{})

func TestMetrics(t *testing.T) {
	TestingT(t)
}
