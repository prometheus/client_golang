// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// registry.go provides a container for centralized exposition of metrics to
// their prospective consumers.
//
//  registry.Register("human_readable_metric_name", "metric docstring", map[string]string{"baseLabel": "baseLabelValue"}, metric)
package registry
