// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package os2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"

	"github.com/uber/cadence/common/elasticsearch/bulk"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

// metricsRequest is used for a callback/metrics needs
var metricsRequest = []bulk.GenericBulkableRequest{&bulk.BulkIndexRequest{}}

type bulkProcessor struct {
	processor opensearchutil.BulkIndexer
	before    bulk.GenericBulkBeforeFunc
	after     bulk.GenericBulkAfterFunc
	logger    log.Logger
}

func (v *bulkProcessor) Start(ctx context.Context) error {
	return nil
}

func (v *bulkProcessor) Stop() error {
	return v.Close()
}
func (v *bulkProcessor) Close() error {
	return v.processor.Close(context.Background())
}

func (v *bulkProcessor) Add(request *bulk.GenericBulkableAddRequest) {

	req := opensearchutil.BulkIndexerItem{
		Index:      request.Index,
		DocumentID: request.ID,
	}
	var callBackRequest bulk.GenericBulkableRequest
	switch request.RequestType {
	case bulk.BulkableDeleteRequest:
		req.Action = "delete"
		req.Version = &request.Version
		req.VersionType = &request.VersionType
		callBackRequest = bulk.NewBulkDeleteRequest().
			ID(request.ID).
			Index(request.Index).
			Version(request.Version).
			VersionType(request.VersionType)
	case bulk.BulkableIndexRequest:
		body, err := json.Marshal(request.Doc)
		if err != nil {
			v.logger.Error("marshal bulk index request doc", tag.Error(err))
			return
		}
		req.Action = "index"
		req.Version = &request.Version
		req.VersionType = &request.VersionType
		req.Body = bytes.NewReader(body)
		callBackRequest = bulk.NewBulkIndexRequest().
			ID(request.ID).
			Index(request.Index).
			Version(request.Version).
			VersionType(request.VersionType).Doc(request.Doc)
	case bulk.BulkableCreateRequest:
		body, err := json.Marshal(request.Doc)
		if err != nil {
			v.logger.Error("marshal bulk create request doc", tag.Error(err))
			return
		}
		versionType := "internal"
		req.Action = "create"
		req.VersionType = &versionType
		req.Body = bytes.NewReader(body)
		callBackRequest = bulk.NewBulkIndexRequest().ID(request.ID).
			Index(request.Index).
			Version(request.Version).
			VersionType(versionType).Doc(request.Doc)

	}

	req.OnFailure = func(ctx context.Context, item opensearchutil.BulkIndexerItem, res opensearchutil.BulkIndexerResponseItem, err error) {
		v.processCallback(0, callBackRequest, req, res, false, err)
	}

	req.OnSuccess = func(ctx context.Context, item opensearchutil.BulkIndexerItem, res opensearchutil.BulkIndexerResponseItem) {
		v.processCallback(0, callBackRequest, req, res, true, nil)
	}
	if err := v.processor.Add(context.Background(), req); err != nil {
		v.logger.Error("adding bulk request to OpenSearch", tag.Error(err))
	}
}

// processCallback processes both success and failure scenarios.
func (v *bulkProcessor) processCallback(index int64, callBackRequest bulk.GenericBulkableRequest, req opensearchutil.BulkIndexerItem, res opensearchutil.BulkIndexerResponseItem, onSuccess bool, err error) {
	v.before(0, metricsRequest)

	gr := []bulk.GenericBulkableRequest{callBackRequest}

	it := bulk.GenericBulkResponseItem{
		Index:   res.Index,
		ID:      res.DocumentID,
		Version: res.Version,
		Status:  res.Status,
		Error:   res.Error,
	}

	if onSuccess {
		gbr := bulk.GenericBulkResponse{
			Took:   0,
			Errors: false,
			Items: []map[string]*bulk.GenericBulkResponseItem{
				{req.Action: &it},
			},
		}

		v.after(0, gr, &gbr, nil /*No errors here*/)
	} else {
		gbr := bulk.GenericBulkResponse{
			Took:   0,
			Errors: res.Error.Type != "" || res.Status > 201,
			Items: []map[string]*bulk.GenericBulkResponseItem{
				{req.Action: &it},
			},
		}

		ge := bulk.GenericError{
			Status:  res.Status,
			Details: err,
		}

		v.after(0, gr, &gbr, &ge)
	}
}

func (v *bulkProcessor) Flush() error {
	// Opensearch client doesn't provide flush method
	return nil
}

func (c *OS2) RunBulkProcessor(_ context.Context, parameters *bulk.BulkProcessorParameters) (bulk.GenericBulkProcessor, error) {
	processor, err := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		Client:        c.client,
		FlushInterval: parameters.FlushInterval,
		NumWorkers:    parameters.NumOfWorkers,
		Decoder:       &NumberDecoder{},
	})

	if err != nil {
		return nil, err
	}

	return &bulkProcessor{
		processor: processor,
		before:    parameters.BeforeFunc,
		after:     parameters.AfterFunc,
		logger:    c.logger,
	}, nil
}

// NumberDecoder uses json.NewDecoder, with UseNumber() enabled, from
// the Go standard library to decode JSON data.
type NumberDecoder struct{}

func (u *NumberDecoder) UnmarshalFromReader(reader io.Reader, response *opensearchutil.BulkIndexerResponse) error {
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	return dec.Decode(response)
}

func (u *NumberDecoder) Decode(reader io.Reader, v interface{}) error {
	dec := json.NewDecoder(reader)
	dec.UseNumber()
	return dec.Decode(v)
}
