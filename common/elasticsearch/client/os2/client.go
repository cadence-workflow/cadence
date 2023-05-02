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
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/opensearch-project/opensearch-go/v2"
	osapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	requestsigner "github.com/opensearch-project/opensearch-go/v2/signer/aws"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

type (
	// OS2 implements Client
	OS2 struct {
		client  *opensearch.Client
		logger  log.Logger
		decoder *NumberDecoder
	}

	Error struct {
		Status  int           `json:"status"`
		Details *ErrorDetails `json:"error,omitempty"`
	}

	ErrorDetails struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
		Index  string `json:"index,omitempty"`
	}
)

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

	return &OS2{
		client:  osClient,
		logger:  logger,
		decoder: &NumberDecoder{},
	}, nil
}

func (c *OS2) IsNotFoundError(err error) bool {
	switch e := err.(type) {
	case *Error:
		return e.Status == http.StatusNotFound
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

	cr := client.Response{}
	if err := c.decoder.Decode(resp.Body, &cr); err != nil {
		return nil, fmt.Errorf("decoding OpenSearch result to client.Response: %w", err)
	}

	if cr.Hits == nil || len(cr.Hits.Hits) == 0 {
		return &cr, io.EOF
	}

	if cr.Hits.TotalHits != nil {
		cr.TotalHits = cr.Hits.TotalHits.Value
	}

	return &cr, nil
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
			Message: fmt.Sprintf("OpenSearch Error: %v", c.parseError(resp)),
		}
	}

	var cr client.Response
	if err := c.decoder.Decode(resp.Body, &cr); err != nil {
		return nil, fmt.Errorf("decoding Opensearch result to client.Response: %w", err)
	}

	if cr.Hits != nil && cr.Hits.TotalHits != nil {
		cr.TotalHits = cr.Hits.TotalHits.Value
	}

	return &cr, nil
}

func (e *Error) Error() string {
	return fmt.Sprintf("Status code: %d, Type: %s, Reason: %s", e.Status, e.Details.Type, e.Details.Reason)
}

func (c *OS2) parseError(response *osapi.Response) error {
	var e Error
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
