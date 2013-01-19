/*
Copyright (c) 2012, Matt T. Proud
All rights reserved.

Use of this source code is governed by a BSD-style
license that can be found in the LICENSE file.
*/

package utility

type Item struct {
	Priority int64
	Value    interface{}
	index    int
}

type PriorityQueue []*Item

func (q PriorityQueue) Len() int {
	return len(q)
}

func (q PriorityQueue) Less(i, j int) bool {
	return q[i].Priority > q[j].Priority
}

func (q PriorityQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *PriorityQueue) Push(x interface{}) {
	queue := *q
	size := len(queue)
	queue = queue[0 : size+1]
	item := x.(*Item)
	item.index = size
	queue[size] = item
	*q = queue
}

func (q *PriorityQueue) Pop() interface{} {
	queue := *q
	size := len(queue)
	item := queue[size-1]
	item.index = -1
	*q = queue[0 : size-1]
	return item
}
