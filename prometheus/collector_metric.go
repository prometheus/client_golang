// Copyright 2022 Percona
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

package prometheus

var (
	descScrapeTime *Desc
	metricCache    chan Metric
)

type MetaMetrics struct {
	scrapeTime  *Desc
	metricCache chan Metric
}

func NewMetaMetricsCollector() Collector {
	descScrapeTime = NewDesc(
		"collector_scrape_time_ms",
		"Time taken for scrape by collector",
		[]string{"exporter", "collector"},
		nil)

	return &MetaMetrics{
		scrapeTime:  descScrapeTime,
		metricCache: makeMetricsCache(),
	}
}

func makeMetricsCache() chan Metric {
	// TODO : Use a more appropriate collector count
	metricCache = make(chan Metric, 100)
	return metricCache
}

func GetScrapeDescripter() *Desc {
	return descScrapeTime
}

func PushMetaMetrics(m Metric) {
	metricCache <- m
}

func (c *MetaMetrics) Describe(ch chan<- *Desc) {
	ch <- c.scrapeTime
}

func (c *MetaMetrics) Collect(ch chan<- Metric) {
	close(c.metricCache)
	for m := range c.metricCache {
		ch <- m
	}
	c.metricCache = makeMetricsCache()
}
