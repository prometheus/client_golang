/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

/*
A Metric is something that can be exposed via the registry framework.
*/
type Metric interface {
	/*
	  Produce a human-consumable representation of the metric.
	*/
	Humanize() string
	/*
		Produce a JSON-consumable representation of the metric.
	*/
	Marshallable() map[string]interface{}
}
