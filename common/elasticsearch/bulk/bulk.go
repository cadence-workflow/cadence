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

package bulk

//go:generate mockgen -package mocks -destination mocks/GenericBulkableRequest.go github.com/uber/cadence/common/elasticsearch/bulk GenericBulkableRequest
//go:generate mockgen -package mocks -destination mocks/GenericBulkProcessor.go github.com/uber/cadence/common/elasticsearch/bulk GenericBulkProcessor

import (
	"context"
	"time"
)

const UnknownStatusCode = -1

type GenericBulkableRequestType int

const (
	BulkableIndexRequest GenericBulkableRequestType = iota
	BulkableDeleteRequest
	BulkableCreateRequest
)

type (
	// GenericBulkProcessor is a bulk processor
	GenericBulkProcessor interface {
		Start(ctx context.Context) error
		Stop() error
		Close() error
		Add(request *GenericBulkableAddRequest)
		Flush() error
	}

	// BulkProcessorParameters holds all required and optional parameters for executing bulk service
	BulkProcessorParameters struct {
		Name          string
		NumOfWorkers  int
		BulkActions   int
		BulkSize      int
		FlushInterval time.Duration
		Backoff       GenericBackoff
		BeforeFunc    GenericBulkBeforeFunc
		AfterFunc     GenericBulkAfterFunc
	}

	// GenericBackoff allows callers to implement their own Backoff strategy.
	GenericBackoff interface {
		// Next implements a BackoffFunc.
		Next(retry int) (time.Duration, bool)
	}

	// GenericBulkBeforeFunc defines the signature of callbacks that are executed
	// before a commit to Elasticsearch.
	GenericBulkBeforeFunc func(executionId int64, requests []GenericBulkableRequest)

	// GenericBulkAfterFunc defines the signature of callbacks that are executed
	// after a commit to Elasticsearch. The err parameter signals an error.
	GenericBulkAfterFunc func(executionId int64, requests []GenericBulkableRequest, response *GenericBulkResponse, err *GenericError)

	// GenericBulkableRequest is a generic interface to bulkable requests.
	GenericBulkableRequest interface {
		String() string
		Source() ([]string, error)
	}

	// GenericBulkableAddRequest a struct to hold a bulk request
	GenericBulkableAddRequest struct {
		Index       string
		Type        string
		ID          string
		VersionType string
		Version     int64
		// request types can be index, delete or create
		RequestType GenericBulkableRequestType
		// should be nil if IsDelete is true
		Doc interface{}
	}

	// GenericBulkResponse is generic struct of bulk response
	GenericBulkResponse struct {
		Took   int                                   `json:"took,omitempty"`
		Errors bool                                  `json:"errors,omitempty"`
		Items  []map[string]*GenericBulkResponseItem `json:"items,omitempty"`
	}

	// GenericError encapsulates error status and details returned from Elasticsearch.
	GenericError struct {
		Status  int   `json:"status"`
		Details error `json:"error,omitempty"`
	}

	// GenericBulkResponseItem is the result of a single bulk request.
	GenericBulkResponseItem struct {
		Index   string `json:"_index,omitempty"`
		Type    string `json:"_type,omitempty"`
		ID      string `json:"_id,omitempty"`
		Version int64  `json:"_version,omitempty"`
		Result  string `json:"result,omitempty"`
		Status  int    `json:"status,omitempty"`
		// the error details
		Error interface{}
	}
)
