package prometheus

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	dto "github.com/prometheus/client_model/go"
)

// A Pusher sends the metrics of a Registry to a remote endpoint, like a
// prometheus-exporter.
type Pusher struct {
	registry Registry
	client   *http.Client
}

// NewPusher returns a new Pusher for the given Registry.
func NewPusher(r Registry) *Pusher {
	return &Pusher{
		registry: r,
		client:   &http.Client{},
	}
}

// SetTimeout sets a timeout for the Pusher's connections.
func (p *Pusher) SetTimeout(d time.Duration) {
	var dial func(network, addr string) (net.Conn, error)

	if d != 0 {
		dial = func(network, address string) (net.Conn, error) {
			deadline := time.Now().Add(d)

			conn, err := (&net.Dialer{Deadline: deadline}).Dial(network, address)

			if err == nil {
				conn.SetDeadline(deadline)
			}

			return conn, err
		}
	}

	p.client.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial:  dial,

		// If connection timeouts are enabled, we need to disable keepalives.
		DisableKeepAlives: dial != nil,
	}
}

// Push sends a PUT request to the provided endpoint. The content of the
// request is a protobuf encoded of the registry's metrics.
// It returns an error for any response other than a 204.
func (p *Pusher) Push(endpoint string) error {
	r, w := io.Pipe()

	req, err := http.NewRequest("PUT", endpoint, r)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", dto.SampleContentType)

	go func() {
		w.CloseWithError(p.registry.dumpSamples(w))
	}()

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode != 204 {
		return &url.Error{
			Op:  "PUT",
			URL: endpoint,
			Err: errors.New(http.StatusText(resp.StatusCode)),
		}
	}

	return nil
}
