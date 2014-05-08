// Copyright (c) 2014, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

import (
	"math/rand"
	"sync"
	"testing"
	"testing/quick"
)

func ExampleGauge() {
	delOps, _ := NewGauge(&Desc{
		Namespace: "our_company",
		Subsystem: "blob_storage",
		Name:      "deletes",
		Help:      "How many delete operations we have conducted against our blob storage system.",
	})

	delOps.Set(900) // That's all, folks!
}

func ExampleGaugeVec() {
	delOps, _ := NewGaugeVec(&Desc{
		Namespace: "our_company",
		Subsystem: "blob_storage",
		Name:      "deletes",
		Help:      "How many delete operations we have conducted against our blob storage system, partitioned by data corpus and qos.",
		VariableLabels: []string{
			// What is the body of data being deleted?
			"corpus",
			// How urgently do we need to delete the data?
			"qos",
		},
	})

	// Oops, we need to delete that embarrassing picture of ourselves.
	delOps.WithLabelValues("profile-pictures", "immediate").Set(4)
	// Those bad cat memes finally get deleted.
	delOps.WithLabels(map[string]string{"corpus": "cat-memes", "qos": "lazy"}).Set(1)
}

func listenGaugeStream(vals, final chan float64, done chan struct{}) {
	var last float64
outer:
	for {
		select {
		case <-done:
			close(vals)
			for last = range vals {
			}

			break outer
		case v := <-vals:
			last = v
		}
	}
	final <- last
	close(final)
}

func TestGaugeConcurrency(t *testing.T) {
	it := func(n uint32) bool {
		mutations := int(n % 10000)
		concLevel := int((n % 15) + 1)

		start := &sync.WaitGroup{}
		start.Add(1)
		end := &sync.WaitGroup{}
		end.Add(concLevel)

		sStream := make(chan float64, mutations*concLevel)
		final := make(chan float64)
		done := make(chan struct{})

		go listenGaugeStream(sStream, final, done)
		go func() {
			end.Wait()
			close(done)
		}()

		gge, err := NewGauge(&Desc{
			Name: "test_gauge",
			Help: "no help can be found here",
		})
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < concLevel; i++ {
			vals := make([]float64, 0, mutations)
			for j := 0; j < mutations; j++ {
				vals = append(vals, rand.NormFloat64())
			}

			go func(vals []float64) {
				start.Wait()
				for _, v := range vals {
					sStream <- v
					gge.Set(v)
				}
				end.Done()
			}(vals)
		}

		start.Done()

		last := <-final

		if last != gge.(*Value).val {
			t.Fatalf("expected %f, got %f", last, gge.(*Value).val)
			return false
		}

		return true
	}

	if err := quick.Check(it, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGaugeVecConcurrency(t *testing.T) {
	it := func(n uint32) bool {
		mutations := int(n % 10000)
		concLevel := int((n % 15) + 1)

		start := &sync.WaitGroup{}
		start.Add(1)
		end := &sync.WaitGroup{}
		end.Add(concLevel)

		sStream := make(chan float64, mutations*concLevel)
		final := make(chan float64)
		done := make(chan struct{})

		go listenGaugeStream(sStream, final, done)
		go func() {
			end.Wait()
			close(done)
		}()

		gge, err := NewGauge(&Desc{
			Name: "test_gauge",
			Help: "no help can be found here",
		})
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < concLevel; i++ {
			vals := make([]float64, 0, mutations)
			for j := 0; j < mutations; j++ {
				vals = append(vals, rand.NormFloat64())
			}

			go func(vals []float64) {
				start.Wait()
				for _, v := range vals {
					sStream <- v
					gge.Set(v)
				}
				end.Done()
			}(vals)
		}

		start.Done()

		last := <-final

		if last != gge.(*Value).val {
			t.Fatalf("expected %f, got %f", last, gge.(*Value).val)
			return false
		}

		return true
	}

	if err := quick.Check(it, nil); err != nil {
		t.Fatal(err)
	}
}
