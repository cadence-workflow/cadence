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

package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/valyala/fastjson"

	"github.com/uber/cadence/.gen/go/indexer"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	es "github.com/uber/cadence/common/elasticsearch"
	esMocks "github.com/uber/cadence/common/elasticsearch/mocks"
	"github.com/uber/cadence/common/elasticsearch/query"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/mocks"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
)

type ESVisibilitySuite struct {
	suite.Suite
	// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
	// not merely log an error
	*require.Assertions
	visibilityStore *esVisibilityStore
	mockESClient    *esMocks.GenericClient
	mockProducer    *mocks.KafkaProducer
}

var (
	testIndex        = "test-index"
	testDomain       = "test-domain"
	testDomainID     = "bfd5c907-f899-4baf-a7b2-2ab85e623ebd"
	testPageSize     = 5
	testEarliestTime = int64(1547596872371000000)
	testLatestTime   = int64(2547596872371000000)
	testWorkflowType = "test-wf-type"
	testWorkflowID   = "test-wid"
	testCloseStatus  = int32(1)

	testRequest = &p.InternalListWorkflowExecutionsRequest{
		DomainUUID:   testDomainID,
		Domain:       testDomain,
		PageSize:     testPageSize,
		EarliestTime: time.Unix(0, testEarliestTime),
		LatestTime:   time.Unix(0, testLatestTime),
	}
	testSearchResult = &p.InternalListWorkflowExecutionsResponse{}
	errTestESSearch  = errors.New("ES error")

	testContextTimeout = 5 * time.Second

	esIndexMaxResultWindow = 3
)

type Source struct {
	Match map[string]struct {
		Query interface{} `json:"query"`
	} `json:"match"`
}

func TestESVisibilitySuite(t *testing.T) {
	suite.Run(t, new(ESVisibilitySuite))
}

func (s *ESVisibilitySuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())

	s.mockESClient = &esMocks.GenericClient{}
	config := &service.Config{
		ESIndexMaxResultWindow: dynamicproperties.GetIntPropertyFn(esIndexMaxResultWindow),
		ValidSearchAttributes:  dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
	}

	s.mockProducer = &mocks.KafkaProducer{}
	mgr := NewElasticSearchVisibilityStore(s.mockESClient, testIndex, s.mockProducer, config, testlogger.New(s.Suite.T()))
	s.visibilityStore = mgr.(*esVisibilityStore)
}

func (s *ESVisibilitySuite) TearDownTest() {
	s.mockESClient.AssertExpectations(s.T())
	s.mockProducer.AssertExpectations(s.T())
}

func (s *ESVisibilitySuite) TestRecordWorkflowExecutionStarted() {
	// test non-empty request fields match
	request := &p.InternalRecordWorkflowExecutionStartedRequest{}
	request.DomainUUID = "domainID"
	request.WorkflowID = "wid"
	request.RunID = "rid"
	request.WorkflowTypeName = "wfType"
	request.StartTimestamp = time.Unix(0, int64(123))
	request.ExecutionTimestamp = time.Unix(0, int64(321))
	request.TaskID = int64(111)
	request.IsCron = true
	request.NumClusters = 2
	request.ClusterAttributeScope = "region"
	request.ClusterAttributeName = "us-east-1"
	memoBytes := []byte(`test bytes`)
	request.Memo = p.NewDataBlob(memoBytes, constants.EncodingTypeThriftRW)
	request.ShardID = 1234

	s.mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.Message) bool {
		fields := input.Fields
		s.Equal(request.DomainUUID, input.GetDomainID())
		s.Equal(request.WorkflowID, input.GetWorkflowID())
		s.Equal(request.RunID, input.GetRunID())
		s.Equal(request.TaskID, input.GetVersion())
		s.Equal(request.WorkflowTypeName, fields[es.WorkflowType].GetStringData())
		s.Equal(request.StartTimestamp.UnixNano(), fields[es.StartTime].GetIntData())
		s.Equal(request.ExecutionTimestamp.UnixNano(), fields[es.ExecutionTime].GetIntData())
		s.Equal(memoBytes, fields[es.Memo].GetBinaryData())
		s.Equal(string(constants.EncodingTypeThriftRW), fields[es.Encoding].GetStringData())
		s.Equal(request.IsCron, fields[es.IsCron].GetBoolData())
		s.Equal((int64)(request.NumClusters), fields[es.NumClusters].GetIntData())
		s.Equal(request.ClusterAttributeScope, fields[es.ClusterAttributeScope].GetStringData())
		s.Equal(request.ClusterAttributeName, fields[es.ClusterAttributeName].GetStringData())
		s.Equal(indexer.VisibilityOperationRecordStarted, *input.VisibilityOperation)
		s.Equal((int64)(request.ShardID), fields[es.ShardID].GetIntData())
		return true
	})).Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err := s.visibilityStore.RecordWorkflowExecutionStarted(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestRecordWorkflowExecutionStarted_EmptyRequest() {
	// test empty request
	request := &p.InternalRecordWorkflowExecutionStartedRequest{
		Memo: &p.DataBlob{},
	}
	s.mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.Message) bool {
		s.Equal(indexer.MessageTypeIndex, input.GetMessageType())
		_, ok := input.Fields[es.Memo]
		s.False(ok)
		_, ok = input.Fields[es.Encoding]
		s.False(ok)
		return true
	})).Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err := s.visibilityStore.RecordWorkflowExecutionStarted(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestRecordWorkflowExecutionClosed() {
	// test non-empty request fields match
	request := &p.InternalRecordWorkflowExecutionClosedRequest{}
	request.DomainUUID = "domainID"
	request.WorkflowID = "wid"
	request.RunID = "rid"
	request.WorkflowTypeName = "wfType"
	request.StartTimestamp = time.Unix(0, int64(123))
	request.ExecutionTimestamp = time.Unix(0, int64(321))
	request.TaskID = int64(111)
	memoBytes := []byte(`test bytes`)
	request.Memo = p.NewDataBlob(memoBytes, constants.EncodingTypeThriftRW)
	request.CloseTimestamp = time.Unix(0, int64(999))
	request.Status = types.WorkflowExecutionCloseStatusTerminated
	request.HistoryLength = int64(20)
	request.IsCron = false
	request.NumClusters = 2
	request.UpdateTimestamp = time.Unix(0, int64(213))
	request.ShardID = 1234
	s.mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.Message) bool {
		fields := input.Fields
		s.Equal(request.DomainUUID, input.GetDomainID())
		s.Equal(request.WorkflowID, input.GetWorkflowID())
		s.Equal(request.RunID, input.GetRunID())
		s.Equal(request.TaskID, input.GetVersion())
		s.Equal(request.WorkflowTypeName, fields[es.WorkflowType].GetStringData())
		s.Equal(request.StartTimestamp.UnixNano(), fields[es.StartTime].GetIntData())
		s.Equal(request.ExecutionTimestamp.UnixNano(), fields[es.ExecutionTime].GetIntData())
		s.Equal(memoBytes, fields[es.Memo].GetBinaryData())
		s.Equal(string(constants.EncodingTypeThriftRW), fields[es.Encoding].GetStringData())
		s.Equal(request.CloseTimestamp.UnixNano(), fields[es.CloseTime].GetIntData())
		s.Equal(int64(request.Status), fields[es.CloseStatus].GetIntData())
		s.Equal(request.HistoryLength, fields[es.HistoryLength].GetIntData())
		s.Equal(request.IsCron, fields[es.IsCron].GetBoolData())
		s.Equal((int64)(request.NumClusters), fields[es.NumClusters].GetIntData())
		s.Equal(indexer.VisibilityOperationRecordClosed, *input.VisibilityOperation)
		s.Equal(request.UpdateTimestamp.UnixNano(), fields[es.UpdateTime].GetIntData())
		s.Equal((int64)(request.ShardID), fields[es.ShardID].GetIntData())
		return true
	})).Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err := s.visibilityStore.RecordWorkflowExecutionClosed(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestRecordWorkflowExecutionClosed_EmptyRequest() {
	// test empty request
	request := &p.InternalRecordWorkflowExecutionClosedRequest{
		Memo: &p.DataBlob{},
	}
	s.mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.Message) bool {
		s.Equal(indexer.MessageTypeIndex, input.GetMessageType())
		_, ok := input.Fields[es.Memo]
		s.False(ok)
		_, ok = input.Fields[es.Encoding]
		s.False(ok)
		return true
	})).Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err := s.visibilityStore.RecordWorkflowExecutionClosed(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestRecordWorkflowExecutionUninitialized() {
	// test non-empty request fields match
	request := &p.InternalRecordWorkflowExecutionUninitializedRequest{}
	request.DomainUUID = "domainID"
	request.WorkflowID = "wid"
	request.RunID = "rid"
	request.WorkflowTypeName = "wfType"
	request.UpdateTimestamp = time.Unix(0, int64(213))
	request.ShardID = 1234

	s.mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.Message) bool {
		fields := input.Fields
		s.Equal(request.DomainUUID, input.GetDomainID())
		s.Equal(request.WorkflowID, input.GetWorkflowID())
		s.Equal(request.RunID, input.GetRunID())
		s.Equal(request.WorkflowTypeName, fields[es.WorkflowType].GetStringData())
		s.Equal(request.UpdateTimestamp.UnixNano(), fields[es.UpdateTime].GetIntData())
		s.Equal(request.ShardID, fields[es.ShardID].GetIntData())
		return true
	})).Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err := s.visibilityStore.RecordWorkflowExecutionUninitialized(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestRecordWorkflowExecutionUninitialized_EmptyRequest() {
	// test empty request
	request := &p.InternalRecordWorkflowExecutionUninitializedRequest{}
	s.mockProducer.On("Publish", mock.Anything, mock.MatchedBy(func(input *indexer.Message) bool {
		s.Equal(indexer.MessageTypeCreate, input.GetMessageType())
		return true
	})).Return(nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	err := s.visibilityStore.RecordWorkflowExecutionUninitialized(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestListOpenWorkflowExecutions() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.True(input.IsOpen)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListOpenWorkflowExecutions(ctx, testRequest)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListOpenWorkflowExecutions(ctx, testRequest)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListOpenWorkflowExecutions failed"))
}

func (s *ESVisibilitySuite) TestListClosedWorkflowExecutions() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.False(input.IsOpen)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListClosedWorkflowExecutions(ctx, testRequest)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListClosedWorkflowExecutions(ctx, testRequest)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListClosedWorkflowExecutions failed"))
}

func (s *ESVisibilitySuite) TestListOpenWorkflowExecutionsByType() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.True(input.IsOpen)
		source, err := matchQueryData(*input.MatchQuery)
		s.NoError(err)
		s.Equal(source.Match[es.WorkflowType].Query, testWorkflowType)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.InternalListWorkflowExecutionsByTypeRequest{
		InternalListWorkflowExecutionsRequest: *testRequest,
		WorkflowTypeName:                      testWorkflowType,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListOpenWorkflowExecutionsByType(ctx, request)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListOpenWorkflowExecutionsByType(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListOpenWorkflowExecutionsByType failed"))
}

func (s *ESVisibilitySuite) TestListClosedWorkflowExecutionsByType() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.False(input.IsOpen)

		source, err := matchQueryData(*input.MatchQuery)
		s.NoError(err)

		s.Equal(source.Match[es.WorkflowType].Query, testWorkflowType)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.InternalListWorkflowExecutionsByTypeRequest{
		InternalListWorkflowExecutionsRequest: *testRequest,
		WorkflowTypeName:                      testWorkflowType,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListClosedWorkflowExecutionsByType(ctx, request)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListClosedWorkflowExecutionsByType(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListClosedWorkflowExecutionsByType failed"))
}

func (s *ESVisibilitySuite) TestListOpenWorkflowExecutionsByWorkflowID() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.True(input.IsOpen)

		source, err := matchQueryData(*input.MatchQuery)
		s.NoError(err)
		s.Equal(source.Match[es.WorkflowID].Query, testWorkflowID)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.InternalListWorkflowExecutionsByWorkflowIDRequest{
		InternalListWorkflowExecutionsRequest: *testRequest,
		WorkflowID:                            testWorkflowID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListOpenWorkflowExecutionsByWorkflowID failed"))
}

func (s *ESVisibilitySuite) TestListClosedWorkflowExecutionsByWorkflowID() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.False(input.IsOpen)

		source, err := matchQueryData(*input.MatchQuery)
		s.NoError(err)
		s.Equal(source.Match[es.WorkflowID].Query, testWorkflowID)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.InternalListWorkflowExecutionsByWorkflowIDRequest{
		InternalListWorkflowExecutionsRequest: *testRequest,
		WorkflowID:                            testWorkflowID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListClosedWorkflowExecutionsByWorkflowID failed"))
}

func (s *ESVisibilitySuite) TestListClosedWorkflowExecutionsByStatus() {
	s.mockESClient.On("Search", mock.Anything, mock.MatchedBy(func(input *es.SearchRequest) bool {
		s.False(input.IsOpen)
		source, err := matchQueryData(*input.MatchQuery)
		s.NoError(err)
		cs := (int32)(source.Match[es.CloseStatus].Query.(float64))
		s.Equal(testCloseStatus, cs)
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	closeStatus := types.WorkflowExecutionCloseStatus(testCloseStatus)
	request := &p.InternalListClosedWorkflowExecutionsByStatusRequest{
		InternalListWorkflowExecutionsRequest: *testRequest,
		Status:                                closeStatus,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListClosedWorkflowExecutionsByStatus(ctx, request)
	s.NoError(err)

	s.mockESClient.On("Search", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListClosedWorkflowExecutionsByStatus(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListClosedWorkflowExecutionsByStatus failed"))
}

func (s *ESVisibilitySuite) TestGetNextPageToken() {
	token, err := es.GetNextPageToken([]byte{})
	s.Equal(0, token.From)
	s.NoError(err)

	from := 5
	input, err := es.SerializePageToken(&es.ElasticVisibilityPageToken{From: from})
	s.NoError(err)
	token, err = es.GetNextPageToken(input)
	s.Equal(from, token.From)
	s.NoError(err)

	badInput := []byte("bad input")
	token, err = es.GetNextPageToken(badInput)
	s.Nil(token)
	s.Error(err)
}

func (s *ESVisibilitySuite) TestDeserializePageToken() {
	token := &es.ElasticVisibilityPageToken{From: 0}
	data, _ := es.SerializePageToken(token)
	result, err := es.DeserializePageToken(data)
	s.NoError(err)
	s.Equal(token, result)

	badInput := []byte("bad input")
	result, err = es.DeserializePageToken(badInput)
	s.Error(err)
	s.Nil(result)
	err, ok := err.(*types.BadRequestError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "unable to deserialize page token"))

	token = &es.ElasticVisibilityPageToken{SortValue: int64(64), TieBreaker: "unique"}
	data, _ = es.SerializePageToken(token)
	result, err = es.DeserializePageToken(data)
	s.NoError(err)
	resultSortValue, err := result.SortValue.(json.Number).Int64()
	s.NoError(err)
	s.Equal(token.SortValue.(int64), resultSortValue)
}

func (s *ESVisibilitySuite) TestSerializePageToken() {
	data, err := es.SerializePageToken(nil)
	s.NoError(err)
	s.True(len(data) > 0)
	token, err := es.DeserializePageToken(data)
	s.NoError(err)
	s.Equal(0, token.From)
	s.Equal(nil, token.SortValue)
	s.Equal("", token.TieBreaker)

	newToken := &es.ElasticVisibilityPageToken{From: 5}
	data, err = es.SerializePageToken(newToken)
	s.NoError(err)
	s.True(len(data) > 0)
	token, err = es.DeserializePageToken(data)
	s.NoError(err)
	s.Equal(newToken, token)

	sortTime := int64(123)
	tieBreaker := "unique"
	newToken = &es.ElasticVisibilityPageToken{SortValue: sortTime, TieBreaker: tieBreaker}
	data, err = es.SerializePageToken(newToken)
	s.NoError(err)
	s.True(len(data) > 0)
	token, err = es.DeserializePageToken(data)
	s.NoError(err)
	resultSortValue, err := token.SortValue.(json.Number).Int64()
	s.NoError(err)
	s.Equal(newToken.SortValue, resultSortValue)
	s.Equal(newToken.TieBreaker, token.TieBreaker)
}

// Move to client_v6_test
// func (s *ESVisibilitySuite) TestConvertSearchResultToVisibilityRecord() {
//	data := []byte(`{"CloseStatus": 0,
//          "CloseTime": 1547596872817380000,
//          "DomainID": "bfd5c907-f899-4baf-a7b2-2ab85e623ebd",
//          "HistoryLength": 29,
//          "KafkaKey": "7-619",
//          "RunID": "e481009e-14b3-45ae-91af-dce6e2a88365",
//          "StartTime": 1547596872371000000,
//          "WorkflowID": "6bfbc1e5-6ce4-4e22-bbfb-e0faa9a7a604-1-2256",
//          "WorkflowType": "TestWorkflowExecute"}`)
//	source := (*json.RawMessage)(&data)
//	searchHit := &elastic.SearchHit{
//		Source: source,
//	}
//
//	// test for open
//	info := s.visibilityStore.convertSearchResultToVisibilityRecord(searchHit)
//	s.NotNil(info)
//	s.Equal("6bfbc1e5-6ce4-4e22-bbfb-e0faa9a7a604-1-2256", info.WorkflowID)
//	s.Equal("e481009e-14b3-45ae-91af-dce6e2a88365", info.RunID)
//	s.Equal("TestWorkflowExecute", info.TypeName)
//	s.Equal(int64(1547596872371000000), info.StartTime.UnixNano())
//
//	// test for close
//	info = s.visibilityStore.convertSearchResultToVisibilityRecord(searchHit)
//	s.NotNil(info)
//	s.Equal("6bfbc1e5-6ce4-4e22-bbfb-e0faa9a7a604-1-2256", info.WorkflowID)
//	s.Equal("e481009e-14b3-45ae-91af-dce6e2a88365", info.RunID)
//	s.Equal("TestWorkflowExecute", info.TypeName)
//	s.Equal(int64(1547596872371000000), info.StartTime.UnixNano())
//	s.Equal(int64(1547596872817380000), info.CloseTime.UnixNano())
//	s.Equal(workflow.WorkflowExecutionCloseStatusCompleted, *info.Status)
//	s.Equal(int64(29), info.HistoryLength)
//
//	// test for error case
//	badData := []byte(`corrupted data`)
//	source = (*json.RawMessage)(&badData)
//	searchHit = &elastic.SearchHit{
//		Source: source,
//	}
//	info = s.visibilityStore.convertSearchResultToVisibilityRecord(searchHit)
//	s.Nil(info)
// }

func (s *ESVisibilitySuite) TestShouldSearchAfter() {
	token := &es.ElasticVisibilityPageToken{}
	s.False(es.ShouldSearchAfter(token))

	token.TieBreaker = "a"
	s.True(es.ShouldSearchAfter(token))
}

// nolint
func (s *ESVisibilitySuite) TestGetESQueryDSL() {
	request := &p.ListWorkflowExecutionsByQueryRequest{
		DomainUUID: testDomainID,
		PageSize:   10,
	}
	token := &es.ElasticVisibilityPageToken{}

	v := s.visibilityStore

	request.Query = ""
	dsl, err := v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_all":{}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = "invaild query"
	dsl, err = v.getESQueryDSL(request, token)
	s.NotNil(err)
	s.Equal("", dsl)

	request.Query = `WorkflowID = 'wid'`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `WorkflowID = 'wid' or WorkflowID = 'another-wid'`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"should":[{"match_phrase":{"WorkflowID":{"query":"wid"}}},{"match_phrase":{"WorkflowID":{"query":"another-wid"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `WorkflowID = 'wid' order by StartTime desc`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `WorkflowID = 'wid' and CloseTime = missing`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}},{"bool":{"must_not":{"exists":{"field":"CloseTime"}}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `WorkflowID = 'wid' or CloseTime = missing`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"should":[{"match_phrase":{"WorkflowID":{"query":"wid"}}},{"bool":{"must_not":{"exists":{"field":"CloseTime"}}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `WorkflowID = 'wid' and CloseStatus = "completed"`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}},{"match_phrase":{"CloseStatus":{"query":"0"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `CloseTime = missing order by CloseTime desc`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"bool":{"must_not":{"exists":{"field":"CloseTime"}}}}]}}]}},"from":0,"size":10,"sort":[{"CloseTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `StartTime = "2018-06-07T15:04:05-08:00"`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"StartTime":{"query":"1528412645000000000"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `WorkflowID = 'wid' and StartTime > "2018-06-07T15:04:05+00:00"`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}},{"range":{"StartTime":{"gt":"1528383845000000000"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `ExecutionTime < 1000`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"gt":"0"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"lt":"1000"}}}]}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `ExecutionTime < 1000 or ExecutionTime > 2000`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"gt":"0"}}},{"bool":{"should":[{"range":{"ExecutionTime":{"lt":"1000"}}},{"range":{"ExecutionTime":{"gt":"2000"}}}]}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `order by ExecutionTime desc`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_all":{}}]}}]}},"from":0,"size":10,"sort":[{"ExecutionTime":"desc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `order by StartTime desc, CloseTime desc`
	_, err = v.getESQueryDSL(request, token)
	s.Equal(errors.New("only one field can be used to sort"), err)

	request.Query = `order by CustomStringField desc`
	_, err = v.getESQueryDSL(request, token)
	s.Equal(errors.New("not able to sort by IndexedValueTypeString field, use IndexedValueTypeKeyword field"), err)

	request.Query = `order by CustomIntField asc`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_all":{}}]}}]}},"from":0,"size":10,"sort":[{"CustomIntField":"asc"},{"RunID":"desc"}]}`, dsl)

	request.Query = `ExecutionTime < "unable to parse"`
	_, err = v.getESQueryDSL(request, token)
	s.Error(err)

	token = s.getTokenHelper(1)
	request.Query = `WorkflowID = 'wid'`
	dsl, err = v.getESQueryDSL(request, token)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}],"search_after":[1,"t"]}`, dsl)

	// invalid union injection
	request.Query = `WorkflowID = 'wid' union select * from dummy`
	_, err = v.getESQueryDSL(request, token)
	s.NotNil(err)
}

func (s *ESVisibilitySuite) TestGetESQueryDSLForScan() {
	request := &p.ListWorkflowExecutionsByQueryRequest{
		DomainUUID: testDomainID,
		PageSize:   10,
	}

	request.Query = `WorkflowID = 'wid' order by StartTime desc`
	dsl, err := getESQueryDSLForScan(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}},"from":0,"size":10}`, dsl)

	request.Query = `WorkflowID = 'wid'`
	dsl, err = getESQueryDSLForScan(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}},"from":0,"size":10}`, dsl)

	request.Query = `CloseTime = missing and (ExecutionTime >= "2019-08-27T15:04:05+00:00" or StartTime <= "2018-06-07T15:04:05+00:00")`
	dsl, err = getESQueryDSLForScan(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"gt":"0"}}},{"bool":{"must":[{"bool":{"must_not":{"exists":{"field":"CloseTime"}}}},{"bool":{"should":[{"range":{"ExecutionTime":{"from":"1566918245000000000"}}},{"range":{"StartTime":{"to":"1528383845000000000"}}}]}}]}}]}}]}},"from":0,"size":10}`, dsl)

	request.Query = `ExecutionTime < 1000 and ExecutionTime > 500`
	dsl, err = getESQueryDSLForScan(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"gt":"0"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"lt":"1000"}}},{"range":{"ExecutionTime":{"gt":"500"}}}]}}]}}]}},"from":0,"size":10}`, dsl)
}

func (s *ESVisibilitySuite) TestGetESQueryDSLForCount() {
	request := &p.CountWorkflowExecutionsRequest{
		DomainUUID: testDomainID,
	}

	// empty query
	dsl, err := getESQueryDSLForCount(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_all":{}}]}}]}}}`, dsl)

	request.Query = `WorkflowID = 'wid' order by StartTime desc`
	dsl, err = getESQueryDSLForCount(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}}}`, dsl)

	request.Query = `CloseTime < "2018-06-07T15:04:05+07:00" and StartTime > "2018-05-04T16:00:00+07:00" and ExecutionTime >= "2018-05-05T16:00:00+07:00"`
	dsl, err = getESQueryDSLForCount(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"gt":"0"}}},{"bool":{"must":[{"range":{"CloseTime":{"lt":"1528358645000000000"}}},{"range":{"StartTime":{"gt":"1525424400000000000"}}},{"range":{"ExecutionTime":{"from":"1525510800000000000"}}}]}}]}}]}}}`, dsl)

	request.Query = `ExecutionTime < 1000`
	dsl, err = getESQueryDSLForCount(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"gt":"0"}}},{"bool":{"must":[{"range":{"ExecutionTime":{"lt":"1000"}}}]}}]}}]}}}`, dsl)

	request.Query = `StartTime = missing and UpdateTime >= "2022-10-04T16:00:00+07:00"`
	dsl, err = getESQueryDSLForCount(request)
	s.Nil(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"bool":{"must_not":{"exists":{"field":"StartTime"}}}},{"range":{"UpdateTime":{"from":"1664874000000000000"}}}]}}]}}}`, dsl)

}

func (s *ESVisibilitySuite) TestAddDomainToQuery() {
	dsl := fastjson.MustParse(`{}`)
	dslStr := dsl.String()
	addDomainToQuery(dsl, "")
	s.Equal(dslStr, dsl.String())

	dsl = fastjson.MustParse(`{"query":{"bool":{"must":[{"match_all":{}}]}}}`)
	addDomainToQuery(dsl, testDomainID)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_all":{}}]}}]}}}`, dsl.String())
}

func (s *ESVisibilitySuite) TestListWorkflowExecutions() {
	s.mockESClient.On("SearchByQuery", mock.Anything, mock.MatchedBy(func(input *es.SearchByQueryRequest) bool {
		s.True(strings.Contains(input.Query, `{"match_phrase":{"CloseStatus":{"query":"5"}}}`))
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.ListWorkflowExecutionsByQueryRequest{
		DomainUUID: testDomainID,
		Domain:     testDomain,
		PageSize:   10,
		Query:      `CloseStatus = 5`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListWorkflowExecutions(ctx, request)
	s.NoError(err)

	s.mockESClient.On("SearchByQuery", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ListWorkflowExecutions(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ListWorkflowExecutions failed"))

	request.Query = `invalid query`
	_, err = s.visibilityStore.ListWorkflowExecutions(context.Background(), request)
	s.Error(err)
	_, ok = err.(*types.BadRequestError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "Error when parse query"))
}

func (s *ESVisibilitySuite) TestListWorkflowExecutionsWithLike() {
	s.mockESClient.On("SearchByQuery", mock.Anything, mock.MatchedBy(func(input *es.SearchByQueryRequest) bool {
		s.True(strings.Contains(input.Query, `{"wildcard":{"WorkflowID":{"value":"wid*","case_insensitive":true}}}`))
		s.Equal(esIndexMaxResultWindow, input.MaxResultWindow)
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.ListWorkflowExecutionsByQueryRequest{
		DomainUUID: testDomainID,
		Domain:     testDomain,
		PageSize:   10,
		Query:      `WorkflowID like 'wid'`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ListWorkflowExecutions(ctx, request)
	s.NoError(err)
}

func (s *ESVisibilitySuite) TestScanWorkflowExecutions() {
	// test first page
	s.mockESClient.On("ScanByQuery", mock.Anything, mock.MatchedBy(func(input *es.ScanByQueryRequest) bool {
		s.True(strings.Contains(input.Query, `{"match_phrase":{"CloseStatus":{"query":"5"}}}`))
		return true
	})).Return(testSearchResult, nil).Once()

	request := &p.ListWorkflowExecutionsByQueryRequest{
		DomainUUID: testDomainID,
		Domain:     testDomain,
		PageSize:   10,
		Query:      `CloseStatus = 5`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	_, err := s.visibilityStore.ScanWorkflowExecutions(ctx, request)
	s.NoError(err)

	// test bad request
	request.Query = `invalid query`
	_, err = s.visibilityStore.ScanWorkflowExecutions(ctx, request)
	s.Error(err)
	_, ok := err.(*types.BadRequestError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "Error when parse query"))

	// test internal error
	request.Query = `CloseStatus = 5`
	s.mockESClient.On("ScanByQuery", mock.Anything, mock.Anything).Return(nil, errTestESSearch).Once()
	_, err = s.visibilityStore.ScanWorkflowExecutions(ctx, request)
	s.Error(err)
	_, ok = err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "ScanWorkflowExecutions failed"))
}

func (s *ESVisibilitySuite) TestCountWorkflowExecutions() {
	s.mockESClient.On("CountByQuery", mock.Anything, testIndex, mock.MatchedBy(func(input string) bool {
		s.True(strings.Contains(input, `{"match_phrase":{"CloseStatus":{"query":"5"}}}`))
		return true
	})).Return(int64(1), nil).Once()

	request := &p.CountWorkflowExecutionsRequest{
		DomainUUID: testDomainID,
		Domain:     testDomain,
		Query:      `CloseStatus = 5`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	resp, err := s.visibilityStore.CountWorkflowExecutions(ctx, request)
	s.NoError(err)
	s.Equal(int64(1), resp.Count)

	// test internal error
	s.mockESClient.On("CountByQuery", mock.Anything, testIndex, mock.Anything).Return(int64(0), errTestESSearch).Once()

	_, err = s.visibilityStore.CountWorkflowExecutions(ctx, request)
	s.Error(err)
	_, ok := err.(*types.InternalServiceError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "CountWorkflowExecutions failed"))

	// test bad request
	request.Query = `invalid query`
	_, err = s.visibilityStore.CountWorkflowExecutions(ctx, request)
	s.Error(err)
	_, ok = err.(*types.BadRequestError)
	s.True(ok)
	s.True(strings.Contains(err.Error(), "Error when parse query"))
}

func (s *ESVisibilitySuite) TestTimeProcessFunc() {
	cases := []struct {
		key   string
		value string
	}{
		{key: "from", value: "1528358645000000000"},
		{key: "to", value: "2018-06-07T15:04:05+07:00"},
		{key: "gt", value: "some invalid time string"},
		{key: "unrelatedKey", value: "should not be modified"},
	}
	expected := []struct {
		value     string
		returnErr bool
	}{
		{value: `"1528358645000000000"`, returnErr: false},
		{value: `"1528358645000000000"`, returnErr: false},
		{value: "", returnErr: true},
		{value: `"should not be modified"`, returnErr: false},
	}

	for i, testCase := range cases {
		value := fastjson.MustParse(fmt.Sprintf(`{"%s": "%s"}`, testCase.key, testCase.value))
		err := timeProcessFunc(nil, "", value)
		if expected[i].returnErr {
			s.Error(err)
			continue
		}
		s.Equal(expected[i].value, value.Get(testCase.key).String())
	}
}

func (s *ESVisibilitySuite) TestCloseStatusProcessFunc() {
	cases := []struct {
		key   string
		value string
	}{
		{key: "from", value: types.WorkflowExecutionCloseStatusTerminated.String()},
		{key: "to", value: strings.ToLower(types.WorkflowExecutionCloseStatusContinuedAsNew.String())},
		{key: "gt", value: fmt.Sprintf(`%d`, types.WorkflowExecutionCloseStatusFailed)},
		{key: "query", value: types.WorkflowExecutionCloseStatusCompleted.String()},
		{key: "query", value: fmt.Sprintf(`%d`, types.WorkflowExecutionCloseStatusCanceled)},
		{key: "query", value: strings.ToTitle(strings.ToLower(types.WorkflowExecutionCloseStatusTimedOut.String()))},
		{key: "query", value: "timeout"},
		{key: "unrelatedKey", value: "should not be modified"},
	}
	expected := []struct {
		value     string
		returnErr bool
	}{
		{value: fmt.Sprintf(`"%d"`, types.WorkflowExecutionCloseStatusTerminated), returnErr: false},
		{value: fmt.Sprintf(`"%d"`, types.WorkflowExecutionCloseStatusContinuedAsNew), returnErr: false},
		{value: fmt.Sprintf(`"%d"`, types.WorkflowExecutionCloseStatusFailed), returnErr: false},
		{value: fmt.Sprintf(`"%d"`, types.WorkflowExecutionCloseStatusCompleted), returnErr: false},
		{value: fmt.Sprintf(`"%d"`, types.WorkflowExecutionCloseStatusCanceled), returnErr: false},
		{value: fmt.Sprintf(`"%d"`, types.WorkflowExecutionCloseStatusTimedOut), returnErr: false},
		{value: "", returnErr: true},
		{value: `"should not be modified"`, returnErr: false},
	}

	for i, testCase := range cases {
		value := fastjson.MustParse(fmt.Sprintf(`{"%s": "%s"}`, testCase.key, testCase.value))
		err := closeStatusProcessFunc(nil, "", value)
		if expected[i].returnErr {
			s.Error(err)
			continue
		}
		s.Equal(expected[i].value, value.Get(testCase.key).String())
	}
}

func (s *ESVisibilitySuite) TestProcessAllValuesForKey() {
	testJSONStr := `{
		"arrayKey": [
			{"testKey1": "value1"},
			{"testKey2": "value2"},
			{"key3": "value3"}
		],
		"key4": {
			"testKey5": "value5",
			"key6": "value6"
		},
		"testArrayKey": [
			{"testKey7": "should not be processed"}
		],
		"testKey8": "value8"
	}`
	dsl := fastjson.MustParse(testJSONStr)
	testKeyFilter := func(key string) bool {
		return strings.HasPrefix(key, "test")
	}
	processedValue := make(map[string]struct{})
	testProcessFunc := func(obj *fastjson.Object, key string, value *fastjson.Value) error {
		s.Equal(obj.Get(key), value)
		processedValue[value.String()] = struct{}{}
		return nil
	}
	s.NoError(processAllValuesForKey(dsl, testKeyFilter, testProcessFunc))

	expectedProcessedValue := map[string]struct{}{
		`"value1"`: {},
		`"value2"`: {},
		`"value5"`: {},
		`[{"testKey7":"should not be processed"}]`: {},
		`"value8"`: {},
	}
	s.Equal(expectedProcessedValue, processedValue)
}

func (s *ESVisibilitySuite) TestGetFieldType() {
	s.Equal(types.IndexedValueTypeInt, s.visibilityStore.getFieldType("StartTime"))
	s.Equal(types.IndexedValueTypeDatetime, s.visibilityStore.getFieldType("Attr.CustomDatetimeField"))
}

func (s *ESVisibilitySuite) TestGetValueOfSearchAfterInJSON() {
	v := s.visibilityStore

	// Int field
	token := s.getTokenHelper(123)
	sortField := definition.CustomIntField
	res, err := v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`[123, "t"]`, res)

	jsonData := `{"SortValue": -9223372036854776000, "TieBreaker": "t"}`
	dec := json.NewDecoder(strings.NewReader(jsonData))
	dec.UseNumber()
	err = dec.Decode(&token)
	s.Nil(err)
	res, err = v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`[-9223372036854775808, "t"]`, res)

	jsonData = `{"SortValue": 9223372036854776000, "TieBreaker": "t"}`
	dec = json.NewDecoder(strings.NewReader(jsonData))
	dec.UseNumber()
	err = dec.Decode(&token)
	s.Nil(err)
	res, err = v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`[9223372036854775807, "t"]`, res)

	// Double field
	token = s.getTokenHelper(1.11)
	sortField = definition.CustomDoubleField
	res, err = v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`[1.11, "t"]`, res)

	jsonData = `{"SortValue": "-Infinity", "TieBreaker": "t"}`
	dec = json.NewDecoder(strings.NewReader(jsonData))
	dec.UseNumber()
	err = dec.Decode(&token)
	s.Nil(err)
	res, err = v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`["-Infinity", "t"]`, res)

	// Keyword field
	token = s.getTokenHelper("keyword")
	sortField = definition.CustomKeywordField
	res, err = v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`["keyword", "t"]`, res)

	token = s.getTokenHelper(nil)
	res, err = v.getValueOfSearchAfterInJSON(token, sortField)
	s.Nil(err)
	s.Equal(`[null, "t"]`, res)
}

func (s *ESVisibilitySuite) getTokenHelper(sortValue interface{}) *es.ElasticVisibilityPageToken {
	token := &es.ElasticVisibilityPageToken{
		SortValue:  sortValue,
		TieBreaker: "t",
	}
	encoded, _ := es.SerializePageToken(token) // necessary, otherwise token is fake and not json decoded
	token, _ = es.DeserializePageToken(encoded)
	return token
}

func (s *ESVisibilitySuite) TestCleanDSL() {
	// dsl without `field`
	dsl := `{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"2b8344db-0ed6-47a4-92fd-bdeb6ead93e3"}}},{"bool":{"must":[{"match_phrase":{"Attr.CustomIntField":{"query":"1"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`
	res := cleanDSL(dsl)
	s.Equal(dsl, res)

	// dsl with `field`
	dsl = `{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"2b8344db-0ed6-47a4-92fd-bdeb6ead93e3"}}},{"bool":{"must":[{"range":{"` + "`Attr.CustomIntField`" + `":{"from":"1","to":"5"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`
	res = cleanDSL(dsl)
	expected := `{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"2b8344db-0ed6-47a4-92fd-bdeb6ead93e3"}}},{"bool":{"must":[{"range":{"Attr.CustomIntField":{"from":"1","to":"5"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`
	s.Equal(expected, res)

	// dsl with mixed
	dsl = `{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"2b8344db-0ed6-47a4-92fd-bdeb6ead93e3"}}},{"bool":{"must":[{"range":{"` + "`Attr.CustomIntField`" + `":{"from":"1","to":"5"}}},{"range":{"` + "`Attr.CustomDoubleField`" + `":{"from":"1.0","to":"2.0"}}},{"range":{"StartTime":{"gt":"0"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`
	res = cleanDSL(dsl)
	expected = `{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"2b8344db-0ed6-47a4-92fd-bdeb6ead93e3"}}},{"bool":{"must":[{"range":{"Attr.CustomIntField":{"from":"1","to":"5"}}},{"range":{"Attr.CustomDoubleField":{"from":"1.0","to":"2.0"}}},{"range":{"StartTime":{"gt":"0"}}}]}}]}},"from":0,"size":10,"sort":[{"StartTime":"desc"},{"RunID":"desc"}]}`
	s.Equal(expected, res)
}

func matchQueryData(matchQuery query.MatchQuery) (Source, error) {
	source, err := matchQuery.Source()
	if err != nil {
		return Source{}, err
	}
	d, err := json.Marshal(source)
	if err != nil {
		return Source{}, err
	}
	to := Source{}
	if err = json.Unmarshal(d, &to); err != nil {
		return Source{}, err
	}
	return to, nil
}

func (s *ESVisibilitySuite) TestGetCustomizedDSLFromSQL() {
	sql := "select * from dummy where WorkflowID like 'wid' LIMIT 100"
	dsl, err := getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"wildcard":{"WorkflowID":{"value":"wid*","case_insensitive":true}}}]}}]}},"from":0,"size":100}`, dsl.String())

	sql = "select * from dummy where Attr.CustomizedKeywordField like 'test'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"wildcard":{"Attr.CustomizedKeywordField":{"value":"test*","case_insensitive":true}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where WorkflowID = 'wid'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"WorkflowID":{"query":"wid"}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where Attr.CustomizedKeywordField = 'test'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"Attr.CustomizedKeywordField":{"query":"test"}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where WorkflowID like 'wid' and Attr.CustomizedKeywordField = 'test'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"Attr.CustomizedKeywordField":{"query":"test"}}},{"wildcard":{"WorkflowID":{"value":"wid*","case_insensitive":true}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where WorkflowID like 'like' and Attr.CustomizedKeywordField = 'like'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"match_phrase":{"Attr.CustomizedKeywordField":{"query":"like"}}},{"wildcard":{"WorkflowID":{"value":"like*","case_insensitive":true}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where WorkflowID like 'wid' and Attr.CustomizedKeywordField like 'test'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"must":[{"wildcard":{"WorkflowID":{"value":"wid*","case_insensitive":true}}},{"wildcard":{"Attr.CustomizedKeywordField":{"value":"test*","case_insensitive":true}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where WorkflowID = 'wid' OR RunID = 'runid'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"should":[{"match_phrase":{"WorkflowID":{"query":"wid"}}},{"match_phrase":{"RunID":{"query":"runid"}}}]}}]}},"from":0,"size":1}`, dsl.String())

	sql = "select * from dummy where WorkflowID like 'wid' OR Attr.CustomizedKeywordField like 'test'"
	dsl, err = getCustomizedDSLFromSQL(sql, testDomainID)
	s.NoError(err)
	s.Equal(`{"query":{"bool":{"must":[{"match_phrase":{"DomainID":{"query":"bfd5c907-f899-4baf-a7b2-2ab85e623ebd"}}},{"bool":{"should":[{"wildcard":{"WorkflowID":{"value":"wid*","case_insensitive":true}}},{"wildcard":{"Attr.CustomizedKeywordField":{"value":"test*","case_insensitive":true}}}]}}]}},"from":0,"size":1}`, dsl.String())
}
