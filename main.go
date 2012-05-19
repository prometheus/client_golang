package main

import (
	"github.com/matttproud/golang_instrumentation/export"
	"net/http"
)

func main() {
	http.Handle("/metrics.json", export.Exporter)
	http.ListenAndServe(":8080", nil)
}
