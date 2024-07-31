package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type scenario int

const (
	contextNotCanceled scenario = 0
	goroutineJump      scenario = 1
)

type scenarios struct {
	enabled [2]bool
	mu      sync.RWMutex
}

func (s *scenarios) SetFromParam(c string, v bool) error {
	if c == "" {
		return errors.New("no {case} parameter in path")
	}
	cN, err := strconv.Atoi(c)
	if err != nil {
		return errors.New("{case} is not a number")
	}
	if cN < 0 || cN >= len(s.enabled) {
		return fmt.Errorf("{case} should be a number from 0 to %d", len(s.enabled)-1)
	}
	s.set(scenario(cN), v)
	return nil
}

func (s *scenarios) set(choice scenario, v bool) {
	s.mu.Lock()
	s.enabled[choice] = v
	s.mu.Unlock()
}

func (s *scenarios) IsEnabled(choice scenario) bool {
	s.mu.RLock()
	ret := s.enabled[choice]
	s.mu.RUnlock()
	return ret
}

func addContextNotCanceledGroup(g *run.Group, reg *prometheus.Registry, shouldBreak func() bool) {
	// Create latency metric for our app operation.
	opLatency := promauto.With(reg).NewHistogram(
		prometheus.HistogramOpts{
			Name:    "brokenapp_operation_latency_seconds",
			Help:    "Tracks the latencies for calls.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.3, 0.6, 1, 3, 6, 9, 20},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Custom contexts can happen...
	// Without it, Go has many clever tricks to avoid extra goroutines per context
	// cancel setup or timers.
	ctx = withCustomContext(ctx)
	g.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
			broken := shouldBreak()

			// Do an operation.
			ctx, cancel := context.WithTimeout(ctx, 1*time.Hour)
			if broken {
				// Bug: Cancel will run until the end of this function... so until program
				// exit of timeout. This means we are leaking goroutines here with
				// all their allocated memory (and a bit of memory for defer).
				defer cancel()
			}

			start := time.Now()
			ret := doOp(ctx)
			since := time.Since(start)
			opLatency.Observe(float64(since.Nanoseconds()) * 1e-9)

			fmt.Println("10 * 1e5th fibonacci number is", ret, "; elapsed", since.String())

			if !broken {
				cancel()
			}
		}
	}, func(err error) {
		cancel()
	})
}

func addGoroutineJumpGroup(g *run.Group, shouldBreak func() bool) {
	ctx, cancel := context.WithCancel(context.Background())
	g.Add(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(30 * time.Second):
			}

			if !shouldBreak() {
				continue
			}

			var wg sync.WaitGroup
			done := make(chan struct{})

			for i := 0; i < 300; i++ {
				time.Sleep(1 * time.Second)
				wg.Add(1)
				go func() {
					<-done
					wg.Done()
				}()
			}
			time.Sleep(30 * time.Second)
			close(done)
			wg.Wait()
		}
	}, func(err error) {
		cancel()
	})
}

type customCtx struct {
	context.Context
}

func withCustomContext(ctx context.Context) context.Context {
	return customCtx{Context: ctx}
}

func (c customCtx) Value(any) any {
	return nil // Noop to avoid optimizations to highlight the negative effect.
}
