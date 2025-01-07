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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination indexer_mock.go github.com/uber/cadence/service/worker/indexer ESProcessor

package indexer

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/codec"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig"
	es "github.com/uber/cadence/common/elasticsearch"
	"github.com/uber/cadence/common/elasticsearch/bulk"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/messaging"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

const (
	versionTypeExternal = "external"
	processorName       = "visibility-processor"
)

var (
	errUnknownMessageType = &types.BadRequestError{Message: "unknown message type"}
	defaultEncoder        = codec.NewThriftRWEncoder()
)

type (
	ESProcessor interface {
		common.Daemon
		Add(request *bulk.GenericBulkableAddRequest, key string, kafkaMsg messaging.Message)
	}
	// Indexer used to consumer data from kafka then send to ElasticSearch
	Indexer struct {
		esIndexName         string
		consumer            messaging.Consumer
		visibilityProcessor ESProcessor
		config              *Config
		logger              log.Logger
		scope               metrics.Scope
		msgEncoder          codec.BinaryEncoder

		isStarted  int32
		isStopped  int32
		shutdownWG sync.WaitGroup
		shutdownCh chan struct{}
	}

	// Config contains all configs for indexer
	Config struct {
		IndexerConcurrency             dynamicconfig.IntPropertyFn
		ESProcessorNumOfWorkers        dynamicconfig.IntPropertyFn
		ESProcessorBulkActions         dynamicconfig.IntPropertyFn // max number of requests in bulk
		ESProcessorBulkSize            dynamicconfig.IntPropertyFn // max total size of bytes in bulk
		ESProcessorFlushInterval       dynamicconfig.DurationPropertyFn
		ValidSearchAttributes          dynamicconfig.MapPropertyFn
		EnableQueryAttributeValidation dynamicconfig.BoolPropertyFn
	}
)

// NewIndexer create a new Indexer
func NewIndexer(
	config *Config,
	client messaging.Client,
	visibilityClient es.GenericClient,
	visibilityName string,
	logger log.Logger,
	metricsClient metrics.Client,
) *Indexer {
	logger = logger.WithTags(tag.ComponentIndexer)

	visibilityProcessor, err := newESProcessor(processorName, config, visibilityClient, logger, metricsClient)
	if err != nil {
		logger.Fatal("Index ES processor state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	consumer, err := client.NewConsumer(common.VisibilityAppName, getConsumerName(visibilityName))
	if err != nil {
		logger.Fatal("Index consumer state changed", tag.LifeCycleStartFailed, tag.Error(err))
	}

	return &Indexer{
		config:              config,
		esIndexName:         visibilityName,
		consumer:            consumer,
		logger:              logger.WithTags(tag.ComponentIndexerProcessor),
		scope:               metricsClient.Scope(metrics.IndexProcessorScope),
		shutdownCh:          make(chan struct{}),
		visibilityProcessor: visibilityProcessor,
		msgEncoder:          defaultEncoder,
	}
}

func getConsumerName(topic string) string {
	return fmt.Sprintf("%s-consumer", topic)
}

func (i *Indexer) Start() error {
	if !atomic.CompareAndSwapInt32(&i.isStarted, 0, 1) {
		return nil
	}

	if err := i.consumer.Start(); err != nil {
		i.logger.Info("Index consumer state changed", tag.LifeCycleStartFailed, tag.Error(err))
		return err
	}

	i.visibilityProcessor.Start()

	i.shutdownWG.Add(1)
	go i.processorPump()

	i.logger.Info("Index bulkProcessor state changed", tag.LifeCycleStarted)
	return nil
}

func (i *Indexer) Stop() {
	if !atomic.CompareAndSwapInt32(&i.isStopped, 0, 1) {
		return
	}

	i.logger.Info("Index bulkProcessor state changed", tag.LifeCycleStopping)
	defer i.logger.Info("Index bulkProcessor state changed", tag.LifeCycleStopped)

	if atomic.LoadInt32(&i.isStarted) == 1 {
		close(i.shutdownCh)
	}

	if success := common.AwaitWaitGroup(&i.shutdownWG, time.Minute); !success {
		i.logger.Info("Index bulkProcessor state changed", tag.LifeCycleStopTimedout)
	}
}

func (i *Indexer) processorPump() {
	defer i.shutdownWG.Done()

	var workerWG sync.WaitGroup
	for workerID := 0; workerID < i.config.IndexerConcurrency(); workerID++ {
		workerWG.Add(1)
		go i.messageProcessLoop(&workerWG)
	}

	<-i.shutdownCh
	// Processor is shutting down, close the underlying consumer and esProcessor
	i.consumer.Stop()
	i.visibilityProcessor.Stop()

	i.logger.Info("Index bulkProcessor pump shutting down.")
	if success := common.AwaitWaitGroup(&workerWG, 10*time.Second); !success {
		i.logger.Warn("Index bulkProcessor timed out on worker shutdown.")
	}
}

func (i *Indexer) messageProcessLoop(workerWG *sync.WaitGroup) {
	defer workerWG.Done()

	for msg := range i.consumer.Messages() {
		sw := i.scope.StartTimer(metrics.IndexProcessorProcessMsgLatency)
		err := i.process(msg)
		sw.Stop()
		if err != nil {
			msg.Nack() //nolint:errcheck
		}
	}
}

func (i *Indexer) process(kafkaMsg messaging.Message) error {
	logger := i.logger.WithTags(tag.KafkaPartition(kafkaMsg.Partition()), tag.KafkaOffset(kafkaMsg.Offset()), tag.AttemptStart(time.Now()))

	indexMsg, err := i.deserialize(kafkaMsg.Value())
	if err != nil {
		logger.Error("Failed to deserialize index messages.", tag.Error(err))
		i.scope.IncCounter(metrics.IndexProcessorCorruptedData)
		return err
	}

	return i.addMessageToES(indexMsg, kafkaMsg, logger)
}

func (i *Indexer) deserialize(payload []byte) (*indexer.Message, error) {
	var msg indexer.Message
	if err := i.msgEncoder.Decode(payload, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (i *Indexer) addMessageToES(indexMsg *indexer.Message, kafkaMsg messaging.Message, logger log.Logger) error {
	docID := es.GenerateDocID(indexMsg.GetWorkflowID(), indexMsg.GetRunID())
	// check and skip invalid docID
	if len(docID) >= es.GetESDocIDSizeLimit() {
		logger.Error("Index message is too long",
			tag.WorkflowDomainID(indexMsg.GetDomainID()),
			tag.WorkflowID(indexMsg.GetWorkflowID()),
			tag.WorkflowRunID(indexMsg.GetRunID()))
		kafkaMsg.Nack()
		return nil
	}

	var keyToKafkaMsg string
	req := &bulk.GenericBulkableAddRequest{
		Index:       i.esIndexName,
		Type:        es.GetESDocType(),
		ID:          docID,
		VersionType: versionTypeExternal,
		Version:     indexMsg.GetVersion(),
	}
	switch indexMsg.GetMessageType() {
	case indexer.MessageTypeIndex:
		keyToKafkaMsg = fmt.Sprintf("%v-%v", kafkaMsg.Partition(), kafkaMsg.Offset())
		req.Doc = i.generateESDoc(indexMsg, keyToKafkaMsg)
		req.RequestType = bulk.BulkableIndexRequest
	case indexer.MessageTypeDelete:
		keyToKafkaMsg = docID
		req.RequestType = bulk.BulkableDeleteRequest
	case indexer.MessageTypeCreate:
		keyToKafkaMsg = fmt.Sprintf("%v-%v", kafkaMsg.Partition(), kafkaMsg.Offset())
		req.Doc = i.generateESDoc(indexMsg, keyToKafkaMsg)
		req.RequestType = bulk.BulkableCreateRequest
	default:
		logger.Error("Unknown message type")
		i.scope.IncCounter(metrics.IndexProcessorCorruptedData)
		return errUnknownMessageType
	}

	i.visibilityProcessor.Add(req, keyToKafkaMsg, kafkaMsg)

	return nil
}

func (i *Indexer) generateESDoc(msg *indexer.Message, keyToKafkaMsg string) map[string]interface{} {
	doc := i.dumpFieldsToMap(msg.Fields, msg.GetDomainID())
	fulfillDoc(doc, msg, keyToKafkaMsg)
	return doc
}

func (i *Indexer) decodeSearchAttrBinary(bytes []byte, key string) interface{} {
	var val interface{}
	err := json.Unmarshal(bytes, &val)
	if err != nil {
		i.logger.Error("Error when decode search attributes values.", tag.Error(err), tag.ESField(key))
		i.scope.IncCounter(metrics.IndexProcessorCorruptedData)
	}
	return val
}

func (i *Indexer) dumpFieldsToMap(fields map[string]*indexer.Field, domainID string) map[string]interface{} {
	doc := make(map[string]interface{})
	attr := make(map[string]interface{})
	for k, v := range fields {
		if !i.isValidFieldToES(k) {
			i.logger.Error("Unregistered field.", tag.ESField(k), tag.WorkflowDomainID(domainID))
			i.scope.IncCounter(metrics.IndexProcessorCorruptedData)
			continue
		}

		// skip VisibilityOperation since it’s not being used for advanced visibility
		if k == es.VisibilityOperation {
			continue
		}

		switch v.GetType() {
		case indexer.FieldTypeString:
			doc[k] = v.GetStringData()
		case indexer.FieldTypeInt:
			doc[k] = v.GetIntData()
		case indexer.FieldTypeBool:
			doc[k] = v.GetBoolData()
		case indexer.FieldTypeBinary:
			if k == definition.Memo {
				doc[k] = v.GetBinaryData()
			} else { // custom search attributes
				attr[k] = i.decodeSearchAttrBinary(v.GetBinaryData(), k)
			}
		default:
			// there must be bug in code and bad deployment, check data sent from producer
			i.logger.Fatal("Unknown field type")
		}
	}
	doc[definition.Attr] = attr
	return doc
}

func (i *Indexer) isValidFieldToES(field string) bool {
	if !i.config.EnableQueryAttributeValidation() {
		return true
	}
	if _, ok := i.config.ValidSearchAttributes()[field]; ok {
		return true
	}
	if field == definition.Memo || field == definition.KafkaKey || field == definition.Encoding || field == es.VisibilityOperation {
		return true
	}
	return false
}

func fulfillDoc(doc map[string]interface{}, msg *indexer.Message, keyToKafkaMsg string) {
	doc[definition.DomainID] = msg.GetDomainID()
	doc[definition.WorkflowID] = msg.GetWorkflowID()
	doc[definition.RunID] = msg.GetRunID()
	doc[definition.KafkaKey] = keyToKafkaMsg
}
