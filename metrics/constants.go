/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

/*
constants.go provides package-level constants for metrics.
*/
package metrics

const (
	valueKey           = "value"
	gaugeTypeValue     = "gauge"
	counterTypeValue   = "counter"
	typeKey            = "type"
	histogramTypeValue = "histogram"
	floatFormat        = 'f'
	floatPrecision     = 6
	floatBitCount      = 64
)
