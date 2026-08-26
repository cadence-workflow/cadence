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

package v6

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/olivere/elastic"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
)

func TestNewV6Client(t *testing.T) {
	logger := testlogger.New(t)
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()
	url, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse bad URL: %v", err)
	}

	connectionConfig := &config.ElasticSearchConfig{
		URL:                *url,
		DisableSniff:       true,
		DisableHealthCheck: true,
	}
	sharedClient := testServer.Client()
	client, err := NewV6Client(connectionConfig, logger, sharedClient, sharedClient)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// failed case due to an unreachable Elasticsearch server
	badURL, err := url.Parse("http://nonexistent.elasticsearch.server:9200")
	if err != nil {
		t.Fatalf("Failed to parse bad URL: %v", err)
	}
	connectionConfig.DisableHealthCheck = false
	connectionConfig.URL = *badURL
	_, err = NewV6Client(connectionConfig, logger, nil, nil)
	assert.Error(t, err)
}

func TestCreateIndex(t *testing.T) {
	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		if r.URL.Path == "/testIndex" && r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"acknowledged": true}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	elasticV6, testServer := getMockClient(t, handler)
	defer testServer.Close()
	err := elasticV6.CreateIndex(context.Background(), "testIndex")
	assert.True(t, handlerCalled, "Expected handler to be called")
	assert.NoError(t, err)
}

func TestPutMapping(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/testIndex/_mapping/_doc" && r.Method == "PUT" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}
			defer r.Body.Close()
			var receivedMapping map[string]interface{}
			if err := json.Unmarshal(body, &receivedMapping); err != nil {
				t.Fatalf("Failed to unmarshal request body: %v", err)
			}

			// Define expected mapping structurally
			expectedMapping := map[string]interface{}{
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type": "text",
					},
					"publish_date": map[string]interface{}{
						"type": "date",
					},
				},
			}

			// Compare structurally
			if !assert.Equal(t, expectedMapping, receivedMapping) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"acknowledged": true}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	elasticV6, testServer := getMockClient(t, handler)
	defer testServer.Close()
	err := elasticV6.PutMapping(context.Background(), "testIndex", `{
        "properties": {
            "title": {
                "type": "text"
            },
            "publish_date": {
                "type": "date"
            }
        }
    }`)
	assert.NoError(t, err)
}

func TestCount(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/testIndex/_count" && r.Method == "POST" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("Failed to read request body: %v", err)
			}
			defer r.Body.Close()
			expectedQuery := `{"query":{"match":{"WorkflowID":"test-workflow-id"}}}`
			if string(body) != expectedQuery {
				t.Fatalf("Expected query %s, got %s", expectedQuery, body)
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"count": 42}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	elasticV6, testServer := getMockClient(t, handler)
	defer testServer.Close()
	count, err := elasticV6.Count(context.Background(), "testIndex", `{"query":{"match":{"WorkflowID":"test-workflow-id"}}}`)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), count)
}

func TestSearch(t *testing.T) {
	testCases := []struct {
		name      string
		query     string
		expected  map[string]interface{}
		expectErr bool
		expectAgg bool
		index     string
		handler   http.HandlerFunc
	}{
		{
			name:  "normal case",
			query: `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			index: "testIndex",
			expected: map[string]interface{}{
				"WorkflowID": "test-workflow-id",
			},
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/testIndex/_search" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"took": 5,
						"timed_out": false,
						"hits": {
							"total": 1,
							"hits": [{
								"_source": {
									"WorkflowID": "test-workflow-id"
								},
								"sort": [1]
							}]
						}
					}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
			expectErr: false,
		},
		{
			name:  "elasticsearch error",
			query: `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			index: "testIndex",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/testIndex/_search" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"error": {
							"root_cause": [
								{
									"type": "index_not_found_exception",
									"reason": "no such index",
									"resource.type": "index_or_alias",
									"resource.id": "testIndex",
									"index_uuid": "_na_",
									"index": "testIndex"
								}
							],
							"type": "index_not_found_exception",
							"reason": "no such index",
							"resource.type": "index_or_alias",
							"resource.id": "testIndex",
							"index_uuid": "_na_",
							"index": "testIndex"
						},
						"status": 404
					}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
			expectErr: true,
		},
		{
			name:      "elasticsearch timeout",
			query:     `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			index:     "testIndex",
			expectErr: true,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/testIndex/_search" {
					w.WriteHeader(http.StatusOK) // Assuming Elasticsearch returns HTTP 200 for timeouts with an indication in the body
					w.Write([]byte(`{
						"took": 30,
						"timed_out": true,
						"hits": {
							"total": 0,
							"hits": []
						}
					}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
		},
		{
			name:  "elasticsearch aggregations",
			query: `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			index: "testIndex",
			expected: map[string]interface{}{
				"WorkflowID": "test-workflow-id",
			},
			expectErr: false,
			expectAgg: true,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/testIndex/_search" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"took": 5,
						"timed_out": false,
						"hits": {
							"total": 1,
							"hits": [{
								"_source": {
									"WorkflowID": "test-workflow-id"
								}
							}]
						},
						"aggregations": {
							"sample_agg": {
								"value": 42
							}
						}
					}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
		},
		{
			name:      "elasticsearch non exist index",
			query:     `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			index:     "test_failure",
			expectErr: true,
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/testIndex/_search" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			elasticV6, testServer := getMockClient(t, tc.handler)
			defer testServer.Close()
			resp, err := elasticV6.Search(context.Background(), tc.index, tc.query)
			if !tc.expectErr {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				// Verify the response details
				assert.Equal(t, int64(5), resp.TookInMillis)
				assert.Equal(t, int64(1), resp.TotalHits)
				assert.NotNil(t, resp.Hits)
				assert.Len(t, resp.Hits.Hits, 1)

				var actual map[string]interface{}
				if err := json.Unmarshal([]byte(string(resp.Hits.Hits[0].Source)), &actual); err != nil {
					t.Fatalf("Failed to unmarshal actual JSON: %v", err)
				}
				assert.Equal(t, tc.expected, actual)

				if tc.expectAgg {
					// Verify the response includes the expected aggregations
					assert.NotNil(t, resp.Aggregations, "Aggregations should not be nil")
					assert.Contains(t, resp.Aggregations, "sample_agg", "Aggregations should contain 'sample_agg'")

					// Additional assertions can be made to verify the contents of the aggregation
					sampleAgg := resp.Aggregations["sample_agg"]
					var aggResult map[string]interface{}
					err = json.Unmarshal(sampleAgg, &aggResult)
					assert.NoError(t, err)
					assert.Equal(t, float64(42), aggResult["value"], "Aggregation 'sample_agg' should have a value of 42")
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestScroll(t *testing.T) {
	testCases := []struct {
		name      string
		query     string
		expected  map[string]interface{}
		expectErr bool
		index     string
		handler   http.HandlerFunc
		scrollID  string
	}{
		{
			name:  "normal case",
			query: `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			expected: map[string]interface{}{
				"WorkflowID": "test-workflow-id",
			},
			index: "testIndex",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/testIndex/_search" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{
						"took": 5,
						"timed_out": false,
						"hits": {
							"total": 1,
							"hits": [{
								"_source": {
									"WorkflowID": "test-workflow-id"
								}
							}]
						},
						"aggregations": {
							"sample_agg": {
								"value": 42
							}
						}
					}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
			expectErr: false,
		},
		{
			name:  "error case",
			query: `{"query":{"bool":{"must":{"match":{"WorkflowID":"test-workflow-id"}}}}}`,
			index: "testIndex",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			expectErr: true,
			scrollID:  "1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			elasticV6, testServer := getMockClient(t, tc.handler)
			defer testServer.Close()
			resp, err := elasticV6.Scroll(context.Background(), tc.index, tc.query, tc.scrollID)
			if !tc.expectErr {
				assert.NoError(t, err)
				var actualSource map[string]interface{}
				err := json.Unmarshal(resp.Hits.Hits[0].Source, &actualSource)
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actualSource)
			} else {
				assert.Error(t, err)
				assert.Nil(t, resp)
			}
		})
	}
}

func TestClearScroll(t *testing.T) {
	var handlerCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		if r.Method == "DELETE" && r.URL.Path == "/_search/scroll" {
			// Simulate a successful clear scroll response
			w.WriteHeader(http.StatusOK)
			response := `{
				"succeeded": true,
				"num_freed": 1
			}`
			w.Write([]byte(response))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})
	elasticV6, testServer := getMockClient(t, handler)
	defer testServer.Close()
	err := elasticV6.ClearScroll(context.Background(), "scrollID")
	assert.True(t, handlerCalled, "Expected handler to be called")
	assert.NoError(t, err)
}

func TestIsNotFoundError(t *testing.T) {
	testCases := []struct {
		name     string
		handler  http.HandlerFunc
		expected bool
	}{
		{
			name: "NotFound error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			}),
			expected: true,
		},
		{
			name: "Other error",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "Bad Request", http.StatusBadRequest)
			}),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			elasticV6, testServer := getMockClient(t, tc.handler)
			defer testServer.Close()
			err := elasticV6.CreateIndex(context.Background(), "testIndex")
			res := elasticV6.IsNotFoundError(err)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func getMockClient(t *testing.T, handler http.HandlerFunc) (ElasticV6, *httptest.Server) {
	testServer := httptest.NewTLSServer(handler)
	mockClient, err := elastic.NewClient(
		elastic.SetURL(testServer.URL),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(false),
		elastic.SetHttpClient(testServer.Client()))
	assert.NoError(t, err)
	return ElasticV6{
		client: mockClient,
	}, testServer
}
