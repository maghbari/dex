// Copyright 2021 Kaleido
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/events/api"

	log "github.com/sirupsen/logrus"
)

type webhookAction struct {
	es   *eventStream
	spec *webhookActionInfo
}

func validateWebhookConfig(spec *webhookActionInfo) error {
	if spec == nil || spec.URL == "" {
		return errors.Errorf(errors.EventStreamsWebhookNoURL)
	}
	if _, err := url.Parse(spec.URL); err != nil {
		return errors.Errorf(errors.EventStreamsWebhookInvalidURL)
	}
	return nil
}

func newWebhookAction(es *eventStream, spec *webhookActionInfo) (*webhookAction, error) {
	if spec.RequestTimeoutSec == 0 {
		spec.RequestTimeoutSec = 120
	}
	if spec.TLSkipHostVerify == nil {
		spec.TLSkipHostVerify = &trueValue
	}
	return &webhookAction{
		es:   es,
		spec: spec,
	}, nil
}

// attemptWebhookAction performs a single attempt of a webhook action
func (w *webhookAction) attemptBatch(batchNumber, attempt uint64, events []*api.EventEntry) error {
	// We perform DNS resolution before each attempt, to exclude private IP address ranges from the target
	esID := w.es.spec.ID
	u, _ := url.Parse(w.spec.URL)
	addr, err := net.ResolveIPAddr("ip4", u.Hostname())
	if err != nil {
		return err
	}
	if w.es.isAddressUnsafe(addr) {
		err := errors.Errorf(errors.EventStreamsWebhookProhibitedAddress, u.Hostname())
		log.Errorf(err.Error())
		return err
	}
	// Set the timeout
	var transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
	netClient := &http.Client{
		Timeout:   time.Duration(w.spec.RequestTimeoutSec) * time.Second,
		Transport: transport,
	}
	log.Infof("%s: POST --> %s [%s] (attempt=%d)", esID, u.String(), addr.String(), attempt)
	reqBytes, err := json.Marshal(&events)
	var req *http.Request
	if err == nil {
		req, err = http.NewRequest("POST", u.String(), bytes.NewReader(reqBytes))
	}
	if err == nil {
		var res *http.Response
		req.Header.Set("Content-Type", "application/json")
		for h, v := range w.spec.Headers {
			req.Header.Set(h, v)
		}
		res, err = netClient.Do(req)
		if err == nil {
			ok := (res.StatusCode >= 200 && res.StatusCode < 300)
			log.Infof("%s: POST <-- %s [%d] ok=%t", esID, u.String(), res.StatusCode, ok)
			if !ok || log.IsLevelEnabled(log.DebugLevel) {
				bodyBytes, _ := ioutil.ReadAll(res.Body)
				log.Infof("%s: Response body: %s", esID, string(bodyBytes))
			}
			if !ok {
				err = errors.Errorf(errors.EventStreamsWebhookFailedHTTPStatus, esID, res.StatusCode)
			}
		}
	}
	if err != nil {
		log.Errorf("%s: POST %s failed (attempt=%d): %s", esID, u.String(), attempt, err)
	}
	return err
}
