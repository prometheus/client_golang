// Copyright 2016 The Prometheus Authors
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

// Copyright (c) 2013, The Prometheus Authors
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Package promhttp contains functions to create http.Handler instances to
// expose Prometheus metrics via HTTP. In later versions of this package, it
// will also contain tooling to instrument instances of http.Handler and
// http.RoundTripper.
//
// promhttp.Handler acts on the prometheus.DefaultGatherer. With HandlerFor,
// you can create a handler for a custom registry or anything that implements
// the Gatherer interface. It also allows to create handlers that act
// differently on errors or allow to log errors.
package promhttp

import "testing"

func TestMiddlewareWrapsFirstToLast(t *testing.T) {
}
