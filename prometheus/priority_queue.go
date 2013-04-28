// Copyright (c) 2013, Prometheus Team
// All rights reserved.

// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package prometheus

type item struct {
	Priority int64
	Value    interface{}
	index    int
}

type priorityQueue []*item

func (q priorityQueue) Len() int {
	return len(q)
}

func (q priorityQueue) Less(i, j int) bool {
	return q[i].Priority > q[j].Priority
}

func (q priorityQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *priorityQueue) Push(x interface{}) {
	queue := *q
	size := len(queue)
	queue = queue[0 : size+1]
	item := x.(*item)
	item.index = size
	queue[size] = item
	*q = queue
}

func (q *priorityQueue) Pop() interface{} {
	queue := *q
	size := len(queue)
	item := queue[size-1]
	item.index = -1
	*q = queue[0 : size-1]
	return item
}
