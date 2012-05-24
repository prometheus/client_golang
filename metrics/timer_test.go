/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package metrics

import (
	. "launchpad.net/gocheck"
	"time"
)

func (s *S) TestTimerStart(c *C) {
	stopWatch := Start(nil)

	c.Assert(stopWatch, Not(IsNil))
	c.Assert(stopWatch.startTime, Not(IsNil))
}

func (s *S) TestTimerStop(c *C) {
	done := make(chan bool)

	var callbackInvoked bool = false
	var complete CompletionCallback = func(duration time.Duration) {
		callbackInvoked = true
		done <- true
	}

	stopWatch := Start(complete)

	c.Check(callbackInvoked, Equals, false)

	d := stopWatch.Stop()

	<-done

	c.Assert(d, Not(IsNil))
	c.Check(callbackInvoked, Equals, true)
}

func (s *S) TestInstrumentCall(c *C) {
	var callbackInvoked bool = false
	var instrumentableInvoked bool = false
	done := make(chan bool, 2)

	var complete CompletionCallback = func(duration time.Duration) {
		callbackInvoked = true
		done <- true
	}

	var instrumentable InstrumentableCall = func() {
		instrumentableInvoked = true
		done <- true
	}

	d := InstrumentCall(instrumentable, complete)

	c.Assert(d, Not(IsNil))

	<-done
	<-done

	c.Check(instrumentableInvoked, Equals, true)
	c.Check(callbackInvoked, Equals, true)
}
