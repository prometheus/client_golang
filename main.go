package main

import (
	"github.com/matttproud/golang_instrumentation/export"
	"net/http"
)

func main() {
	exporter := export.DefaultRegistry.YieldExporter()

	http.Handle("/metrics.json", exporter)
	http.ListenAndServe(":8080", nil)
}
