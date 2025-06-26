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

package os2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/opensearch-project/opensearch-go/v4"
	osapi "github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v4/opensearchtransport"
	requestsigner "github.com/opensearch-project/opensearch-go/v4/signer/aws"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

type (
	// OS2 implements Client
	OS2 struct {
		client  *osapi.Client
		logger  log.Logger
		decoder *NumberDecoder
	}

	osError struct {
		Status  int           `json:"status"`
		Details *errorDetails `json:"error,omitempty"`
	}

	errorDetails struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
		Index  string `json:"index,omitempty"`
	}

	// response holds data retrieved from OpenSearch
	response struct {
		TookInMillis int64 `json:"took,omitempty"`
		Hits         *searchHits
		Aggregations map[string]json.RawMessage `json:"aggregations,omitempty"`
		Sort         []interface{}              `json:"sort,omitempty"` // sort information
		ScrollID     string                     `json:"_scroll_id,omitempty"`
	}

	// searchHits specifies the list of search hits.
	searchHits struct {
		TotalHits *totalHits   `json:"total,omitempty"` // total number of hits found
		Hits      []*searchHit `json:"hits,omitempty"`  // the actual hits returned
	}

	// totalHits specifies total number of hits and its relation
	totalHits struct {
		Value int64 `json:"value"` // value of the total hit count
	}

	// searchHit is a single hit.
	searchHit struct {
		Index  string          `json:"_index,omitempty"`  // index name
		ID     string          `json:"_id,omitempty"`     // external or internal
		Sort   []interface{}   `json:"sort,omitempty"`    // sort information
		Source json.RawMessage `json:"_source,omitempty"` // stored document source
	}

	convertLogger struct {
		logger log.Logger
	}
)

var _ opensearchtransport.Logger = (*convertLogger)(nil)

func (c convertLogger) LogRoundTrip(request *http.Request, h *http.Response, err error, t time.Time, duration time.Duration) error {
	// req and resp bodies must not be touched because we have not enabled them, and doing so might affect the request
	if err != nil {
		// possible future enhancement: bulk failures are MUCH more relevant than query failures like timeouts.
		// this can probably be figured out by checking the request URL, if the volume proves too high.
		c.logger.Error(
			"opensearch request failed",
			tag.Error(err),
			tag.Dynamic("request_uri", request.URL.String()),
			tag.Dynamic("request_method", request.Method),
			tag.Dynamic("response_code", h.StatusCode),
			tag.Duration(duration),
		)
	}
	return nil
}

func (c convertLogger) RequestBodyEnabled() bool  { return false }
func (c convertLogger) ResponseBodyEnabled() bool { return false }

// NewClient returns a new implementation of GenericClient
func NewClient(
	connectConfig *config.ElasticSearchConfig,
	logger log.Logger,
	tlsClient *http.Client,
) (*OS2, error) {

	osconfig := osapi.Config{
		Client: opensearch.Config{
			Addresses:    []string{connectConfig.URL.String()},
			MaxRetries:   5,
			RetryBackoff: func(i int) time.Duration { return time.Duration(i) * 100 * time.Millisecond },
			Logger:       &convertLogger{logger: logger},
		},
	}

	if len(connectConfig.CustomHeaders) > 0 {
		osconfig.Client.Header = http.Header{}

		for key, value := range connectConfig.CustomHeaders {
			osconfig.Client.Header.Set(key, value)
		}
	}

	// DiscoverNodesOnStart is false by default. Turn it on only when disable sniff is set to False in ES config
	if !connectConfig.DisableSniff {
		osconfig.Client.DiscoverNodesOnStart = true
	}

	if connectConfig.AWSSigning.Enable {
		credentials, region, err := connectConfig.AWSSigning.GetCredentials()
		if err != nil {
			return nil, fmt.Errorf("getting aws credentials: %w", err)
		}

		sessionOptions := session.Options{
			Config: aws.Config{
				Region:      region,
				Credentials: credentials,
			},
		}

		signer, err := requestsigner.NewSigner(sessionOptions)
		if err != nil {
			return nil, fmt.Errorf("creating aws signer: %w", err)
		}

		osconfig.Client.Signer = signer
	}

	if tlsClient != nil {
		osconfig.Client.Transport = tlsClient.Transport
		logger.Info("Using TLS client")
	}

	osClient, err := osapi.NewClient(osconfig)

	if err != nil {
		return nil, fmt.Errorf("creating OpenSearch client: %w", err)
	}

	// initial health check
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := osClient.Ping(ctx, nil /*PingReq*/)

	if err != nil {
		return nil, fmt.Errorf("OpenSearch client unable to ping: %w", err)
	}

	if resp.IsError() {
		return nil, fmt.Errorf("OpenSearch client received error on ping: %s", resp)
	}

	return &OS2{
		client:  osClient,
		logger:  logger,
		decoder: &NumberDecoder{},
	}, nil
}

func (c *OS2) IsNotFoundError(err error) bool {
	var clientErr *osError
	if errors.As(err, &clientErr) {
		return clientErr.Status == http.StatusNotFound
	}

	return false
}

func (c *OS2) PutMapping(ctx context.Context, index, body string) error {

	req := osapi.MappingPutReq{
		Indices: []string{index},
		Body:    strings.NewReader(body),
	}

	_, err := c.client.Indices.Mapping.Put(ctx, req)
	if err != nil {
		return fmt.Errorf("OpenSearch PutMapping: %w", err)
	}

	return nil
}

func (c *OS2) CreateIndex(ctx context.Context, index string) error {
	req := osapi.IndicesCreateReq{
		Index: index,
	}

	_, err := c.client.Indices.Create(ctx, req)

	if err != nil {
		return err
	}
	return nil
}

func (c *OS2) Count(ctx context.Context, index, query string) (int64, error) {

	req := &osapi.IndicesCountReq{
		Indices: []string{index},
		Body:    strings.NewReader(query),
	}
	resp, err := c.client.Indices.Count(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("OpenSearch Count: %w", err)
	}

	return int64(resp.Count), nil
}

func (c *OS2) ClearScroll(ctx context.Context, scrollID string) error {
	_, err := c.client.Scroll.Delete(ctx, osapi.ScrollDeleteReq{
		ScrollIDs: []string{scrollID},
	})
	if err != nil {
		return fmt.Errorf("OpenSearch ClearScroll: %w", err)
	}
	return nil
}

func (c *OS2) Scroll(ctx context.Context, index, body, scrollID string) (*client.Response, error) {

	var scrollResp *osapi.ScrollGetResp
	var searchResp *osapi.SearchResp
	var err error
	// handle scroll id get call
	if len(scrollID) != 0 {
		scrollResp, err = c.client.Scroll.Get(ctx, osapi.ScrollGetReq{
			ScrollID: scrollID,
			Params: osapi.ScrollGetParams{
				Scroll: time.Minute,
				// do not set scroll ID here as it will be added to the params and scroll ID can be excessively long
			},
		})
		if err != nil {
			return nil, fmt.Errorf("opensearch Scroll get error: %w", err)
		}

		resp := &client.Response{
			TookInMillis: int64(scrollResp.Took),
			TotalHits:    int64(scrollResp.Hits.Total.Value),
			Hits: &client.SearchHits{
				TotalHits: &client.TotalHits{
					Value: int64(scrollResp.Hits.Total.Value),
				},
				Hits: osHitsToSearchHits(scrollResp.Hits.Hits),
			},
			ScrollID: *scrollResp.ScrollID,
		}
		// no more hits
		if scrollResp == nil || scrollResp.Hits.Hits == nil || len(scrollResp.Hits.Hits) == 0 {
			return resp, io.EOF
		}
		return resp, nil
	}

	// when scrollID is not passed, it is normal search request
	searchResp, err = c.client.Search(ctx, &osapi.SearchReq{
		Indices: []string{index},
		Body:    strings.NewReader(body),
		Params: osapi.SearchParams{
			Scroll: time.Minute,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("opensearch scroll search error: %w", err)
	}

	var aggRes map[string]json.RawMessage
	if searchResp.Aggregations != nil {
		err := json.Unmarshal(searchResp.Aggregations, &aggRes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal searchResp.Aggregations: %w", err)
		}
	}

	resp := &client.Response{
		TookInMillis: int64(searchResp.Took),
		TotalHits:    int64(searchResp.Hits.Total.Value),
		Hits: &client.SearchHits{
			TotalHits: &client.TotalHits{
				Value: int64(searchResp.Hits.Total.Value),
			},
			Hits: osHitsToSearchHits(searchResp.Hits.Hits),
		},
		Aggregations: aggRes,
		ScrollID:     *searchResp.ScrollID,
	}

	if searchResp.Hits.Hits == nil || len(searchResp.Hits.Hits) == 0 {
		return resp, io.EOF
	}

	return resp, nil
}

func (c *OS2) Search(ctx context.Context, index, body string) (*client.Response, error) {

	resp, err := c.client.Search(ctx, &osapi.SearchReq{
		Indices: []string{index},
		Body:    strings.NewReader(body),
	})

	if err != nil {
		return nil, fmt.Errorf("OpenSearch Search: %w", err)
	}

	var sort []interface{}
	var aggRes map[string]json.RawMessage
	var totalHits int64

	if resp.Aggregations != nil {
		err := json.Unmarshal(resp.Aggregations, &aggRes)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal resp.Aggregations during opensearch Search: %w", err)
		}
	}

	if len(resp.Hits.Hits) > 0 {
		totalHits = int64(resp.Hits.Total.Value)
		for _, sh := range resp.Hits.Hits {
			sort = sh.Sort
		}
	}
	searchHits := osHitsToSearchHits(resp.Hits.Hits)

	return &client.Response{
		TookInMillis: int64(resp.Took),
		TotalHits:    totalHits,
		Hits: &client.SearchHits{
			Hits: searchHits,
		},
		Aggregations: aggRes,
		Sort:         sort,
	}, nil
}

func (e *osError) Error() string {
	return fmt.Sprintf("Status code: %d, Type: %s, Reason: %s", e.Status, e.Details.Type, e.Details.Reason)
}

func osHitsToSearchHits(osSearchHits []osapi.SearchHit) []*client.SearchHit {

	var hits []*client.SearchHit
	if len(osSearchHits) == 0 {
		return hits
	}
	for _, h := range osSearchHits {
		hits = append(hits, &client.SearchHit{
			Source: h.Source,
		},
		)
	}
	return hits
}
