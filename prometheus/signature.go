// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"fmt"
	"hash/fnv"
	"sort"

	"github.com/prometheus/client_golang/model"
)

// LabelsToSignature provides a way of building a unique signature
// (i.e., fingerprint) for a given label set sequence.
func labelsToSignature(labels map[string]string) uint64 {
	names := make(model.LabelNames, 0, len(labels))
	for name := range labels {
		names = append(names, model.LabelName(name))
	}

	sort.Sort(names)

	hasher := fnv.New64a()

	for _, name := range names {
		fmt.Fprintf(hasher, string(name), labels[string(name)])
	}

	return hasher.Sum64()
}
