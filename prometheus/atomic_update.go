// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"math"
	"runtime"
	"sync/atomic"
)

// atomicAddFloat adds the provided float atomically to another float
// represented by the bit pattern the bits pointer is pointing to.
// It uses a minimal backoff strategy to handle extreme contention without
// regressing uncontended cases.
func atomicAddFloat(bits *uint64, v float64) {
	const (
		spinAttempts = 32 // Number of pure spin attempts before backoff
	)
	attempts := 0

	for {
		loadedBits := atomic.LoadUint64(bits)
		newBits := math.Float64bits(math.Float64frombits(loadedBits) + v)
		if atomic.CompareAndSwapUint64(bits, loadedBits, newBits) {
			return
		}

		attempts++
		if attempts < spinAttempts {
			// Pure spin for uncontended and lightly contended cases
			continue
		}

		// After many consecutive failures, use minimal backoff to
		// reduce CPU usage under extreme contention.
		runtime.Gosched()
	}
}
