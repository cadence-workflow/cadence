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

package client

import (
	"context"
	"encoding/json"

	"github.com/uber/cadence/common/elasticsearch/bulk"
)

// Client is a generic ES client implementation.
// This interface allows to use different Elasticsearch and OpenSearch versions
// without exposing implementation details and structs
type Client interface {
	// ClearScroll clears the search context and results for a scrolling search.
	ClearScroll(ctx context.Context, scrollID string) error
	// Count returns number of document matches by given query
	Count(ctx context.Context, index, body string) (int64, error)
	// CreateIndex creates index with given name
	CreateIndex(ctx context.Context, index string) error
	// IsNotFoundError checks if error is a "not found"
	IsNotFoundError(err error) bool
	// PutMapping updates Client with new field mapping
	PutMapping(ctx context.Context, index, body string) error
	// RunBulkProcessor starts bulk indexing processor
	// @TODO consider to extract Bulk Processor as a separate entity
	RunBulkProcessor(ctx context.Context, p *bulk.BulkProcessorParameters) (bulk.GenericBulkProcessor, error)
	// Scroll retrieves the next batch of results for a scrolling search.
	Scroll(ctx context.Context, index, body, scrollID string) (*Response, error)
	// Search returns Elasticsearch hit bytes and additional metadata
	Search(ctx context.Context, index, body string) (*Response, error)
}

// Response is used to pass data retrieved from Elasticsearch/OpenSearch to upper layer
type Response struct {
	TookInMillis int64
	TotalHits    int64
	Hits         *SearchHits
	Aggregations map[string]json.RawMessage
	Sort         []interface{}
	ScrollID     string
}

// SearchHits specifies the list of search hits.
type SearchHits struct {
	TotalHits *TotalHits   // total number of hits found
	Hits      []*SearchHit // the actual hits returned
}

// TotalHits specifies total number of hits and its relation
type TotalHits struct {
	Value int64 // value of the total hit count
}

// SearchHit is a single hit.
type SearchHit struct {
	Index  string          // index name
	ID     string          // external or internal
	Sort   []interface{}   // sort information
	Source json.RawMessage // stored document source
}
