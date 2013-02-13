// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// constants.go provides package-level constants for metrics.
package metrics

const (
	counterTypeValue   = "counter"
	floatBitCount      = 64
	floatFormat        = 'f'
	floatPrecision     = 6
	gaugeTypeValue     = "gauge"
	histogramTypeValue = "histogram"
	typeKey            = "type"
	valueKey           = "value"
	labelsKey          = "labels"
)
