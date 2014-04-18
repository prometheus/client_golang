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

func ExampleGauge() {
	delOps := NewGauge(GaugeDesc{
		Desc{
			Namespace: "our_company",
			Subsystem: "blob_storage",
			Name:      "deletes",

			Help: "How many delete operations we have conducted against our blob storage system.",
		},
	})

	delOps.Set(900) // That's all, folks!
}

func ExampleGaugeVec() {
	delOps := NewGaugeVec(GaugeVecDesc{
		Desc{
			Namespace: "our_company",
			Subsystem: "blob_storage",
			Name:      "deletes",

			Help: "How many delete operations we have conducted against our blob storage system, partitioned by data corpus and qos.",
		},

		Labels: []string{
			// What is the body of data being deleted?
			"corpus",
			// How urgently do we need to delete the data?
			"qos",
		},
	})

	// Oops, we need to delete that embarrassing picture of ourselves.
	delOps.Set(4, "profile-pictures", "immediate")
	// Those bad cat memes finally get deleted.
	delOps.Set(1, "cat-memes", "lazy")
}
