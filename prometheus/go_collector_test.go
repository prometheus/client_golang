package prometheus

import (
	"runtime/debug"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestGoCollector(t *testing.T) {
	var (
		c      = NewGoCollector()
		ch     = make(chan Metric)
		waitc  = make(chan struct{})
		closec = make(chan struct{})
		old    = -1
	)
	defer close(closec)

	go func() {
		c.Collect(ch)
		go func(c <-chan struct{}) {
			<-c
		}(closec)
		<-waitc
		c.Collect(ch)
	}()

	for {
		select {
		case metric := <-ch:
			switch m := metric.(type) {
			// Attention, this also catches Counter...
			case Gauge:
				pb := &dto.Metric{}
				m.Write(pb)
				if pb.GetGauge() == nil {
					continue
				}

				if old == -1 {
					old = int(pb.GetGauge().GetValue())
					close(waitc)
					continue
				}

				if diff := int(pb.GetGauge().GetValue()) - old; diff != 1 {
					// TODO: This is flaky in highly concurrent situations.
					t.Errorf("want 1 new goroutine, got %d", diff)
				}

				return
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("expected collect timed out")
		}
	}
}

func TestGCCollector(t *testing.T) {
	var (
		c        = NewGoCollector()
		ch       = make(chan Metric)
		waitc    = make(chan struct{})
		closec   = make(chan struct{})
		oldGC    uint64
		oldPause float64
	)
	defer close(closec)

	go func() {
		c.Collect(ch)
		// force GC
		debug.FreeOSMemory()
		<-waitc
		c.Collect(ch)
	}()

	first := true
	for {
		select {
		case metric := <-ch:
			switch m := metric.(type) {
			case *constSummary, *value:
				pb := &dto.Metric{}
				m.Write(pb)
				if pb.GetSummary() == nil {
					continue
				}

				if first {
					first = false
					oldGC = *pb.GetSummary().SampleCount
					oldPause = *pb.GetSummary().SampleSum
					close(waitc)
					continue
				}
				if diff := *pb.GetSummary().SampleCount - oldGC; diff != 1 {
					t.Errorf("want 1 new garbage collection run, got %d", diff)
				}
				if diff := *pb.GetSummary().SampleSum - oldPause; diff <= 0 {
					t.Errorf("want moar pause, got %f", diff)
				}
				return
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("expected collect timed out")
		}
	}
}
