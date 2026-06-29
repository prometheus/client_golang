# client_golang Tutorial: Export Prometheus instrumentation with OpenTelemetry OTLP

`client_golang` is built around the Prometheus pull model. For most long-running
services, exposing `/metrics` with `promhttp` is still the recommended setup.

If you already instrumented your code with `client_golang` but need to send
metrics into an OpenTelemetry pipeline, you can bridge a Prometheus
`Gatherer` into the OpenTelemetry SDK and export it with OTLP.

This is useful for:

- short-lived batch jobs that may exit before being scraped
- environments standardized on OTLP and the OpenTelemetry Collector
- gradual migrations where existing Prometheus instrumentation should stay
  unchanged

## Example

The example below keeps existing Prometheus instrumentation, bridges a custom
registry into the OpenTelemetry SDK, and exports the resulting metrics with the
OTLP HTTP exporter.

```go
package main

import (
	"context"
	"log"

	bridgeprometheus "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	ctx := context.Background()

	reg := prometheus.NewRegistry()
	jobsProcessed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "jobs_processed_total",
		Help: "Total number of processed jobs.",
	})
	reg.MustRegister(jobsProcessed)

	exporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpoint("localhost:4318"),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		log.Fatal(err)
	}

	bridge := bridgeprometheus.NewMetricProducer(
		bridgeprometheus.WithGatherer(reg),
	)

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithProducer(bridge)),
		),
	)
	defer func() {
		if err := meterProvider.Shutdown(ctx); err != nil {
			log.Printf("shutdown metric provider: %v", err)
		}
	}()

	jobsProcessed.Inc()
}
```

## Notes

- If your application uses the default Prometheus registry, you can omit
  `bridgeprometheus.WithGatherer(reg)` and `NewMetricProducer` will read from
  `prometheus.DefaultGatherer`.
- `Shutdown` flushes pending metric data. For short-lived jobs, make sure you
  shut the provider down before exit.
- If you also use native OpenTelemetry metrics, attach them to the same
  `MeterProvider` so both pipelines are exported together.

## Related documentation

- Prometheus bridge package:
  <https://pkg.go.dev/go.opentelemetry.io/contrib/bridges/prometheus>
- OTLP metric HTTP exporter:
  <https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp>
