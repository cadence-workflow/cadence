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
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/uber/cadence/common/elasticsearch/bulk"
	"github.com/uber/cadence/common/log/testlogger"
)

func TestStart(t *testing.T) {
	ctx := context.Background()
	processor := &bulkProcessor{}
	if err := processor.Start(ctx); err != nil {
		t.Errorf("Start() error = %v, wantErr %v", err, nil)
	}
}

func TestStopAndClose(t *testing.T) {
	osClient, testServer := getSecureMockOS2Client(t, nil, false)
	defer testServer.Close()

	params := bulk.BulkProcessorParameters{
		FlushInterval: 1,
		NumOfWorkers:  1,
	}
	processor, err := osClient.RunBulkProcessor(context.Background(), &params)
	assert.NoError(t, err)

	if err := processor.Stop(); err != nil {
		t.Errorf("Stop() error = %v, wantErr %v", err, nil)
	}
}

func TestFlush(t *testing.T) {
	processor := &bulkProcessor{}
	if err := processor.Flush(); err != nil {
		t.Errorf("Flush() error = %v, wantErr %v", err, nil)
	}
}

func TestAddDeleteRequest(t *testing.T) {
	testcases := []struct {
		name      string
		input     bulk.GenericBulkableAddRequest
		expectErr bool
	}{
		{
			name: "delete request",
			input: bulk.GenericBulkableAddRequest{
				RequestType: bulk.BulkableDeleteRequest,
				Index:       "test-index",
				ID:          "test-id",
				Version:     1,
				VersionType: "external",
			},
			expectErr: false,
		},
		{
			name: "index request success",
			input: bulk.GenericBulkableAddRequest{
				RequestType: bulk.BulkableIndexRequest,
				Index:       "test-index",
				ID:          "test-id",
				Doc:         map[string]interface{}{"field": "value"},
			},
			expectErr: false,
		},
		{
			name: "create request success",
			input: bulk.GenericBulkableAddRequest{
				RequestType: bulk.BulkableCreateRequest,
				Index:       "test-index",
				ID:          "test-id",
				Doc:         map[string]interface{}{"field": "value"},
			},
			expectErr: false,
		},
		{
			name: "create request failure",
			input: bulk.GenericBulkableAddRequest{
				RequestType: bulk.BulkableCreateRequest,
				Index:       "test-index",
				ID:          "test-id",
				Doc:         func() {},
			},
			expectErr: false,
		},
		{
			name: "index request failure",
			input: bulk.GenericBulkableAddRequest{
				RequestType: bulk.BulkableIndexRequest,
				Index:       "test-index",
				ID:          "test-id",
				Doc:         func() {},
			},
			expectErr: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			osClient, testServer := getSecureMockOS2Client(t, nil, false)
			defer testServer.Close()

			params := bulk.BulkProcessorParameters{
				FlushInterval: 1,
				NumOfWorkers:  1,
			}
			processor, err := osClient.RunBulkProcessor(context.Background(), &params)
			assert.NoError(t, err)
			processor.Add(&tc.input)
			processor.Stop()
		})
	}
}

func TestProcessCallback(t *testing.T) {
	osClient, testServer := getSecureMockOS2Client(t, nil, false)
	defer testServer.Close()
	processor, _ := opensearchutil.NewBulkIndexer(opensearchutil.BulkIndexerConfig{
		Client:        osClient.client,
		FlushInterval: 1,
		NumWorkers:    1,
		Decoder:       &NumberDecoder{},
	})
	bulkProcessor := &bulkProcessor{
		processor: processor,
		before:    beforeFunc,
		after:     func(int64, []bulk.GenericBulkableRequest, *bulk.GenericBulkResponse, *bulk.GenericError) {},
	}

	callBackRequest := bulk.NewBulkIndexRequest().ID("test-id").Index("test-index").Doc(map[string]interface{}{"field": "value"})
	req := opensearchutil.BulkIndexerItem{Index: "test-index", DocumentID: "test-id"}
	res := opensearchutil.BulkIndexerResponseItem{Index: "test-index", DocumentID: "test-id", Status: 200}

	tests := []struct {
		name        string
		onSuccess   bool
		errorPassed error
		expectError bool
	}{
		{"Success Callback", true, nil, false},
		{"Failure Callback", false, errors.New("test error"), true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bulkProcessor.processCallback(0, callBackRequest, req, res, test.onSuccess, test.errorPassed)
		})
	}
}

func beforeFunc(executionID int64, requests []bulk.GenericBulkableRequest) {}

func TestRunBulkProcessor(t *testing.T) {
	osClient, testServer := getSecureMockOS2Client(t, nil, false)
	defer testServer.Close()

	testcases := []struct {
		name      string
		input     bulk.BulkProcessorParameters
		expectErr bool
	}{
		{
			name: "normal",
			input: bulk.BulkProcessorParameters{
				FlushInterval: 1,
				NumOfWorkers:  1,
			},
			expectErr: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			processor, err := osClient.RunBulkProcessor(context.Background(), &tc.input)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, processor)
			}
		})
	}
}

func TestUnmarshalFromReader(t *testing.T) {
	// test unmarshalFromReader
	testcases := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "normal",
			input:     `{"status":"ok"}`,
			expectErr: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := &NumberDecoder{}
			reader := createReaderFromString(tc.input)

			var response opensearchutil.BulkIndexerResponse
			err := decoder.UnmarshalFromReader(reader, &response)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	// test decode
	testcases := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			name:      "normal",
			input:     `{"status":"ok"}`,
			expectErr: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			decoder := &NumberDecoder{}
			reader := createReaderFromString(tc.input)

			var response opensearchutil.BulkIndexerResponse
			err := decoder.Decode(reader, &response)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Function to create a sample reader from a string
func createReaderFromString(s string) io.Reader {
	return bytes.NewBufferString(s)
}

// MockProcessor is a mock of the processor that will capture the requests.
type MockProcessor struct {
	mock.Mock
}

func (m *MockProcessor) Add(ctx context.Context, req opensearchutil.BulkIndexerItem) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockProcessor) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockProcessor) Stats() opensearchutil.BulkIndexerStats {
	args := m.Called()
	return args.Get(0).(opensearchutil.BulkIndexerStats)
}

func TestBulkProcessorPayload(t *testing.T) {
	mockProcessor := new(MockProcessor)
	logger := testlogger.New(t)
	v := &bulkProcessor{
		processor: mockProcessor,
		logger:    logger,
	}

	request := &bulk.GenericBulkableAddRequest{
		Index:       "test-index",
		ID:          "123",
		Version:     1,
		VersionType: "external",
		RequestType: bulk.BulkableIndexRequest,
		Doc: map[string]interface{}{
			"field1": "value1",
		},
	}

	expectedBody, err := json.Marshal(request.Doc)
	assert.NoError(t, err)
	expectedReq := opensearchutil.BulkIndexerItem{
		Index:       "test-index",
		DocumentID:  "123",
		Action:      "index",
		Version:     &request.Version,
		VersionType: &request.VersionType,
		Body:        bytes.NewReader(expectedBody),
	}

	mockProcessor.On("Add", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		actualReq := args.Get(1).(opensearchutil.BulkIndexerItem)

		assert.Equal(t, expectedReq.Index, actualReq.Index)
		assert.Equal(t, expectedReq.DocumentID, actualReq.DocumentID)
		assert.Equal(t, expectedReq.Action, actualReq.Action)
		assert.Equal(t, expectedReq.Version, actualReq.Version)
		assert.Equal(t, expectedReq.VersionType, actualReq.VersionType)

		var expectedDoc, actualDoc map[string]interface{}
		assert.NoError(t, json.NewDecoder(expectedReq.Body).Decode(&expectedDoc))
		assert.NoError(t, json.NewDecoder(actualReq.Body).Decode(&actualDoc))
		assert.Equal(t, expectedDoc, actualDoc)

	}).Return(nil)

	v.Add(request)
	mockProcessor.AssertExpectations(t)
}
