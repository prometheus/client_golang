// Copyright 2015 The Prometheus Authors
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

// A simple example of querying the prometheus api using the golang client

package main

import (
	"context"
	"fmt"
	prometheus "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostname := "" //The prometheus api url, in format "http(s)://<ip>:<port_name>" or "http(s)://prometheus.example.com/"
	podName := ""  //Pod name (optional), if memory usage of containers in a specific podd is required

	fmt.Println("Getting container memory usage...")

	config := prometheus.Config{Address: hostname, RoundTripper: prometheus.DefaultRoundTripper}
	client, err := prometheus.NewClient(config)
	if err != nil {
		fmt.Println(err)
	}
	httpApi := v1.NewAPI(client)

	//Query time series data(memory usage in bytes for containers) for 5m
	queryString := "container_memory_usage_bytes{pod_name=\"" + podName + "\"}[5m]"
	//Replace above query_string with your own query

	result, err := httpApi.Query(ctx, queryString, time.Now()) //returns a model.Value interface (defined in (https://github.com/prometheus/common/blob/master/model/value.go)

	if err != nil {
		fmt.Println(err)
	}

	numberOfContainersInPod := result.(model.Matrix).Len()
	fmt.Println("Number of containers in pod = ", numberOfContainersInPod)

	for i, s := range result.(model.Matrix) { //cast model.Value to model.Matrix which is an array of SamplePair structs (defined in (https://github.com/prometheus/common/blob/master/model/value.go)
		for _, k := range s.Values {
			fmt.Println("At(timestamp)="+k.Timestamp.String()+", memory usage(in bytes) for container #", i, "="+k.Value.String())
		}
	}
}
