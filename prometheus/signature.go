// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"bytes"
	"sort"
)

const (
	delimiter = "|"
)

// LabelsToSignature provides a way of building a unique signature
// (i.e., fingerprint) for a given label set sequence.
func labelsToSignature(labels map[string]string) string {
	// TODO(matt): This is a wart, and we'll want to validate that collisions
	//             do not occur in less-than-diligent environments.
	cardinality := len(labels)
	keys := make([]string, 0, cardinality)

	for label := range labels {
		keys = append(keys, label)
	}

	sort.Strings(keys)

	buffer := bytes.Buffer{}

	for _, label := range keys {
		buffer.WriteString(label)
		buffer.WriteString(delimiter)
		buffer.WriteString(labels[label])
	}

	return buffer.String()
}
