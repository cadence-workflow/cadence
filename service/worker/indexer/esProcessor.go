// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/collection"
	"github.com/uber/cadence/common/definition"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/elasticsearch/bulk"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
)

const (
	// retry configs for es bulk bulkProcessor
	esProcessorInitialRetryInterval = 200 * time.Millisecond
	esProcessorMaxRetryInterval     = 20 * time.Second
)

type (
	// ESProcessorImpl implements ESProcessor, it's an agent of GenericBulkProcessor
	ESProcessorImpl struct {
		bulkProcessor bulk.GenericBulkProcessor
		mapToKafkaMsg collection.ConcurrentTxMap // used to map ES request to kafka message
		config        *Config
		logger        log.Logger
		scope         metrics.Scope
		msgEncoder    codec.BinaryEncoder
	}

	kafkaMessageWithMetrics struct { // value of ESProcessorImpl.mapToKafkaMsg
		message        messaging.Message
		swFromAddToAck *metrics.Stopwatch // metric from message add to process, to message ack/nack
	}
)

// newESProcessor creates new ESProcessor
func newESProcessor(
	name string,
	config *Config,
	client es.GenericClient,
	logger log.Logger,
	metricsClient metrics.Client,
) (*ESProcessorImpl, error) {
	p := &ESProcessorImpl{
		config:     config,
		logger:     logger.WithTags(tag.ComponentIndexerESProcessor),
		scope:      metricsClient.Scope(metrics.ESProcessorScope),
		msgEncoder: defaultEncoder,
	}

	params := &bulk.BulkProcessorParameters{
		Name:          name,
		NumOfWorkers:  config.ESProcessorNumOfWorkers(),
		BulkActions:   config.ESProcessorBulkActions(),
		BulkSize:      config.ESProcessorBulkSize(),
		FlushInterval: config.ESProcessorFlushInterval(),
		Backoff:       bulk.NewExponentialBackoff(esProcessorInitialRetryInterval, esProcessorMaxRetryInterval),
		BeforeFunc:    p.bulkBeforeAction,
		AfterFunc:     p.bulkAfterAction,
	}
	processor, err := client.RunBulkProcessor(context.Background(), params)
	if err != nil {
		return nil, err
	}

	p.bulkProcessor = processor
	p.mapToKafkaMsg = collection.NewShardedConcurrentTxMap(1024, p.hashFn)
	return p, nil
}

func (p *ESProcessorImpl) Start() {
	// current implementation (v6 and v7) allows to invoke Start() multiple times
	p.bulkProcessor.Start(context.Background())

}
func (p *ESProcessorImpl) Stop() {
	p.bulkProcessor.Stop() //nolint:errcheck
	p.mapToKafkaMsg = nil
}

// Add an ES request, and an map item for kafka message
func (p *ESProcessorImpl) Add(request *bulk.GenericBulkableAddRequest, key string, kafkaMsg messaging.Message) {
	actionWhenFoundDuplicates := func(key interface{}, value interface{}) error {
		return kafkaMsg.Ack()
	}
	sw := p.scope.StartTimer(metrics.ESProcessorProcessMsgLatency)
	mapVal := newKafkaMessageWithMetrics(kafkaMsg, &sw)
	_, isDup, _ := p.mapToKafkaMsg.PutOrDo(key, mapVal, actionWhenFoundDuplicates)
	if isDup {
		return
	}
	p.bulkProcessor.Add(request)
}

// bulkBeforeAction is triggered before bulk bulkProcessor commit
func (p *ESProcessorImpl) bulkBeforeAction(executionID int64, requests []bulk.GenericBulkableRequest) {
	p.scope.AddCounter(metrics.ESProcessorRequests, int64(len(requests)))
}

// bulkAfterAction is triggered after bulk bulkProcessor commit
func (p *ESProcessorImpl) bulkAfterAction(id int64, requests []bulk.GenericBulkableRequest, response *bulk.GenericBulkResponse, err *bulk.GenericError) {
	if err != nil {
		// This happens after configured retry, which means something bad happens on cluster or index
		// When cluster back to live, bulkProcessor will re-commit those failure requests
		p.logger.Error("Error commit bulk request.", tag.Error(err.Details))

		isRetryable := isResponseRetriable(err.Status)
		for _, request := range requests {
			if isRetryable {
				// retryable errors will be retried by the bulk processor
				p.logger.Error("ES request failed", tag.ESRequest(request.String()))
			} else {
				key := p.retrieveKafkaKey(request)
				if key == "" {
					continue
				}
				wid, rid, domainID := p.getMsgWithInfo(key)
				p.logger.Error("ES request failed and is not retryable",
					tag.ESResponseStatus(err.Status),
					tag.ESRequest(request.String()),
					tag.WorkflowID(wid),
					tag.WorkflowRunID(rid),
					tag.WorkflowDomainID(domainID))

				// check if it is a delete request and status code
				// 404 means the document does not exist
				// 409 means means the document's version does not match (or if the document has been updated or deleted by another process)
				// this can happen during the data migration, the doc was deleted in the old index but not exists in the new index
				if err.Status == 409 {
					p.logger.Info("Request encountered a version conflict. Acknowledging to prevent retry.",
						tag.ESResponseStatus(err.Status), tag.ESRequest(request.String()),
						tag.WorkflowID(wid),
						tag.WorkflowRunID(rid),
						tag.WorkflowDomainID(domainID))
					p.ackKafkaMsg(key)
				} else if err.Status == 404 {
					req, err := request.Source()
					if err == nil && p.isDeleteRequest(req) {
						p.logger.Info("Document has been deleted. Acknowledging to prevent retry.",
							tag.ESResponseStatus(404), tag.ESRequest(request.String()),
							tag.WorkflowID(wid),
							tag.WorkflowRunID(rid),
							tag.WorkflowDomainID(domainID))
						p.ackKafkaMsg(key)
					} else {
						p.logger.Error("Get request source err.", tag.Error(err), tag.ESRequest(request.String()))
						p.scope.IncCounter(metrics.ESProcessorCorruptedData)
						p.nackKafkaMsg(key)
					}
				} else {
					// For all other non-retryable errors, nack the message
					p.nackKafkaMsg(key)
				}
			}
			p.scope.IncCounter(metrics.ESProcessorFailures)
		}
		return
	}

	responseItems := response.Items
	for i := 0; i < len(requests); i++ {
		key := p.retrieveKafkaKey(requests[i])
		if key == "" {
			continue
		}
		responseItem := responseItems[i]
		for _, resp := range responseItem {
			switch {
			case isResponseSuccess(resp.Status):
				p.ackKafkaMsg(key)
			case !isResponseRetriable(resp.Status):
				wid, rid, domainID := p.getMsgWithInfo(key)
				p.logger.Error("ES request failed.",
					tag.ESResponseStatus(resp.Status), tag.ESResponseError(getErrorMsgFromESResp(resp)), tag.WorkflowID(wid), tag.WorkflowRunID(rid),
					tag.WorkflowDomainID(domainID))
				p.nackKafkaMsg(key)
			default: // bulk bulkProcessor will retry
				p.logger.Info("ES request retried.", tag.ESResponseStatus(resp.Status))
				p.scope.IncCounter(metrics.ESProcessorRetries)
			}
		}
	}
}

func (p *ESProcessorImpl) ackKafkaMsg(key string) {
	p.ackKafkaMsgHelper(key, false)
}

func (p *ESProcessorImpl) nackKafkaMsg(key string) {
	p.ackKafkaMsgHelper(key, true)
}

func (p *ESProcessorImpl) ackKafkaMsgHelper(key string, nack bool) {
	kafkaMsg, ok := p.getKafkaMsg(key)
	if !ok {
		return
	}

	if nack {
		kafkaMsg.Nack()
	} else {
		kafkaMsg.Ack()
	}

	p.mapToKafkaMsg.Remove(key)
}

func (p *ESProcessorImpl) getKafkaMsg(key string) (kafkaMsg *kafkaMessageWithMetrics, ok bool) {
	msg, ok := p.mapToKafkaMsg.Get(key)
	if !ok {
		return // duplicate kafka message
	}
	kafkaMsg, ok = msg.(*kafkaMessageWithMetrics)
	if !ok { // must be bug in code and bad deployment
		p.logger.Fatal("Message is not kafka message.", tag.ESKey(key))
	}
	return kafkaMsg, ok
}

func (p *ESProcessorImpl) retrieveKafkaKey(request bulk.GenericBulkableRequest) string {
	req, err := request.Source()
	if err != nil {
		p.logger.Error("Get request source err.", tag.Error(err), tag.ESRequest(request.String()))
		p.scope.IncCounter(metrics.ESProcessorCorruptedData)
		return ""
	}

	var key string
	if !p.isDeleteRequest(req) { // index or update requests
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(req[1]), &body); err != nil {
			p.logger.Error("Unmarshal index request body err.", tag.Error(err))
			p.scope.IncCounter(metrics.ESProcessorCorruptedData)
			return ""
		}

		k, ok := body[definition.KafkaKey]
		if !ok {
			// must be bug in code and bad deployment, check processor that add es requests
			panic(definition.KafkaKey + " not found")
		}
		key, ok = k.(string)
		if !ok {
			// must be bug in code and bad deployment, check processor that add es requests
			panic("kafkaKey is not string")
		}
	} else { // delete requests
		var body map[string]map[string]interface{}
		if err := json.Unmarshal([]byte(req[0]), &body); err != nil {
			p.logger.Error("Unmarshal delete request body err.", tag.Error(err))
			p.scope.IncCounter(metrics.ESProcessorCorruptedData)
			return ""
		}
		opMap, ok := body["delete"]
		if !ok {
			// must be bug, check if dependency changed
			panic("delete key not found in request")
		}
		k, ok := opMap["_id"]
		if !ok {
			// must be bug in code and bad deployment, check processor that add es requests
			panic("_id not found in request opMap")
		}
		key, _ = k.(string)
	}
	return key
}

func (p *ESProcessorImpl) getMsgWithInfo(key string) (wid string, rid string, domainID string) {
	kafkaMsg, ok := p.getKafkaMsg(key)
	if !ok {
		return
	}

	var msg indexer.Message
	if err := p.msgEncoder.Decode(kafkaMsg.message.Value(), &msg); err != nil {
		p.logger.Error("failed to deserialize kafka message.", tag.Error(err))
		return
	}
	return msg.GetWorkflowID(), msg.GetRunID(), msg.GetDomainID()
}

func (p *ESProcessorImpl) hashFn(key interface{}) uint32 {
	id, ok := key.(string)
	if !ok {
		return 0
	}
	numOfShards := p.config.IndexerConcurrency()
	return uint32(common.WorkflowIDToHistoryShard(id, numOfShards))
}

func (p *ESProcessorImpl) isDeleteRequest(request []string) bool {
	// The Source() method typically returns a slice of strings, where each string represents a part of the bulk request in JSON format.
	// For delete operations, the Source() method typically returns only one part
	// The metadata that specifies the delete action, including _index and _id.
	// "{\"delete\":{\"_index\":\"my-index\",\"_id\":\"1\"}}"
	// For index/update operations, the Source() method typically returns two parts
	// reference: https://www.elastic.co/guide/en/elasticsearch/reference/6.8/docs-bulk.html
	return len(request) == 1
}

// 409 - Version Conflict
// 404 - Not Found
func isResponseSuccess(status int) bool {
	if status >= 200 && status < 300 || status == 409 || status == 404 {
		return true
	}
	return false
}

// isResponseRetriable is complaint with GenericBulkProcessorService.RetryItemStatusCodes
// responses with these status will be kept in queue and retried until success
// 408 - Request Timeout
// 429 - Too Many Requests
// 500 - Node not connected
// 503 - Service Unavailable
// 507 - Insufficient Storage
var retryableStatusCode = map[int]struct{}{408: {}, 429: {}, 500: {}, 503: {}, 507: {}}

func isResponseRetriable(status int) bool {
	_, ok := retryableStatusCode[status]
	return ok
}

func getErrorMsgFromESResp(resp *bulk.GenericBulkResponseItem) string {
	var errMsg string
	if resp.Error != nil {
		errMsg = fmt.Sprintf("%v", resp.Error)
	}
	return errMsg
}

func newKafkaMessageWithMetrics(kafkaMsg messaging.Message, stopwatch *metrics.Stopwatch) *kafkaMessageWithMetrics {
	return &kafkaMessageWithMetrics{
		message:        kafkaMsg,
		swFromAddToAck: stopwatch,
	}
}

func (km *kafkaMessageWithMetrics) Ack() {
	km.message.Ack() // nolint:errcheck
	if km.swFromAddToAck != nil {
		km.swFromAddToAck.Stop()
	}
}

func (km *kafkaMessageWithMetrics) Nack() {
	km.message.Nack() //nolint:errcheck
	if km.swFromAddToAck != nil {
		km.swFromAddToAck.Stop()
	}
}
