package prometheus

import (
	"strings"
	"testing"
)

// panicCollector panics in its Collect/Describe methods, simulating a
// misbehaving user collector (the scenario from issue #1877 that PR #1961 set
// out to make non-fatal via safeCollect).
type panicCollector struct {
	desc           *Desc
	panicInCollect bool
}

func (c *panicCollector) Describe(ch chan<- *Desc) {
	if !c.panicInCollect {
		panic("boom from wrapped Describe")
	}
	ch <- c.desc
}

func (c *panicCollector) Collect(ch chan<- Metric) {
	if c.panicInCollect {
		panic("boom from wrapped Collect")
	}
}

// TestWrappedCollectorCollectPanicRecovered verifies that a panic raised by a
// collector registered through WrapRegistererWith is recovered during Gather
// (turned into an error) rather than crashing the whole process.
//
// PR #1961 added safeCollect to recover panics from collector.Collect, but a
// wrappingCollector runs the inner collector's Collect in a NEW goroutine, so
// on unfixed code the panic escapes safeCollect (recover only catches panics
// from the same goroutine) and crashes the process.
func TestWrappedCollectorCollectPanicRecovered(t *testing.T) {
	reg := NewRegistry()
	pc := &panicCollector{desc: NewDesc("probe_metric", "help", nil, nil), panicInCollect: true}
	wrapped := WrapRegistererWith(Labels{"zone": "a"}, reg)
	wrapped.MustRegister(pc)

	mfs, err := reg.Gather()
	if err == nil {
		t.Fatalf("expected a recovered-panic error from Gather, got nil (mfs=%d)", len(mfs))
	}
	if !strings.Contains(err.Error(), "panic recovered") {
		t.Fatalf("expected panic-recovered error, got: %v", err)
	}
	t.Logf("Collect panic recovered as expected: %v", err)
}

// TestWrappedCollectorDescribePanicRecovered verifies the symmetric Describe
// path: a panic in a wrapped collector's Describe (e.g. during registration of
// a checked collector) is recovered instead of crashing the process.
func TestWrappedCollectorDescribePanicRecovered(t *testing.T) {
	reg := NewRegistry()
	pc := &panicCollector{desc: NewDesc("probe_metric", "help", nil, nil), panicInCollect: false}
	wrapped := WrapRegistererWith(Labels{"zone": "a"}, reg)

	// Registration triggers Describe on the checked collector. With the fix,
	// the panic is recovered and surfaced as a registration error rather than
	// crashing the process.
	err := wrapped.Register(pc)
	if err == nil {
		t.Fatalf("expected a recovered-panic error from Register/Describe, got nil")
	}
	if !strings.Contains(err.Error(), "panic recovered") {
		t.Fatalf("expected panic-recovered error, got: %v", err)
	}
	t.Logf("Describe panic recovered as expected: %v", err)
}
