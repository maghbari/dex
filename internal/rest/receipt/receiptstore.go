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

package receipt

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/auth"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/receipt/api"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/hyperledger/firefly-fabconnect/internal/ws"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const (
	defaultReceiptLimit      = 10
	defaultRetryTimeout      = 120 * 1000
	defaultRetryInitialDelay = 500
	backoffFactor            = 1.1
)

var uuidCharsVerifier, _ = regexp.Compile("^[0-9a-zA-Z-]+$")

type ReceiptStore interface {
	Init(ws.WebSocketChannels, ...api.ReceiptStorePersistence) error
	ValidateConf() error
	ProcessReceipt(msgBytes []byte)
	GetReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	GetReceipt(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	Close()
}

type receiptStore struct {
	config      *conf.ReceiptsDBConf
	persistence api.ReceiptStorePersistence
	ws          ws.WebSocketChannels
}

func NewReceiptStore(config *conf.RESTGatewayConf) ReceiptStore {
	var receiptStorePersistence api.ReceiptStorePersistence
	if config.Receipts.LevelDB.Path != "" {
		leveldbStore := newLevelDBReceipts(&config.Receipts)
		receiptStorePersistence = leveldbStore
	} else if config.Receipts.MongoDB.URL != "" {
		mongoStore := newMongoReceipts(&config.Receipts)
		receiptStorePersistence = mongoStore
	} else {
		memStore := newMemoryReceipts(&config.Receipts)
		receiptStorePersistence = memStore
	}

	if config.Receipts.RetryTimeoutMS <= 0 {
		config.Receipts.RetryTimeoutMS = defaultRetryTimeout
	}
	if config.Receipts.RetryInitialDelayMS <= 0 {
		config.Receipts.RetryInitialDelayMS = defaultRetryInitialDelay
	}
	return &receiptStore{
		config:      &config.Receipts,
		persistence: receiptStorePersistence,
	}
}
func (r *receiptStore) ValidateConf() error {
	return r.persistence.ValidateConf()
}

func (r *receiptStore) Init(ws ws.WebSocketChannels, mocked ...api.ReceiptStorePersistence) error {
	r.ws = ws
	if mocked != nil {
		// only used in test code to pass in a mocked impl
		r.persistence = mocked[0]
		return nil
	} else {
		// the regular runtime does this
		return r.persistence.Init()
	}
}

func (r *receiptStore) extractHeaders(parsedMsg map[string]interface{}) map[string]interface{} {
	if iHeaders, exists := parsedMsg["headers"]; exists {
		if headers, ok := iHeaders.(map[string]interface{}); ok {
			return headers
		}
	}
	return nil
}

func (r *receiptStore) ProcessReceipt(msgBytes []byte) {

	// Parse the reply as JSON
	var parsedMsg map[string]interface{}
	if err := json.Unmarshal(msgBytes, &parsedMsg); err != nil {
		log.Errorf("Unable to unmarshal reply message '%s' as JSON: %s", string(msgBytes), err)
		return
	}

	// Extract the headers
	headers := r.extractHeaders(parsedMsg)
	if headers == nil {
		log.Errorf("Failed to extract request headers from '%+v'", parsedMsg)
		return
	}

	// The one field we require is the original ID (as it's the key in MongoDB)
	requestID := utils.GetMapString(headers, "requestId")
	if requestID == "" {
		log.Errorf("Failed to extract headers.requestId from '%+v'", parsedMsg)
		return
	}
	reqOffset := utils.GetMapString(headers, "reqOffset")
	msgType := utils.GetMapString(headers, "type")
	result := ""
	if msgType == messages.MsgTypeError {
		result = utils.GetMapString(parsedMsg, "errorMessage")
	} else {
		result = utils.GetMapString(parsedMsg, "transactionHash")
	}
	log.Infof("Received reply message. requestId='%s' reqOffset='%s' type='%s': %s", requestID, reqOffset, msgType, result)

	parsedMsg["receivedAt"] = time.Now().UnixNano() / int64(time.Millisecond)
	parsedMsg["_id"] = requestID

	// Insert the receipt into persistence - captures errors
	if requestID != "" && r.persistence != nil {
		r.writeReceipt(requestID, parsedMsg)
	}

}

func (r *receiptStore) writeReceipt(requestID string, receipt map[string]interface{}) {
	startTime := time.Now()
	delay := time.Duration(r.config.RetryInitialDelayMS) * time.Millisecond
	attempt := 0
	retryTimeout := time.Duration(r.config.RetryTimeoutMS) * time.Millisecond

	for {
		if attempt > 0 {
			log.Infof("%s: Waiting %.2fs before re-attempt:%d mongo write", requestID, delay.Seconds(), attempt)
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * backoffFactor)
		}
		attempt++
		err := r.persistence.AddReceipt(requestID, &receipt)
		if err == nil {
			log.Infof("%s: Inserted receipt into receipt store", receipt["_id"])
			break
		}

		log.Errorf("%s: addReceipt attempt: %d failed, err: %s", requestID, attempt, err)

		// Check if the reason is that there is a receipt already
		existing, qErr := r.persistence.GetReceipt(requestID)
		if qErr == nil && existing != nil {
			log.Warnf("%s: existing  receipt: %+v", requestID, *existing)
			log.Warnf("%s: duplicate receipt: %+v", requestID, receipt)
			break
		}

		timeRetrying := time.Since(startTime)
		if timeRetrying > retryTimeout {
			log.Infof("%s: receipt: %+v", requestID, receipt)
			log.Panicf("%s: Failed to insert into receipt store after %.2fs: %s", requestID, timeRetrying.Seconds(), err)
		}
	}
	if r.ws != nil {
		r.ws.SendReply(receipt)
	}
}

// getReplies handles a HTTP request for recent replies
func (r *receiptStore) GetReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	err := auth.AuthListAsyncReplies(req.Context())
	if err != nil {
		log.Errorf("Error querying replies: %s", err)
		errors.RestErrReply(res, req, errors.Errorf(errors.Unauthorized), 401)
		return
	}

	// Default limit - which is set to zero (infinite) if we have specific IDs being request
	limit := defaultReceiptLimit
	_ = req.ParseForm()
	ids, ok := req.Form["id"]
	if ok {
		limit = 0 // can be explicitly set below, but no imposed limit when we have a list of IDs
		for idx, id := range ids {
			if !uuidCharsVerifier.MatchString(id) {
				log.Errorf("Invalid id '%s' %d", id, idx)
				errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreInvalidRequestID), 400)
				return
			}
		}
	}

	// Extract limit
	limitStr := req.FormValue("limit")
	if limitStr != "" {
		if customLimit, err := strconv.ParseInt(limitStr, 10, 32); err == nil {
			if int(customLimit) > r.config.QueryLimit {
				log.Errorf("Invalid limit value: %s", err)
				errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreInvalidRequestMaxLimit, r.config.QueryLimit), 400)
				return
			} else if customLimit > 0 {
				limit = int(customLimit)
			}
		} else {
			log.Errorf("Invalid limit value: %s", err)
			errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreInvalidRequestBadLimit), 400)
			return
		}
	}

	// Extract skip
	var skip int
	skipStr := req.FormValue("skip")
	if skipStr != "" {
		if skipI64, err := strconv.ParseInt(skipStr, 10, 32); err == nil && skipI64 > 0 {
			skip = int(skipI64)
		} else {
			log.Errorf("Invalid skip value: %s", err)
			errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreInvalidRequestBadSkip), 400)
			return
		}
	}

	// Verify since - if specified
	var sinceEpochMS int64
	since := req.FormValue("since")
	if since != "" {
		if isoTime, err := time.Parse(time.RFC3339Nano, since); err == nil {
			sinceEpochMS = isoTime.UnixNano() / int64(time.Millisecond)
		} else {
			if sinceEpochMS, err = strconv.ParseInt(since, 10, 64); err != nil {
				log.Errorf("since '%s' cannot be parsed as RFC3339 or millisecond timestamp: %s", since, err)
				errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreInvalidRequestBadSince), 400)
				return
			}
		}
	}

	from := req.FormValue("from")
	to := req.FormValue("to")
	start := req.FormValue("start")

	// Call the persistence tier - which must return an empty array when no results (not an error)
	results, err := r.persistence.GetReceipts(skip, limit, ids, sinceEpochMS, from, to, start)
	if err != nil {
		log.Errorf("Error querying replies: %s", err)
		errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreFailedQuery, err), 500)
		return
	}
	log.Debugf("Replies query: skip=%d limit=%d replies=%d", skip, limit, len(*results))
	r.marshalAndReply(res, req, results)

}

// getReply handles a HTTP request for an individual reply
func (r *receiptStore) GetReceipt(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	log.Infof("--> %s %s", req.Method, req.URL)

	err := auth.AuthReadAsyncReplyByUUID(req.Context())
	if err != nil {
		log.Errorf("Error querying reply: %s", err)
		errors.RestErrReply(res, req, errors.Errorf(errors.Unauthorized), 401)
		return
	}

	requestID := params.ByName("id")
	// Call the persistence tier - which must return an empty array when no results (not an error)
	result, err := r.persistence.GetReceipt(requestID)
	if err != nil {
		log.Errorf("Error querying reply: %s", err)
		errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreFailedQuerySingle, err), 500)
		return
	} else if result == nil {
		errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreFailedNotFound), 404)
		log.Infof("Reply not found")
		return
	}
	log.Infof("Reply found")
	r.marshalAndReply(res, req, result)
}

func (r *receiptStore) Close() {
	r.persistence.Close()
}

func (r *receiptStore) marshalAndReply(res http.ResponseWriter, req *http.Request, result interface{}) {
	// Serialize and return
	resBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Errorf("Error serializing receipts: %s", err)
		errors.RestErrReply(res, req, errors.Errorf(errors.ReceiptStoreSerializeResponse), 500)
		return
	}
	status := 200
	log.Infof("<-- %s %s [%d]", req.Method, req.URL, status)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	_, _ = res.Write(resBytes)
}
