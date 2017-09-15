package prometheus

import (
	"testing"
)

func Test_My(t *testing.T) {
	c1 := NewCounter(CounterOpts{
		"name": "test_counter",
		"help": "test Help",
	})
	c1.Inc()
	if c1.Value() == float64(1) {
		t.Log("pass")
	}
}
