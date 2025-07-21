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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/opensearch-project/opensearch-go/v2"
	osapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
)

type MockTransport struct{}

func (m *MockTransport) Perform(req *http.Request) (*http.Response, error) {
	// Simulate a network or connection error
	return nil, fmt.Errorf("forced connection error")
}

func TestNewClient(t *testing.T) {
	logger := testlogger.New(t)
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{ "status": "green" }`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer testServer.Close()
	url, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse bad URL: %v", err)
	}
	badURL, err := url.Parse("http://nonexistent.elasticsearch.server:9200")
	if err != nil {
		t.Fatalf("Failed to parse bad URL: %v", err)
	}
	tests := []struct {
		name        string
		config      *config.ElasticSearchConfig
		handlerFunc http.HandlerFunc
		expectedErr bool
	}{
		{
			name: "without aws signing config",
			config: &config.ElasticSearchConfig{
				URL:          *url,
				DisableSniff: false,
				CustomHeaders: map[string]string{
					"key": "value",
				},
			},
			expectedErr: false,
		},
		{
			name: "with wrong aws signing config",
			config: &config.ElasticSearchConfig{
				URL:          *badURL,
				DisableSniff: false,
				AWSSigning: config.AWSSigning{
					Enable: true,
				},
			},
			expectedErr: true, //will fail to ping os sever
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, logger, testServer.Client())

			if !tt.expectedErr {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestCreateIndex(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.HandlerFunc
		expectErr bool
		secure    bool
	}{
		{
			name: "normal case",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PUT" && r.URL.Path == "/test-index" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"acknowledged": true, "shards_acknowledged": true, "index": "test-index"}`))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}),
			expectErr: false,
			secure:    true,
		},
		{
			name: "error case",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			}),
			expectErr: true,
			secure:    true,
		},
		{
			name: "not valid config",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.NotFound(w, r)
			}),
			expectErr: true,
			secure:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os2Client, testServer := getSecureMockOS2Client(t, tt.handler, tt.secure)
			defer testServer.Close()

			err := os2Client.CreateIndex(context.Background(), "test-index")
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOSError(t *testing.T) {
	tests := []struct {
		name          string
		givenError    osError
		expectedError string
	}{
		{
			name: "document missing error",
			givenError: osError{
				Status: 404,
				Details: &errorDetails{
					Type:   "document_missing_exception",
					Reason: "document missing [doc-id]",
				},
			},
			expectedError: "Status code: 404, Type: document_missing_exception, Reason: document missing [doc-id]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualError := tt.givenError.Error()
			assert.Equal(t, tt.expectedError, actualError, "The formatted error message did not match the expected value")
		})
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "Invalid decoder",
			responseBody:   `{"error": "index_not_found_exception"}`,
			expectError:    true,
			expectedErrMsg: "index_not_found_exception",
		},
		{
			name: "valid error response",
			responseBody: `{
				"status": 404,
				"error": {
					"type": "index_not_found_exception",
					"reason": "index_not_found_exception: no such index",
					"index": "test-index"
				}
			}`,
			expectError:    false,
			expectedErrMsg: "index_not_found_exception: no such index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := osapi.Response{
				StatusCode: 400,
				Body:       io.NopCloser(bytes.NewBufferString(tt.responseBody)),
			}

			os2Client, testServer := getSecureMockOS2Client(t, nil, false)
			defer testServer.Close()

			err := os2Client.parseError(&response)

			if !tt.expectError {
				if parsedErr, ok := err.(*osError); ok && parsedErr.Details != nil {
					assert.Equal(t, tt.expectedErrMsg, parsedErr.Details.Reason, "Error message mismatch for case: %s", tt.name)
				} else {
					t.Errorf("Failed to assert error reason for case: %s", tt.name)
				}
			} else {
				assert.Error(t, err, "Expected an error for case: %s", tt.name)
			}
		})
	}
}

func getSecureMockOS2Client(t *testing.T, handler http.HandlerFunc, secure bool) (*OS2, *httptest.Server) {
	testServer := httptest.NewTLSServer(handler)
	osConfig := opensearch.Config{
		Addresses: []string{testServer.URL},
	}

	if secure {
		osConfig.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	client, err := opensearch.NewClient(osConfig)
	if err != nil {
		t.Fatalf("Failed to create open search client: %v", err)
	}
	mockClient := &OS2{
		client:  client,
		logger:  testlogger.New(t),
		decoder: &NumberDecoder{},
	}
	assert.NoError(t, err)
	return mockClient, testServer
}

func TestCloseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test response body"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request to test server: %v", err)
	}

	osResponse := &osapi.Response{
		StatusCode: resp.StatusCode,
		Body:       resp.Body,
		Header:     resp.Header,
	}

	// Assert that the response body is open before calling closeBody
	_, err = osResponse.Body.Read(make([]byte, 1))
	assert.NoError(t, err, "Expected response body to be open before calling closeBody")

	closeBody(osResponse)

	// Attempt to read from the body again should result in an error because it's closed
	_, err = osResponse.Body.Read(make([]byte, 1))
	assert.Error(t, err, "Expected response body to be closed after calling closeBody")
}

func TestPutMapping(t *testing.T) {
	testCases := []struct {
		name        string
		handler     http.HandlerFunc
		index       string
		body        string
		expectedErr bool
	}{
		{
			name: "Successful PutMapping",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			index:       "testIndex",
			body:        `{"properties": {"field": {"type": "text"}}}`,
			expectedErr: false,
		},
		{
			name: "Failed PutMapping",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			index:       "nonExistentIndex",
			body:        `{"properties": {"field": {"type": "text"}}}`,
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os2Client, testServer := getSecureMockOS2Client(t, tc.handler, true)
			defer testServer.Close()

			err := os2Client.PutMapping(context.Background(), tc.index, tc.body)

			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPutMappingError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	os2Client, testServer := getSecureMockOS2Client(t, http.HandlerFunc(handler), true)
	defer testServer.Close()
	os2Client.client.Transport = &MockTransport{}
	err := os2Client.PutMapping(context.Background(), "testIndex", `{"properties": {"field": {"type": "text"}}}`)
	assert.Error(t, err)
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
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  map[string]interface{}{},
					"status": 404,
				})
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
			os2Client, testServer := getSecureMockOS2Client(t, tc.handler, true)
			defer testServer.Close()
			err := os2Client.CreateIndex(context.Background(), "testIndex")
			res := os2Client.IsNotFoundError(err)
			assert.Equal(t, tc.expected, res)
		})
	}
}

func TestCount(t *testing.T) {
	testCases := []struct {
		name          string
		handler       http.HandlerFunc
		index         string
		query         string
		expectedCount int64
		expectError   bool
	}{
		{
			name: "Successful Count",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"count": 42}`)
			},
			index:         "testIndex",
			query:         "{}",
			expectedCount: 42,
			expectError:   false,
		},
		{
			name: "OpenSearch Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, `{"error": "Internal Server Error"}`)
			},
			index:       "testIndex",
			query:       "{}",
			expectError: true,
		},
		{
			name: "Decoding Error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{"count": "should be an int64"}`)
			},
			index:       "testIndex",
			query:       "{}",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os2Client, testServer := getSecureMockOS2Client(t, tc.handler, true)
			defer testServer.Close()

			count, err := os2Client.Count(context.Background(), tc.index, tc.query)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCount, count)
			}
		})
	}
}

func TestScroll(t *testing.T) {
	testCases := []struct {
		name             string
		scrollID         string
		handler          http.HandlerFunc
		expectError      bool
		expectedScrollID string // Add more fields as needed for assertions
	}{
		{
			name:     "Initial Search Request",
			scrollID: "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"_scroll_id": "scrollID123", "took": 10, "hits": {"total": {"value": 2}, "hits": [{"_source": {"field1": "value1"}}]}}`)
			},
			expectError:      false,
			expectedScrollID: "scrollID123",
		},
		{
			name:     "Subsequent Scroll Request",
			scrollID: "existingScrollID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"_scroll_id": "scrollID456", "took": 5, "hits": {"total": {"value": 1}, "hits": [{"_source": {"field2": "value2"}}]}}`)
			},
			expectError:      false,
			expectedScrollID: "scrollID456",
		},
		{
			name:     "Error Response",
			scrollID: "",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, `{"error": "Internal Server Error"}`)
			},
			expectError: true,
		},
		{
			name:     "No More Hits",
			scrollID: "someScrollID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"_scroll_id": "scrollIDNoHits", "took": 5, "hits": {"hits": []}}`)
			},
			expectError:      false,
			expectedScrollID: "scrollIDNoHits",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os2Client, testServer := getSecureMockOS2Client(t, tc.handler, true)
			defer testServer.Close()

			resp, err := os2Client.Scroll(context.Background(), "testIndex", "{}", tc.scrollID)

			if tc.expectError {
				assert.Error(t, err)
			} else if tc.name == "No More Hits" {
				assert.Equal(t, io.EOF, err, "Expected io.EOF error for no more hits")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tc.expectedScrollID, resp.ScrollID)
			}
		})
	}
}

func TestClearScroll(t *testing.T) {
	testCases := []struct {
		name          string
		scrollID      string
		handler       http.HandlerFunc
		expectedError bool
	}{
		{
			name:     "Successful Scroll Clear",
			scrollID: "testScrollID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintln(w, `{}`)
			},
			expectedError: false,
		},
		{
			name:     "OpenSearch Server Error",
			scrollID: "testScrollID",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintln(w, `{"error": {"root_cause": [{"type": "internal_server_error","reason": "Internal server error"}],"type": "internal_server_error","reason": "Internal server error"}}`)
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os2Client, testServer := getSecureMockOS2Client(t, tc.handler, true)
			defer testServer.Close()

			err := os2Client.ClearScroll(context.Background(), tc.scrollID)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSearch(t *testing.T) {
	testCases := []struct {
		name          string
		index         string
		body          string
		handler       http.HandlerFunc
		expectedError bool
		expectedHits  int
	}{
		{
			name:  "Successful Search",
			index: "testIndex",
			body:  `{"query": {"match_all": {}}}`,
			handler: func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"took": 10, "hits": {"total": {"value": 2}, "hits": [{"_source": {"field": "value"}}, {"_source": {"field": "another value"}}]}}`)
			},
			expectedError: false,
			expectedHits:  2,
		},
		{
			name:  "OpenSearch Error",
			index: "testIndex",
			body:  `{"query": {"match_all": {}}}`,
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintln(w, `{"error": "Bad request"}`)
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			os2Client, testServer := getSecureMockOS2Client(t, tc.handler, true)
			defer testServer.Close()

			resp, err := os2Client.Search(context.Background(), tc.index, tc.body)

			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Len(t, resp.Hits.Hits, tc.expectedHits)
			}
		})
	}
}
