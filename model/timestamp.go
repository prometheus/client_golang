// Copyright 2013 Prometheus Team
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

package model

import (
	"fmt"
	native_time "time"
)

// TODO(julius): Should this use milliseconds/nanoseconds instead? This is
//               mostly hidden from the user of these types when using the
//               methods below, so it will be easy to change this later
//               without requiring significant user code changes.

// Time in seconds since the epoch (January 1, 1970 UTC).
type Timestamp int64

// Duration in seconds.
type Duration int64

const (
	// Second must be less or equal to the value of time.Second!
	Second Duration = 1
	Minute          = 60 * Second
	Hour            = 60 * Minute
)

// Equal reports whether two timestamps represent the same instant.
func (t Timestamp) Equal(o Timestamp) bool {
	return t == o
}

// Before reports whether the timestamp t is before o.
func (t Timestamp) Before(o Timestamp) bool {
	return t < o
}

// Before reports whether the timestamp t is after o.
func (t Timestamp) After(o Timestamp) bool {
	return t > o
}

// Add returns the Timestamp t + d.
func (t Timestamp) Add(d Duration) Timestamp {
	return t + Timestamp(d)
}

// Sub returns the Duration t - o.
func (t Timestamp) Sub(o Timestamp) Duration {
	return Duration(t - o)
}

// Time returns the time.Time representation of t.
func (t Timestamp) Time() native_time.Time {
	return native_time.Unix(int64(t)/int64(Second), (int64(t) % int64(Second)))
}

// Unix returns t as a Unix time, the number of seconds elapsed
// since January 1, 1970 UTC.
func (t Timestamp) Unix() int64 {
	return int64(t) / int64(Second)
}

// String returns a string representation of the timestamp.
func (t Timestamp) String() string {
	return fmt.Sprint(int64(t))
}

// Now returns the current time as a Timestamp.
func Now() Timestamp {
	return TimestampFromTime(native_time.Now())
}

// TimestampFromTime returns the Timestamp equivalent to the time.Time t.
func TimestampFromTime(t native_time.Time) Timestamp {
	return TimestampFromUnix(t.Unix())
}

// TimestampFromUnix returns the Timestamp equivalent to the Unix timestamp t.
func TimestampFromUnix(t int64) Timestamp {
	return Timestamp(t * int64(Second))
}

// TimeDuration returns the time.Duration equivalent to d.
func (d Duration) TimeDuration() native_time.Duration {
	return native_time.Duration(d) * (native_time.Second / native_time.Duration(Second))
}

// NewDuration creates a new Duration equivalent to the time.Duration t.
func NewDuration(t native_time.Duration) Duration {
	return Duration(t / (native_time.Second / native_time.Duration(Second)))
}
