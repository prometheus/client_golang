package n9epush_test

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/n9epush"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var defaultPushGateway string = "http://localhost:2080/v1/push"

func Example_Push() {
	metrics := n9epush.NewMetrics("rd", "metrics", "test", nil)
	metrics.StartPushLoop("26", 1, defaultPushGateway)

	bucket := []float64{.005, .01, .025, .05, .075, .1, .25, .5, .75, 1, 2.5, 5, 7.5, 10}

}

func ExamplePusher_Push() {
	completionTime := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_backup_last_completion_timestamp_seconds",
		Help: "The timestamp of the last successful completion of a DB backup.",
	})
	completionTime.SetToCurrentTime()
	if err := push.New("http://pushgateway:9091", "db_backup").
		Collector(completionTime).
		Grouping("db", "customers").
		Push(); err != nil {
		fmt.Println("Could not push completion time to Pushgateway:", err)
	}
}
