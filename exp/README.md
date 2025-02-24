# client_golang experimental module

Contains experimental utilities and APIs for Prometheus.
The module may be contain breaking changes or be removed in the future.

Packages within this module are listed below.

## Remote

Implements bindings from Prometheus remote APIs (remote write v1 and v2 for now).

Contains flexible method for building API clients, that can send remote write protocol messages.

```go
    import (
        "github.com/prometheus/client_golang/exp/api/remote"
    )
    ...

	remoteAPI, err := remote.NewAPI(
		url,
		remote.WithAPIHTTPClient(httpClient),
		remote.WithAPILogger(logger.With("component", "remote_write_api")),
	)
    ...

    stats, err := remoteAPI.Write(ctx, remote.WriteV2ContentType, protoWriteReq)
```

Also contains handler methods for applications that would like to handle and store remote write requests.

```go
    import (
        "github.com/prometheus/client_golang/exp/api/remote"
    )
    ...
    
    type db {}

    func NewStorage() *db {}

    func (d *db) Store(ctx context.Context, cType remote.WriteContentType, req *http.Request) (*WriteResponse, error) {}
    ...

    storage = NewStorage()
    remoteWriteHandler := remote.NewHandler(storage, remote.WithHandlerLogger(logger.With("component", "remote_write_handler")))
```

For more details, see [go doc](https://pkg.go.dev/github.com/prometheus/client_golang/exp/api/remote).