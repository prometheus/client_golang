// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	. "github.com/matttproud/gocheck"
	"testing"
)

type S struct{}

var _ = Suite(&S{})

func TestPrometheus(t *testing.T) {
	TestingT(t)
}
