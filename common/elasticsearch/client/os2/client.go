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
	"github.com/opensearch-project/opensearch-go/v2"
	osapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v2/opensearchtransport"
	requestsigner "github.com/opensearch-project/opensearch-go/v2/signer/aws"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	// OS2 implements Client
	OS2 struct {
		client  *opensearch.Client
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

	osconfig := opensearch.Config{
		Addresses:    []string{connectConfig.URL.String()},
		MaxRetries:   5,
		RetryBackoff: func(i int) time.Duration { return time.Duration(i) * 100 * time.Millisecond },
		Logger:       &convertLogger{logger: logger},
	}

	if len(connectConfig.CustomHeaders) > 0 {
		osconfig.Header = http.Header{}

		for key, value := range connectConfig.CustomHeaders {
			osconfig.Header.Set(key, value)
		}
	}

	// DiscoverNodesOnStart is false by default. Turn it on only when disable sniff is set to False in ES config
	if !connectConfig.DisableSniff {
		osconfig.DiscoverNodesOnStart = true
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

		osconfig.Signer = signer
	}

	if tlsClient != nil {
		osconfig.Transport = tlsClient.Transport
		logger.Info("Using TLS client")
	}

	osClient, err := opensearch.NewClient(osconfig)

	if err != nil {
		return nil, fmt.Errorf("creating OpenSearch client: %w", err)
	}

	// initial health check
	resp, err := osClient.Ping()

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

	req := osapi.IndicesPutMappingRequest{
		Index: []string{index},
		Body:  strings.NewReader(body),
	}

	resp, err := req.Do(ctx, c.client)
	if err != nil {
		return fmt.Errorf("OpenSearch PutMapping: %w", err)
	}

	defer closeBody(resp)

	if resp.IsError() {
		return c.parseError(resp)
	}

	return nil
}

func (c *OS2) CreateIndex(ctx context.Context, index string) error {
	req := osapi.IndicesCreateRequest{
		Index: index,
	}

	resp, err := req.Do(ctx, c.client)

	if err != nil {
		return fmt.Errorf("OpenSearch CreateIndex: %w", err)
	}

	defer closeBody(resp)

	if resp.IsError() {
		return c.parseError(resp)
	}

	return nil
}

func (c *OS2) Count(ctx context.Context, index, query string) (int64, error) {

	resp, err := c.client.Count(c.client.Count.WithIndex(index), c.client.Count.WithBody(strings.NewReader(query)))

	if err != nil {
		return 0, fmt.Errorf("OpenSearch Count: %w", err)
	}

	defer closeBody(resp)
	if resp.IsError() {
		return 0, c.parseError(resp)
	}

	type CountResponse struct {
		Count int64 `json:"count"`
	}

	count := &CountResponse{}
	if err := c.decoder.Decode(resp.Body, count); err != nil {
		return 0, fmt.Errorf("decoding Opensearch Count result to int64: %w", err)
	}

	return count.Count, nil
}

func (c *OS2) ClearScroll(ctx context.Context, scrollID string) error {
	resp, err := c.client.ClearScroll(
		c.client.ClearScroll.WithContext(ctx),
		c.client.ClearScroll.WithScrollID(scrollID))

	if err != nil {
		return fmt.Errorf("OpenSearch ClearScroll: %w", err)
	}

	defer closeBody(resp)
	if resp.IsError() {
		return c.parseError(resp)
	}

	return nil
}

func (c *OS2) Scroll(ctx context.Context, index, body, scrollID string) (*client.Response, error) {

	var resp *osapi.Response
	var err error
	if len(scrollID) != 0 {
		resp, err = c.client.Scroll(
			c.client.Scroll.WithScrollID(scrollID),
			c.client.Scroll.WithScroll(time.Minute),
			c.client.Scroll.WithContext(ctx),
		)
	} else {
		// when scrollID is not passed, it is normal search request
		resp, err = c.client.Search(
			c.client.Search.WithIndex(index),
			c.client.Search.WithBody(strings.NewReader(body)),
			c.client.Search.WithScroll(time.Minute),
			c.client.Search.WithContext(ctx),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("OpenSearch Scroll: %w", err)
	}

	defer closeBody(resp)

	if resp.IsError() {
		return nil, c.parseError(resp)
	}

	var osResponse response
	var totalHits int64

	if err := c.decoder.Decode(resp.Body, &osResponse); err != nil {
		return nil, fmt.Errorf("decoding OpenSearch result to Response: %w", err)
	}
	var hits []*client.SearchHit
	// no more hits
	if osResponse.Hits == nil || len(osResponse.Hits.Hits) == 0 {
		return &client.Response{
			ScrollID:     osResponse.ScrollID,
			TookInMillis: osResponse.TookInMillis,
			Hits:         &client.SearchHits{Hits: hits},
		}, io.EOF
	}

	for _, h := range osResponse.Hits.Hits {
		hits = append(hits, &client.SearchHit{Source: h.Source})
	}

	if osResponse.Hits.TotalHits != nil {
		totalHits = osResponse.Hits.TotalHits.Value
	}

	return &client.Response{
		TookInMillis: osResponse.TookInMillis,
		TotalHits:    totalHits,
		Hits:         &client.SearchHits{Hits: hits},
		Aggregations: osResponse.Aggregations,
		ScrollID:     osResponse.ScrollID,
	}, nil

}

func (c *OS2) Search(ctx context.Context, index, body string) (*client.Response, error) {
	resp, err := c.client.Search(
		c.client.Search.WithContext(ctx),
		c.client.Search.WithIndex(index),
		c.client.Search.WithBody(strings.NewReader(body)),
	)

	if err != nil {
		return nil, fmt.Errorf("OpenSearch Search: %w", err)
	}

	defer closeBody(resp)
	if resp.IsError() {
		return nil, types.InternalServiceError{
			Message: fmt.Sprintf("OpenSearch Search Error: %v", c.parseError(resp)),
		}
	}

	var osResponse response
	if err := c.decoder.Decode(resp.Body, &osResponse); err != nil {
		return nil, fmt.Errorf("decoding Opensearch result to Response: %w", err)
	}

	var hits []*client.SearchHit
	var sort []interface{}
	var totalHits int64

	if osResponse.Hits != nil && osResponse.Hits.TotalHits != nil {
		totalHits = osResponse.Hits.TotalHits.Value
		for _, h := range osResponse.Hits.Hits {
			sort = h.Sort
			hits = append(hits, &client.SearchHit{Source: h.Source})
		}
	}

	return &client.Response{
		TookInMillis: osResponse.TookInMillis,
		TotalHits:    totalHits,
		Hits:         &client.SearchHits{Hits: hits},
		Aggregations: osResponse.Aggregations,
		Sort:         sort,
	}, nil
}

func (e *osError) Error() string {
	return fmt.Sprintf("Status code: %d, Type: %s, Reason: %s", e.Status, e.Details.Type, e.Details.Reason)
}

func (c *OS2) parseError(response *osapi.Response) error {
	var e osError
	if err := c.decoder.Decode(response.Body, &e); err != nil {
		return err
	}

	return &e
}

func closeBody(response *osapi.Response) {
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
}
