// Copyright 2026 The Prometheus Authors
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

// ExpiredCleaner is implemented by collectors that support TTL-based cleanup of
// unused children (for example MetricVec with a non-zero Opts.TTL).
// Registry.Gather calls CleanupExpired on registered collectors that implement
// this interface so expired children can be reclaimed even when Collect alone
// would only skip them.
type ExpiredCleaner interface {
	CleanupExpired() int
}

// ttlMetric is implemented by decorator wrappers that track last access time.
type ttlMetric interface {
	Metric
	lastAccessed() int64
	touch()
}
