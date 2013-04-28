// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// Prometheus' client side metric primitives and telemetry exposition framework.
//
// This package provides both metric primitives and tools for their exposition
// to the Prometheus time series collection and computation framework.
//
//  prometheus.Register("human_readable_metric_name", "metric docstring", map[string]string{"baseLabel": "baseLabelValue"}, metric)
//
// The examples under github.com/prometheus/client_golang/examples should be
// consulted.
package prometheus
