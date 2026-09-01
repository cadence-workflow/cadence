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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	elastic "github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	schemaes "github.com/uber/cadence/schema/elasticsearch"
)

func TestNewV7Client(t *testing.T) {
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
	client, err := NewV7Client(connectionConfig, logger, sharedClient, sharedClient)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// failed case due to an unreachable Elasticsearch server
	badURL, err := url.Parse("http://nonexistent.elasticsearch.server:9200")
	if err != nil {
		t.Fatalf("Failed to parse bad URL: %v", err)
	}
	connectionConfig.DisableHealthCheck = false
	connectionConfig.URL = *badURL
	_, err = NewV7Client(connectionConfig, logger, nil, nil)
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
	elasticV7, testServer := getMockClient(t, handler)
	defer testServer.Close()
	err := elasticV7.CreateIndex(context.Background(), "testIndex")
	assert.True(t, handlerCalled, "Expected handler to be called")
	assert.NoError(t, err)
}

func TestPutMappings(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/testIndex/_mapping" && r.Method == "PUT" {
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
	elasticV7, testServer := getMockClient(t, handler)
	defer testServer.Close()
	err := elasticV7.PutMappings(context.Background(), "testIndex", map[string]any{
		"properties": map[string]any{
			"title": map[string]any{
				"type": "text",
			},
			"publish_date": map[string]any{
				"type": "date",
			},
		},
	})
	assert.NoError(t, err)
}

func TestIsHealthy(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"test-node","cluster_name":"test-cluster","version":{"number":"7.10.0"}}`))
		})
		elasticV7, testServer := getMockClient(t, handler)
		defer testServer.Close()

		err := elasticV7.IsHealthy(context.Background())
		assert.NoError(t, err)
	})

	t.Run("invalid ping url", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		elasticV7, testServer := getMockClient(t, handler)
		defer testServer.Close()

		elasticV7.pingURL = "http://127.0.0.1:0"
		err := elasticV7.IsHealthy(context.Background())
		assert.Error(t, err)
	})
}

func TestPutIndexTemplate(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		expectErr bool
	}{
		{name: "acknowledged", body: `{"acknowledged": true}`},
		{name: "not acknowledged", body: `{"acknowledged": false}`, expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PUT" || r.URL.Path != "/_template/test-template" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			})

			elasticV7, testServer := getMockClient(t, handler)
			defer testServer.Close()

			err := elasticV7.PutIndexTemplate(context.Background(), "test-template", []byte(`{"template":"*"}`))
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestHasIndex(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		expectExists bool
		expectErr    bool
	}{
		{name: "exists", status: http.StatusOK, expectExists: true},
		{name: "missing", status: http.StatusNotFound, expectExists: false},
		{name: "server error", status: http.StatusInternalServerError, expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "HEAD" || r.URL.Path != "/testIndex" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(tc.status)
			})

			elasticV7, testServer := getMockClient(t, handler)
			defer testServer.Close()

			exists, err := elasticV7.HasIndex(context.Background(), "testIndex")
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectExists, exists)
		})
	}
}

func TestDeleteIndex(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		expectErr bool
	}{
		{name: "acknowledged", body: `{"acknowledged": true}`},
		{name: "not acknowledged", body: `{"acknowledged": false}`, expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" || r.URL.Path != "/testIndex" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			})

			elasticV7, testServer := getMockClient(t, handler)
			defer testServer.Close()

			err := elasticV7.DeleteIndex(context.Background(), "testIndex")
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestGetMappings(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || !strings.HasPrefix(r.URL.Path, "/testIndex/_mapping") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"testIndex":{"mappings":{"properties":{"title":{"type":"text"}}}}}`))
	})

	elasticV7, testServer := getMockClient(t, handler)
	defer testServer.Close()

	mappings, err := elasticV7.GetMappings(context.Background(), "testIndex")
	assert.NoError(t, err)
	assert.Contains(t, mappings, "testIndex")
}

func TestMappingsFromTemplate(t *testing.T) {
	elasticV7 := &ElasticV7{}

	mappings, err := elasticV7.MappingsFromTemplate([]byte(`{"mappings":{"properties":{"title":{"type":"text"}}}}`))
	assert.NoError(t, err)
	assert.Contains(t, mappings, "properties")

	_, err = elasticV7.MappingsFromTemplate([]byte(`not-json`))
	assert.Error(t, err)
}

func TestLatestTemplate(t *testing.T) {
	elasticV7 := &ElasticV7{}
	assert.Equal(t, schemaes.IndexTemplateV7, elasticV7.LatestTemplate())
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
	elasticV7, testServer := getMockClient(t, handler)
	defer testServer.Close()
	count, err := elasticV7.Count(context.Background(), "testIndex", `{"query":{"match":{"WorkflowID":"test-workflow-id"}}}`)
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
			elasticV7, testServer := getMockClient(t, tc.handler)
			defer testServer.Close()
			resp, err := elasticV7.Search(context.Background(), tc.index, tc.query)
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
			elasticV7, testServer := getMockClient(t, tc.handler)
			defer testServer.Close()
			resp, err := elasticV7.Scroll(context.Background(), tc.index, tc.query, tc.scrollID)
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
	elasticV7, testServer := getMockClient(t, handler)
	defer testServer.Close()
	err := elasticV7.ClearScroll(context.Background(), "scrollID")
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
			elasticV7, testServer := getMockClient(t, tc.handler)
			defer testServer.Close()
			err := elasticV7.CreateIndex(context.Background(), "testIndex")
			res := elasticV7.IsNotFoundError(err)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func getMockClient(t *testing.T, handler http.HandlerFunc) (ElasticV7, *httptest.Server) {
	testServer := httptest.NewTLSServer(handler)
	mockClient, err := elastic.NewClient(
		elastic.SetURL(testServer.URL),
		elastic.SetSniff(false),
		elastic.SetHealthcheck(false),
		elastic.SetHttpClient(testServer.Client()))
	assert.NoError(t, err)
	return ElasticV7{
		client:  mockClient,
		pingURL: testServer.URL,
	}, testServer
}
