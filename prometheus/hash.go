// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"hash/fnv"
)

func hashLabelValues(vs ...string) uint64 {
	h := fnv.New64a()
	for _, v := range vs {
		h.Write([]byte(v))
	}
	return h.Sum64()
}
