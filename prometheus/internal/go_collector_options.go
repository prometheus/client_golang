package internal

import "regexp"

type GoCollectorRule struct {
	Matcher *regexp.Regexp
	Deny    bool
	Buckets interface{}
}

// GoCollectorOptions should not be used be directly by anything, except `collectors` package.
// Use it via collectors package instead. See issue
// https://github.com/prometheus/client_golang/issues/1030.
//
// This is internal, so external users only can use it via `collector.WithGoCollector*` methods
type GoCollectorOptions struct {
	DisableMemStatsLikeMetrics bool
	RuntimeMetricSumForHist    map[string]string
	RuntimeMetricRules         []GoCollectorRule
}
