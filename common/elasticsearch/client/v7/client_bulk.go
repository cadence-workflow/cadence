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

package v7

import (
	"context"
	"time"

	"github.com/olivere/elastic/v7"

	"github.com/uber/cadence/common/elasticsearch/bulk"
)

type // bulkProcessorParametersV7 holds all required and optional parameters for executing bulk service
bulkProcessorParametersV7 struct {
	Name          string
	NumOfWorkers  int
	BulkActions   int
	BulkSize      int
	FlushInterval time.Duration
	Backoff       elastic.Backoff
	BeforeFunc    elastic.BulkBeforeFunc
	AfterFunc     elastic.BulkAfterFunc
}

type v7BulkProcessor struct {
	processor *elastic.BulkProcessor
}

func (v *v7BulkProcessor) Start(ctx context.Context) error {
	return v.processor.Start(ctx)
}

func (v *v7BulkProcessor) Stop() error {
	return v.processor.Stop()
}

func (v *v7BulkProcessor) Close() error {
	return v.processor.Close()
}

func (v *v7BulkProcessor) Add(request *bulk.GenericBulkableAddRequest) {
	var req elastic.BulkableRequest
	switch request.RequestType {
	case bulk.BulkableDeleteRequest:
		req = elastic.NewBulkDeleteRequest().
			Index(request.Index).
			Id(request.ID)
	case bulk.BulkableIndexRequest:
		req = elastic.NewBulkIndexRequest().
			Index(request.Index).
			Id(request.ID).
			VersionType(request.VersionType).
			Version(request.Version).
			Doc(request.Doc)
	case bulk.BulkableCreateRequest:
		// for bulk create request still calls the bulk index method
		// with providing operation type
		req = elastic.NewBulkIndexRequest().
			OpType("create").
			Index(request.Index).
			Id(request.ID).
			VersionType("internal").
			Doc(request.Doc)
	}
	v.processor.Add(req)
}

func (v *v7BulkProcessor) Flush() error {
	return v.processor.Flush()
}

func (c *ElasticV7) runBulkProcessor(ctx context.Context, p *bulkProcessorParametersV7) (*v7BulkProcessor, error) {
	processor, err := c.client.BulkProcessor().
		Name(p.Name).
		Workers(p.NumOfWorkers).
		BulkActions(p.BulkActions).
		BulkSize(p.BulkSize).
		FlushInterval(p.FlushInterval).
		Backoff(p.Backoff).
		Before(p.BeforeFunc).
		After(p.AfterFunc).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	return &v7BulkProcessor{
		processor: processor,
	}, nil
}

func fromV7ToGenericBulkResponse(response *elastic.BulkResponse) *bulk.GenericBulkResponse {
	if response == nil {
		return &bulk.GenericBulkResponse{}
	}
	return &bulk.GenericBulkResponse{
		Took:   response.Took,
		Errors: response.Errors,
		Items:  fromV7ToGenericBulkResponseItemMaps(response.Items),
	}
}

func fromV7ToGenericBulkResponseItemMaps(items []map[string]*elastic.BulkResponseItem) []map[string]*bulk.GenericBulkResponseItem {
	var gitems []map[string]*bulk.GenericBulkResponseItem
	for _, it := range items {
		gitems = append(gitems, fromV7ToGenericBulkResponseItemMap(it))
	}
	return gitems
}

func fromV7ToGenericBulkResponseItemMap(m map[string]*elastic.BulkResponseItem) map[string]*bulk.GenericBulkResponseItem {
	if m == nil {
		return nil
	}
	gm := make(map[string]*bulk.GenericBulkResponseItem, len(m))
	for k, v := range m {
		gm[k] = fromV7ToGenericBulkResponseItem(v)
	}
	return gm
}

func fromV7ToGenericBulkResponseItem(v *elastic.BulkResponseItem) *bulk.GenericBulkResponseItem {
	return &bulk.GenericBulkResponseItem{
		Index:   v.Index,
		Type:    v.Type,
		ID:      v.Id,
		Version: v.Version,
		Result:  v.Result,
		Status:  v.Status,
	}
}

func fromV7ToGenericBulkableRequests(requests []elastic.BulkableRequest) []bulk.GenericBulkableRequest {
	var v7Reqs []bulk.GenericBulkableRequest
	for _, req := range requests {
		v7Reqs = append(v7Reqs, req)
	}
	return v7Reqs
}

func (c *ElasticV7) RunBulkProcessor(ctx context.Context, parameters *bulk.BulkProcessorParameters) (bulk.GenericBulkProcessor, error) {
	beforeFunc := func(executionId int64, requests []elastic.BulkableRequest) {
		parameters.BeforeFunc(executionId, fromV7ToGenericBulkableRequests(requests))
	}

	afterFunc := func(executionId int64, requests []elastic.BulkableRequest, response *elastic.BulkResponse, err error) {
		parameters.AfterFunc(
			executionId,
			fromV7ToGenericBulkableRequests(requests),
			fromV7ToGenericBulkResponse(response),
			convertV7ErrorToGenericError(err))
	}

	return c.runBulkProcessor(ctx, &bulkProcessorParametersV7{
		Name:          parameters.Name,
		NumOfWorkers:  parameters.NumOfWorkers,
		BulkActions:   parameters.BulkActions,
		BulkSize:      parameters.BulkSize,
		FlushInterval: parameters.FlushInterval,
		Backoff:       parameters.Backoff,
		BeforeFunc:    beforeFunc,
		AfterFunc:     afterFunc,
	})
}
func convertV7ErrorToGenericError(err error) *bulk.GenericError {
	if err == nil {
		return nil
	}
	status := bulk.UnknownStatusCode
	switch e := err.(type) {
	case *elastic.Error:
		status = e.Status
	}
	return &bulk.GenericError{
		Status:  status,
		Details: err,
	}
}
