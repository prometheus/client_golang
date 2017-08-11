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

package alertmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/common/model"
)

const (
	statusAPIError = 422
	apiPrefix      = "/api/v1"

	epSilence     = "/silence/:id"
	epSilences    = "/silences"
	epAlerts      = "/alerts"
	epAlertGroups = "/alerts/groups"
)

}

// apiClient wraps a regular client and processes successful API responses.
// Successful also includes responses that errored at the API level.
type apiClient struct {
	api.Client
}

type apiResponse struct {
	Status    string          `json:"status"`
	Data      json.RawMessage `json:"data"`
	ErrorType api.ErrorType   `json:"errorType"`
	Error     string          `json:"error"`
}

func (c apiClient) Do(ctx context.Context, req *http.Request) (*http.Response, []byte, error) {
	resp, body, err := c.Client.Do(ctx, req)
	if err != nil {
		return resp, body, err
	}

	code := resp.StatusCode

	if code/100 != 2 && code != statusAPIError {
		return resp, body, &api.Error{
			Type: api.ErrBadResponse,
			Msg:  fmt.Sprintf("bad response code %d", resp.StatusCode),
		}
	}

	var result apiResponse

	if err = json.Unmarshal(body, &result); err != nil {
		return resp, body, &api.Error{
			Type: api.ErrBadResponse,
			Msg:  err.Error(),
		}
	}

	if (code == statusAPIError) != (result.Status == "error") {
		err = &api.Error{
			Type: api.ErrBadResponse,
			Msg:  "inconsistent body for response code",
		}
	}

	if code == statusAPIError && result.Status == "error" {
		err = &api.Error{
			Type: result.ErrorType,
			Msg:  result.Error,
		}
	}

	return resp, []byte(result.Data), err
}

type AlertAPI interface {
	// List all the active alerts.
	List(ctx context.Context) ([]*model.Alert, error)
	// Push a list of alerts into the Alertmanager.
	Push(ctx context.Context, alerts ...*model.Alert) error
}

// NewAlertAPI returns a new AlertAPI for the client.
func NewAlertAPI(c api.Client) AlertAPI {
	return &httpAlertAPI{client: apiClient{c}}
}

type httpAlertAPI struct {
	client api.Client
}

func (h *httpAlertAPI) List(ctx context.Context) ([]*model.Alert, error) {
	u := h.client.URL(epAlerts, nil)

	req, _ := http.NewRequest("GET", u.String(), nil)

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var alts []*model.Alert
	err = json.Unmarshal(body, &alts)

	return alts, err
}

func (h *httpAlertAPI) Push(ctx context.Context, alerts ...*model.Alert) error {
	u := h.client.URL(epAlerts, nil)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(alerts); err != nil {
		return err
	}

	req, _ := http.NewRequest("POST", u.String(), &buf)

	_, _, err := h.client.Do(ctx, req)
	return err
}

// SilenceAPI provides bindings the Alertmanager's silence API.
type SilenceAPI interface {
	// Get returns the silence associated with the given ID.
	Get(ctx context.Context, id uint64) (*model.Silence, error)
	// Set updates or creates the given silence and returns its ID.
	Set(ctx context.Context, sil *model.Silence) (uint64, error)
	// Del deletes the silence with the given ID.
	Del(ctx context.Context, id uint64) error
	// List all silences of the server.
	List(ctx context.Context) ([]*model.Silence, error)
}

// NewSilenceAPI returns a new SilenceAPI for the client.
func NewSilenceAPI(c api.Client) SilenceAPI {
	return &httpSilenceAPI{client: apiClient{c}}
}

type httpSilenceAPI struct {
	client api.Client
}

func (h *httpSilenceAPI) Get(ctx context.Context, id uint64) (*model.Silence, error) {
	u := h.client.URL(epSilence, map[string]string{
		"id": strconv.FormatUint(id, 10),
	})

	req, _ := http.NewRequest("GET", u.String(), nil)

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var sil model.Silence
	err = json.Unmarshal(body, &sil)

	return &sil, err
}

func (h *httpSilenceAPI) Del(ctx context.Context, id uint64) error {
	u := h.client.URL(epSilence, map[string]string{
		"id": strconv.FormatUint(id, 10),
	})

	req, _ := http.NewRequest("DELETE", u.String(), nil)

	_, _, err := h.client.Do(ctx, req)
	return err
}

func (h *httpSilenceAPI) Set(ctx context.Context, sil *model.Silence) (uint64, error) {
	var (
		u      *url.URL
		method string
	)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(sil); err != nil {
		return 0, err
	}

	// Talk to different endpoints depending on whether its a new silence or not.
	if sil.ID != 0 {
		u = h.client.URL(epSilence, map[string]string{
			"id": strconv.FormatUint(sil.ID, 10),
		})
		method = "PUT"
	} else {
		u = h.client.URL(epSilences, nil)
		method = "POST"
	}

	req, _ := http.NewRequest(method, u.String(), &buf)

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return 0, err
	}

	var res struct {
		SilenceID uint64 `json:"silenceId"`
	}
	err = json.Unmarshal(body, &res)

	return res.SilenceID, err
}

func (h *httpSilenceAPI) List(ctx context.Context) ([]*model.Silence, error) {
	u := h.client.URL(epSilences, nil)

	req, _ := http.NewRequest("GET", u.String(), nil)

	_, body, err := h.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	var sils []*model.Silence
	err = json.Unmarshal(body, &sils)

	return sils, err
}
