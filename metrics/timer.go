// Copyright (c) 2012, Matt T. Proud
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// timer.go provides a scalar metric that times how long a given event takes.

package metrics

import (
	"time"
)

// This callback is called upon the completion of the timer—i.e., when it stops.
type CompletionCallback func(duration time.Duration)

// This is meant to capture a function that a StopWatch can call for purposes
// of instrumentation.
type InstrumentableCall func()

type StopWatch struct {
	startTime    time.Time
	endTime      time.Time
	onCompletion CompletionCallback
}

// Return a new StopWatch that is ready for instrumentation.
func Start(onCompletion CompletionCallback) *StopWatch {
	return &StopWatch{
		startTime:    time.Now(),
		onCompletion: onCompletion,
	}
}

// Stop the StopWatch returning the elapsed duration of its lifetime while
// firing an optional CompletionCallback in the background.
func (s *StopWatch) Stop() time.Duration {
	s.endTime = time.Now()
	duration := s.endTime.Sub(s.startTime)

	if s.onCompletion != nil {
		go s.onCompletion(duration)
	}

	return duration
}

// Provide a quick way of instrumenting a InstrumentableCall and emitting its
// duration.
func InstrumentCall(instrumentable InstrumentableCall, onCompletion CompletionCallback) time.Duration {
	s := Start(onCompletion)
	instrumentable()
	return s.Stop()
}
