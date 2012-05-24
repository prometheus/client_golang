/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package utility

type Optional struct {
	value interface{}
}

func EmptyOptional() *Optional {
	emission := &Optional{value: nil}

	return emission
}

func Of(value interface{}) *Optional {
	emission := &Optional{value: value}

	return emission
}

func (o *Optional) IsSet() bool {
	return o.value != nil
}

func (o *Optional) Get() interface{} {
	if o.value == nil {
		panic("Expected a value to be set.")
	}

	return o.value
}

func (o *Optional) Or(a interface{}) interface{} {
	if o.IsSet() {
		return o.Get()
	}
	return a
}
