/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package utility

import (
	. "github.com/matttproud/gocheck"
)

func (s *S) TestEmptyOptional(c *C) {
	var o *Optional = EmptyOptional()

	c.Assert(o, Not(IsNil))
	c.Check(o.IsSet(), Equals, false)
	c.Assert("default", Equals, o.Or("default"))
}

func (s *S) TestOf(c *C) {
	var o *Optional = Of(1)

	c.Assert(o, Not(IsNil))
	c.Check(o.IsSet(), Equals, true)
	c.Check(o.Get(), Equals, 1)
	c.Check(o.Or(2), Equals, 1)
}
