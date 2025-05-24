// Copyright (c) 2020 Uber Technologies, Inc.
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

package v7

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/olivere/elastic/v7"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/elasticsearch/client"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	// ElasticV7 implements ES7
	ElasticV7 struct {
		client *elastic.Client
		logger log.Logger
	}
	convertlogger func(msg string, tags ...tag.Tag)
)

var _ elastic.Logger = (convertlogger)(nil)

func (c convertlogger) Printf(format string, v ...interface{}) {
	c(fmt.Sprintf(format, v...))
}

// NewV7Client returns a new implementation of GenericClient
func NewV7Client(
	connectConfig *config.ElasticSearchConfig,
	logger log.Logger,
	tlsClient *http.Client,
	awsSigningClient *http.Client,
) (*ElasticV7, error) {
	clientOptFuncs := []elastic.ClientOptionFunc{
		elastic.SetURL(connectConfig.URL.String()),
		elastic.SetRetrier(elastic.NewBackoffRetrier(elastic.NewExponentialBackoff(128*time.Millisecond, 513*time.Millisecond))),
		elastic.SetDecoder(&elastic.NumberDecoder{}), // critical to ensure decode of int64 won't lose precise
		// elastic.SetInfoLog(convertlogger(logger.Info)),   // logs start/stop of components, and every successful request's code and timing.  probably too noisy.
		elastic.SetErrorLog(convertlogger(logger.Error)), // logs errors when they occur, sounds likely not noisy
	}
	if connectConfig.DisableSniff {
		clientOptFuncs = append(clientOptFuncs, elastic.SetSniff(false))
	}
	if connectConfig.DisableHealthCheck {
		clientOptFuncs = append(clientOptFuncs, elastic.SetHealthcheck(false))
	}

	if awsSigningClient != nil {
		clientOptFuncs = append(clientOptFuncs, elastic.SetHttpClient(awsSigningClient))
	}

	if tlsClient != nil {
		clientOptFuncs = append(clientOptFuncs, elastic.SetHttpClient(tlsClient))
	}

	client, err := elastic.NewClient(clientOptFuncs...)
	if err != nil {
		return nil, err
	}

	return &ElasticV7{
		client: client,
		logger: logger,
	}, nil
}

func (c *ElasticV7) IsNotFoundError(err error) bool {
	return elastic.IsNotFound(err)
}

func (c *ElasticV7) PutMapping(ctx context.Context, index, body string) error {
	_, err := c.client.PutMapping().Index(index).BodyString(body).Do(ctx)
	return err
}
func (c *ElasticV7) CreateIndex(ctx context.Context, index string) error {
	_, err := c.client.CreateIndex(index).Do(ctx)
	return err
}
func (c *ElasticV7) Count(ctx context.Context, index, query string) (int64, error) {
	return c.client.Count(index).BodyString(query).Do(ctx)
}

func (c *ElasticV7) ClearScroll(ctx context.Context, scrollID string) error {
	return elastic.NewScrollService(c.client).ScrollId(scrollID).Clear(ctx)
}

func (c *ElasticV7) Search(ctx context.Context, index string, body string) (*client.Response, error) {
	esResult, err := c.client.Search(index).Source(body).Do(ctx)
	if err != nil {
		return nil, err
	}

	if esResult.Error != nil {
		return nil, types.InternalServiceError{
			Message: fmt.Sprintf("ElasticSearch Error: %#v", esResult.Error),
		}
	} else if esResult.TimedOut {
		return nil, types.InternalServiceError{
			Message: fmt.Sprintf("ElasticSearch Error: Request timed out: %v ms", esResult.TookInMillis),
		}
	}

	var sort []interface{}
	var hits []*client.SearchHit

	if esResult != nil && esResult.Hits != nil {
		for _, h := range esResult.Hits.Hits {
			hits = append(hits, &client.SearchHit{Source: h.Source})
			sort = h.Sort
		}
	}

	return &client.Response{
		TookInMillis: esResult.TookInMillis,
		TotalHits:    esResult.TotalHits(),
		Hits:         &client.SearchHits{Hits: hits},
		Aggregations: esResult.Aggregations,
		Sort:         sort,
	}, nil

}

func (c *ElasticV7) Scroll(ctx context.Context, index, body, scrollID string) (*client.Response, error) {
	scrollService := elastic.NewScrollService(c.client)
	var esResult *elastic.SearchResult
	var err error

	// we are not returning error immediately here, as result + error combination is possible
	if len(scrollID) == 0 {
		esResult, err = scrollService.Index(index).Body(body).Do(ctx)
	} else {
		esResult, err = scrollService.ScrollId(scrollID).Do(ctx)
	}

	if esResult == nil {
		return nil, err
	}

	var hits []*client.SearchHit
	if esResult.Hits != nil {
		for _, h := range esResult.Hits.Hits {
			hits = append(hits, &client.SearchHit{Source: h.Source})
		}
	}

	return &client.Response{
		TookInMillis: esResult.TookInMillis,
		TotalHits:    esResult.TotalHits(),
		Hits:         &client.SearchHits{Hits: hits},
		Aggregations: esResult.Aggregations,
		ScrollID:     esResult.ScrollId,
	}, err
}
