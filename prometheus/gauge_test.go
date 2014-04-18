// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"testing"
	"testing/quick"
)

func ExampleNewGauge() {
	var delOps = NewGauge(GaugeDesc{
		Desc{
			Namespace: "our_company",
			Subsystem: "blob_storage",
			Name:      "deletes",
		},
	})

	MustRegisterOnce(delOps) // Execute this in func init() {}
}
