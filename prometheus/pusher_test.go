package prometheus

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func TestPusherPush(t *testing.T) {
	var (
		registry = NewRegistry()
		counter  = NewCounter()

		pusher = NewPusher(registry)

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer r.Body.Close()

			time.Sleep(100 * time.Millisecond)

			if r.Header.Get("Content-Type") != dto.SampleContentType {
				http.Error(w, "", http.StatusNotAcceptable)
				return
			}

			var (
				sample = new(dto.Sample)
				dec    = dto.NewDecoder(r.Body)
			)

			for {
				if err := dec.Decode(sample); err != nil {
					if err == io.EOF {
						break
					}

					http.Error(w, "", http.StatusBadRequest)
					return
				}
			}

			w.WriteHeader(http.StatusNoContent)
		}))
	)

	defer server.Close()
	registry.Register("http_requests", "", nil, counter)

	counter.Increment(nil)
	counter.Increment(map[string]string{"foo": "bar"})

	if err := pusher.Push(server.URL); err != nil {
		t.Fatal("unexpected error:", err)
	}

	pusher.SetTimeout(1 * time.Millisecond)

	if err := pusher.Push(server.URL); !strings.Contains(err.Error(), "i/o timeout") {
		t.Fatal("expected timeout error, got:", err)
	}
}
