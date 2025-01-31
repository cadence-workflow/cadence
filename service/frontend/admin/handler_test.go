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

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/client/history"
	"github.com/uber/cadence/client/matching"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/asyncworkflow/queueconfigapi"
	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	esmock "github.com/uber/cadence/common/elasticsearch/mocks"
	"github.com/uber/cadence/common/isolationgroup/isolationgroupapi"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
	"github.com/uber/cadence/service/frontend/validate"
)

type (
	adminHandlerSuite struct {
		suite.Suite
		*require.Assertions

		controller        *gomock.Controller
		mockResource      *resource.Test
		mockHistoryClient *history.MockClient
		mockDomainCache   *cache.MockDomainCache
		frontendClient    *frontend.MockClient
		mockResolver      *membership.MockResolver

		mockHistoryV2Mgr *mocks.HistoryV2Manager

		domainName string
		domainID   string

		handler *adminHandlerImpl
	}
)

func TestAdminHandlerSuite(t *testing.T) {
	s := new(adminHandlerSuite)
	suite.Run(t, s)
}

func (s *adminHandlerSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.domainName = "some random domain name"
	s.domainID = "some random domain ID"

	s.controller = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.controller, metrics.Frontend)
	s.mockDomainCache = s.mockResource.DomainCache
	s.mockHistoryClient = s.mockResource.HistoryClient
	s.mockHistoryV2Mgr = s.mockResource.HistoryMgr
	s.frontendClient = s.mockResource.FrontendClient
	s.mockResolver = s.mockResource.MembershipResolver

	params := &resource.Params{
		Logger:          testlogger.New(s.T()),
		ThrottledLogger: testlogger.New(s.T()),
		MetricScope:     tally.NewTestScope(service.Frontend, make(map[string]string)),
		MetricsClient:   metrics.NewNoopMetricsClient(),
		PersistenceConfig: config.Persistence{
			NumHistoryShards: 1,
		},
	}
	config := &frontendcfg.Config{
		EnableAdminProtection:  dynamicconfig.GetBoolPropertyFn(false),
		EnableGracefulFailover: dynamicconfig.GetBoolPropertyFn(false),
	}

	dh := domain.NewMockHandler(s.controller)
	s.handler = NewHandler(s.mockResource, params, config, dh).(*adminHandlerImpl)
	s.handler.Start()
}

func (s *adminHandlerSuite) TearDownTest() {
	s.controller.Finish()
	s.mockResource.Finish(s.T())
	s.handler.Stop()
}

func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_NormalWorkflow() {
	s.testMaintainCorruptWorkflow(nil, nil, false)
}

func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_WorkflowDoesNotExist() {
	err := &types.EntityNotExistsError{Message: "Workflow does not exist"}
	s.testMaintainCorruptWorkflow(err, nil, false)
}

func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_NoStartEvent() {
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(s.domainName, nil).AnyTimes()
	err := &types.InternalServiceError{Message: "unable to get workflow start event"}
	s.testMaintainCorruptWorkflow(err, nil, true)
}
func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_NoStartEventHistory() {
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(s.domainName, nil).AnyTimes()
	err := &types.InternalServiceError{Message: "unable to get workflow start event"}
	s.testMaintainCorruptWorkflow(nil, err, true)
}

func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_UnableToGetScheduledEvent() {
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(s.domainName, nil).AnyTimes()
	err := &types.InternalServiceError{Message: "unable to get activity scheduled event"}
	s.testMaintainCorruptWorkflow(err, nil, true)
}
func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_UnableToGetScheduledEventHistory() {
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(s.domainName, nil).AnyTimes()
	err := &types.InternalServiceError{Message: "unable to get activity scheduled event"}
	s.testMaintainCorruptWorkflow(nil, err, true)
}

func (s *adminHandlerSuite) TestMaintainCorruptWorkflow_CorruptedHistory() {
	s.mockDomainCache.EXPECT().GetDomainName(gomock.Any()).Return(s.domainName, nil).AnyTimes()
	err := &types.InternalDataInconsistencyError{
		Message: "corrupted history event batch, eventID is not continuous",
	}
	s.testMaintainCorruptWorkflow(err, nil, true)
}

func (s *adminHandlerSuite) testMaintainCorruptWorkflow(
	describeWorkflowError error,
	getHistoryError error,
	expectDeletion bool,
) {
	handler := s.handler
	handler.params = &resource.Params{}
	ctx := context.Background()

	request := &types.AdminMaintainWorkflowRequest{
		Domain: s.domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: "someWorkflowID",
			RunID:      uuid.New(),
		},
		SkipErrors: true,
	}

	// need to reeturn error here to start deleting
	describeResp := &types.DescribeWorkflowExecutionResponse{}
	s.frontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).
		Return(describeResp, describeWorkflowError)

	// need to reeturn error here to start deleting
	historyResponse := &types.GetWorkflowExecutionHistoryResponse{}
	s.frontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).
		Return(historyResponse, getHistoryError).AnyTimes()

	if expectDeletion {
		hostInfo := membership.NewHostInfo("taskListA:thriftPort")
		s.mockResolver.EXPECT().Lookup(gomock.Any(), gomock.Any()).Return(hostInfo, nil)
		s.mockDomainCache.EXPECT().GetDomainID(s.domainName).Return(s.domainID, nil)

		testMutableState := &types.DescribeMutableStateResponse{
			MutableStateInDatabase: "{\"ExecutionInfo\":{\"BranchToken\":\"WQsACgAAACQ2MzI5YzEzMi1mMGI0LTQwZmUtYWYxMS1hODVmMDA3MzAzODQLABQAAAAkOWM5OWI1MjItMGEyZi00NTdmLWEyNDgtMWU0OTA0ZDg4YzVhDwAeDAAAAAAA\"}}",
		}
		s.mockHistoryClient.EXPECT().DescribeMutableState(gomock.Any(), gomock.Any()).Return(testMutableState, nil)

		s.mockHistoryV2Mgr.On("DeleteHistoryBranch", mock.Anything, mock.Anything).Return(nil).Once()
		s.mockResource.ExecutionMgr.On("DeleteWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()
		s.mockResource.ExecutionMgr.On("DeleteCurrentWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()
		s.mockResource.VisibilityMgr.On("DeleteWorkflowExecution", mock.Anything, mock.Anything).Return(nil).Once()
	}

	_, err := handler.MaintainCorruptWorkflow(ctx, request)
	s.Nil(err)
}

func (s *adminHandlerSuite) Test_ConvertIndexedValueTypeToESDataType() {
	tests := []struct {
		input    types.IndexedValueType
		expected string
	}{
		{
			input:    types.IndexedValueTypeString,
			expected: "text",
		},
		{
			input:    types.IndexedValueTypeKeyword,
			expected: "keyword",
		},
		{
			input:    types.IndexedValueTypeInt,
			expected: "long",
		},
		{
			input:    types.IndexedValueTypeDouble,
			expected: "double",
		},
		{
			input:    types.IndexedValueTypeBool,
			expected: "boolean",
		},
		{
			input:    types.IndexedValueTypeDatetime,
			expected: "date",
		},
		{
			input:    types.IndexedValueType(-1),
			expected: "",
		},
	}

	for _, test := range tests {
		s.Equal(test.expected, convertIndexedValueTypeToESDataType(test.input))
	}
}

func (s *adminHandlerSuite) Test_GetWorkflowExecutionRawHistoryV2_FailedOnInvalidWorkflowID() {

	ctx := context.Background()
	_, err := s.handler.GetWorkflowExecutionRawHistoryV2(ctx,
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "",
				RunID:      uuid.New(),
			},
			StartEventID:      common.Int64Ptr(1),
			StartEventVersion: common.Int64Ptr(100),
			EndEventID:        common.Int64Ptr(10),
			EndEventVersion:   common.Int64Ptr(100),
			MaximumPageSize:   1,
			NextPageToken:     nil,
		})
	s.Error(err)
}

func (s *adminHandlerSuite) Test_GetWorkflowExecutionRawHistoryV2_FailedOnInvalidRunID() {
	ctx := context.Background()
	_, err := s.handler.GetWorkflowExecutionRawHistoryV2(ctx,
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "workflowID",
				RunID:      "runID",
			},
			StartEventID:      common.Int64Ptr(1),
			StartEventVersion: common.Int64Ptr(100),
			EndEventID:        common.Int64Ptr(10),
			EndEventVersion:   common.Int64Ptr(100),
			MaximumPageSize:   1,
			NextPageToken:     nil,
		})
	s.Error(err)
}

func (s *adminHandlerSuite) Test_GetWorkflowExecutionRawHistoryV2_FailedOnInvalidSize() {
	ctx := context.Background()
	_, err := s.handler.GetWorkflowExecutionRawHistoryV2(ctx,
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "workflowID",
				RunID:      uuid.New(),
			},
			StartEventID:      common.Int64Ptr(1),
			StartEventVersion: common.Int64Ptr(100),
			EndEventID:        common.Int64Ptr(10),
			EndEventVersion:   common.Int64Ptr(100),
			MaximumPageSize:   -1,
			NextPageToken:     nil,
		})
	s.Error(err)
}

func (s *adminHandlerSuite) Test_GetWorkflowExecutionRawHistoryV2_FailedOnDomainCache() {
	ctx := context.Background()
	s.mockDomainCache.EXPECT().GetDomainID(s.domainName).Return("", fmt.Errorf("test")).Times(1)
	_, err := s.handler.GetWorkflowExecutionRawHistoryV2(ctx,
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "workflowID",
				RunID:      uuid.New(),
			},
			StartEventID:      common.Int64Ptr(1),
			StartEventVersion: common.Int64Ptr(100),
			EndEventID:        common.Int64Ptr(10),
			EndEventVersion:   common.Int64Ptr(100),
			MaximumPageSize:   1,
			NextPageToken:     nil,
		})
	s.Error(err)
}

func (s *adminHandlerSuite) Test_GetWorkflowExecutionRawHistoryV2() {
	ctx := context.Background()
	s.mockDomainCache.EXPECT().GetDomainID(s.domainName).Return(s.domainID, nil).AnyTimes()
	branchToken := []byte{1}
	versionHistory := persistence.NewVersionHistory(branchToken, []*persistence.VersionHistoryItem{
		persistence.NewVersionHistoryItem(int64(10), int64(100)),
	})
	rawVersionHistories := persistence.NewVersionHistories(versionHistory)
	versionHistories := rawVersionHistories.ToInternalType()
	mState := &types.GetMutableStateResponse{
		NextEventID:        11,
		CurrentBranchToken: branchToken,
		VersionHistories:   versionHistories,
	}
	s.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), gomock.Any()).Return(mState, nil).AnyTimes()

	s.mockHistoryV2Mgr.On("ReadRawHistoryBranch", mock.Anything, mock.Anything).Return(&persistence.ReadRawHistoryBranchResponse{
		HistoryEventBlobs: []*persistence.DataBlob{},
		NextPageToken:     []byte{},
		Size:              0,
	}, nil)
	_, err := s.handler.GetWorkflowExecutionRawHistoryV2(ctx,
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "workflowID",
				RunID:      uuid.New(),
			},
			StartEventID:      common.Int64Ptr(1),
			StartEventVersion: common.Int64Ptr(100),
			EndEventID:        common.Int64Ptr(10),
			EndEventVersion:   common.Int64Ptr(100),
			MaximumPageSize:   10,
			NextPageToken:     nil,
		})
	s.NoError(err)
}

func (s *adminHandlerSuite) Test_GetWorkflowExecutionRawHistoryV2_SameStartIDAndEndID() {
	ctx := context.Background()
	s.mockDomainCache.EXPECT().GetDomainID(s.domainName).Return(s.domainID, nil).AnyTimes()
	branchToken := []byte{1}
	versionHistory := persistence.NewVersionHistory(branchToken, []*persistence.VersionHistoryItem{
		persistence.NewVersionHistoryItem(int64(10), int64(100)),
	})
	rawVersionHistories := persistence.NewVersionHistories(versionHistory)
	versionHistories := rawVersionHistories.ToInternalType()
	mState := &types.GetMutableStateResponse{
		NextEventID:        11,
		CurrentBranchToken: branchToken,
		VersionHistories:   versionHistories,
	}
	s.mockHistoryClient.EXPECT().GetMutableState(gomock.Any(), gomock.Any()).Return(mState, nil).AnyTimes()

	resp, err := s.handler.GetWorkflowExecutionRawHistoryV2(ctx,
		&types.GetWorkflowExecutionRawHistoryV2Request{
			Domain: s.domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "workflowID",
				RunID:      uuid.New(),
			},
			StartEventID:      common.Int64Ptr(10),
			StartEventVersion: common.Int64Ptr(100),
			MaximumPageSize:   1,
			NextPageToken:     nil,
		})
	s.Nil(resp.NextPageToken)
	s.NoError(err)
}

func (s *adminHandlerSuite) Test_SetRequestDefaultValueAndGetTargetVersionHistory_DefinedStartAndEnd() {
	inputStartEventID := int64(1)
	inputStartVersion := int64(10)
	inputEndEventID := int64(100)
	inputEndVersion := int64(11)
	firstItem := persistence.NewVersionHistoryItem(inputStartEventID, inputStartVersion)
	endItem := persistence.NewVersionHistoryItem(inputEndEventID, inputEndVersion)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{firstItem, endItem})
	versionHistories := persistence.NewVersionHistories(versionHistory)
	request := &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain: s.domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID",
			RunID:      uuid.New(),
		},
		StartEventID:      common.Int64Ptr(inputStartEventID),
		StartEventVersion: common.Int64Ptr(inputStartVersion),
		EndEventID:        common.Int64Ptr(inputEndEventID),
		EndEventVersion:   common.Int64Ptr(inputEndVersion),
		MaximumPageSize:   10,
		NextPageToken:     nil,
	}

	targetVersionHistory, err := s.handler.setRequestDefaultValueAndGetTargetVersionHistory(
		request,
		versionHistories,
	)
	s.Equal(request.GetStartEventID(), inputStartEventID)
	s.Equal(request.GetEndEventID(), inputEndEventID)
	s.Equal(targetVersionHistory, versionHistory)
	s.NoError(err)
}

func (s *adminHandlerSuite) Test_SetRequestDefaultValueAndGetTargetVersionHistory_DefinedEndEvent() {
	inputStartEventID := int64(1)
	inputEndEventID := int64(100)
	inputStartVersion := int64(10)
	inputEndVersion := int64(11)
	firstItem := persistence.NewVersionHistoryItem(inputStartEventID, inputStartVersion)
	targetItem := persistence.NewVersionHistoryItem(inputEndEventID, inputEndVersion)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{firstItem, targetItem})
	versionHistories := persistence.NewVersionHistories(versionHistory)
	request := &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain: s.domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID",
			RunID:      uuid.New(),
		},
		StartEventID:      nil,
		StartEventVersion: nil,
		EndEventID:        common.Int64Ptr(inputEndEventID),
		EndEventVersion:   common.Int64Ptr(inputEndVersion),
		MaximumPageSize:   10,
		NextPageToken:     nil,
	}

	targetVersionHistory, err := s.handler.setRequestDefaultValueAndGetTargetVersionHistory(
		request,
		versionHistories,
	)
	s.Equal(request.GetStartEventID(), inputStartEventID-1)
	s.Equal(request.GetEndEventID(), inputEndEventID)
	s.Equal(targetVersionHistory, versionHistory)
	s.NoError(err)
}

func (s *adminHandlerSuite) Test_SetRequestDefaultValueAndGetTargetVersionHistory_DefinedStartEvent() {
	inputStartEventID := int64(1)
	inputEndEventID := int64(100)
	inputStartVersion := int64(10)
	inputEndVersion := int64(11)
	firstItem := persistence.NewVersionHistoryItem(inputStartEventID, inputStartVersion)
	targetItem := persistence.NewVersionHistoryItem(inputEndEventID, inputEndVersion)
	versionHistory := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{firstItem, targetItem})
	versionHistories := persistence.NewVersionHistories(versionHistory)
	request := &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain: s.domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID",
			RunID:      uuid.New(),
		},
		StartEventID:      common.Int64Ptr(inputStartEventID),
		StartEventVersion: common.Int64Ptr(inputStartVersion),
		EndEventID:        nil,
		EndEventVersion:   nil,
		MaximumPageSize:   10,
		NextPageToken:     nil,
	}

	targetVersionHistory, err := s.handler.setRequestDefaultValueAndGetTargetVersionHistory(
		request,
		versionHistories,
	)
	s.Equal(request.GetStartEventID(), inputStartEventID)
	s.Equal(request.GetEndEventID(), inputEndEventID+1)
	s.Equal(targetVersionHistory, versionHistory)
	s.NoError(err)
}

func (s *adminHandlerSuite) Test_SetRequestDefaultValueAndGetTargetVersionHistory_NonCurrentBranch() {
	inputStartEventID := int64(1)
	inputEndEventID := int64(100)
	inputStartVersion := int64(10)
	inputEndVersion := int64(101)
	item1 := persistence.NewVersionHistoryItem(inputStartEventID, inputStartVersion)
	item2 := persistence.NewVersionHistoryItem(inputEndEventID, inputEndVersion)
	versionHistory1 := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{item1, item2})
	item3 := persistence.NewVersionHistoryItem(int64(10), int64(20))
	item4 := persistence.NewVersionHistoryItem(int64(20), int64(51))
	versionHistory2 := persistence.NewVersionHistory([]byte{}, []*persistence.VersionHistoryItem{item1, item3, item4})
	versionHistories := persistence.NewVersionHistories(versionHistory1)
	_, _, err := versionHistories.AddVersionHistory(versionHistory2)
	s.NoError(err)
	request := &types.GetWorkflowExecutionRawHistoryV2Request{
		Domain: s.domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: "workflowID",
			RunID:      uuid.New(),
		},
		StartEventID:      common.Int64Ptr(9),
		StartEventVersion: common.Int64Ptr(20),
		EndEventID:        common.Int64Ptr(inputEndEventID),
		EndEventVersion:   common.Int64Ptr(inputEndVersion),
		MaximumPageSize:   10,
		NextPageToken:     nil,
	}

	targetVersionHistory, err := s.handler.setRequestDefaultValueAndGetTargetVersionHistory(
		request,
		versionHistories,
	)
	s.Equal(request.GetStartEventID(), inputStartEventID)
	s.Equal(request.GetEndEventID(), inputEndEventID)
	s.Equal(targetVersionHistory, versionHistory1)
	s.NoError(err)
}

func (s *adminHandlerSuite) Test_AddSearchAttribute_Validate() {
	handler := s.handler
	handler.params = &resource.Params{}
	ctx := context.Background()

	type test struct {
		Name     string
		Request  *types.AddSearchAttributeRequest
		Expected error
	}
	// request validation tests
	testCases1 := []test{
		{
			Name:     "nil request",
			Request:  nil,
			Expected: &types.BadRequestError{Message: "Request is nil."},
		},
		{
			Name:     "empty request",
			Request:  &types.AddSearchAttributeRequest{},
			Expected: &types.BadRequestError{Message: "SearchAttributes are not provided"},
		},
	}
	for _, testCase := range testCases1 {
		s.Equal(testCase.Expected, handler.AddSearchAttribute(ctx, testCase.Request))
	}

	dynamicConfig := dynamicconfig.NewMockClient(s.controller)
	handler.params.DynamicConfig = dynamicConfig
	// add advanced visibility store related config
	handler.params.ESConfig = &config.ElasticSearchConfig{}
	esClient := &esmock.GenericClient{}
	defer func() { esClient.AssertExpectations(s.T()) }()
	handler.params.ESClient = esClient
	handler.esClient = esClient

	mockValidAttr := map[string]interface{}{
		"testkey": types.IndexedValueTypeKeyword,
	}
	dynamicConfig.EXPECT().GetMapValue(dynamicconfig.ValidSearchAttributes, nil).
		Return(mockValidAttr, nil).AnyTimes()

	testCases2 := []test{
		{
			Name: "reserved key",
			Request: &types.AddSearchAttributeRequest{
				SearchAttribute: map[string]types.IndexedValueType{
					"WorkflowID": 1,
				},
			},
			Expected: &types.BadRequestError{Message: "Key [WorkflowID] is reserved by system"},
		},
		{
			Name: "key already whitelisted",
			Request: &types.AddSearchAttributeRequest{
				SearchAttribute: map[string]types.IndexedValueType{
					"testkey": 1,
				},
			},
			Expected: &types.BadRequestError{Message: "Key [testkey] is already whitelisted as a different type"},
		},
	}
	for _, testCase := range testCases2 {
		s.Equal(testCase.Expected, handler.AddSearchAttribute(ctx, testCase.Request))
	}

	dcUpdateTest := test{
		Name: "dynamic config update failed",
		Request: &types.AddSearchAttributeRequest{
			SearchAttribute: map[string]types.IndexedValueType{
				"testkey2": -1,
			},
		},
		Expected: &types.BadRequestError{Message: "Unknown value type, IndexedValueType(-1)"},
	}
	dynamicConfig.EXPECT().UpdateValue(dynamicconfig.ValidSearchAttributes, map[string]interface{}{
		"testkey":  types.IndexedValueTypeKeyword,
		"testkey2": -1,
	}).Return(errors.New("error"))
	err := handler.AddSearchAttribute(ctx, dcUpdateTest.Request)
	s.Equal(dcUpdateTest.Expected, err)

	// ES operations tests
	dynamicConfig.EXPECT().UpdateValue(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	convertFailedTest := test{
		Name: "unknown value type",
		Request: &types.AddSearchAttributeRequest{
			SearchAttribute: map[string]types.IndexedValueType{
				"testkey3": -1,
			},
		},
		Expected: &types.BadRequestError{Message: "Unknown value type, IndexedValueType(-1)"},
	}
	s.Equal(convertFailedTest.Expected, handler.AddSearchAttribute(ctx, convertFailedTest.Request))

	esClient.On("PutMapping", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("error"))
	esClient.On("IsNotFoundError", mock.Anything).Return(false)
	esErrorTest := test{
		Name: "es error",
		Request: &types.AddSearchAttributeRequest{
			SearchAttribute: map[string]types.IndexedValueType{
				"testkey4": 1,
			},
		},
		Expected: &types.InternalServiceError{Message: "Failed to update ES mapping, err: error"},
	}
	s.Equal(esErrorTest.Expected, handler.AddSearchAttribute(ctx, esErrorTest.Request))
}

func (s *adminHandlerSuite) Test_AddSearchAttribute_Permission() {
	ctx := context.Background()
	handler := s.handler
	handler.config = &frontendcfg.Config{
		EnableAdminProtection: dynamicconfig.GetBoolPropertyFn(true),
		AdminOperationToken:   dynamicconfig.GetStringPropertyFn(dynamicconfig.AdminOperationToken.DefaultString()),
	}

	type test struct {
		Name     string
		Request  *types.AddSearchAttributeRequest
		Expected error
	}
	testCases := []test{
		{
			Name: "unknown token",
			Request: &types.AddSearchAttributeRequest{
				SecurityToken: "unknown",
			},
			Expected: validate.ErrNoPermission,
		},
		{
			Name: "correct token",
			Request: &types.AddSearchAttributeRequest{
				SecurityToken: dynamicconfig.AdminOperationToken.DefaultString(),
			},
			Expected: &types.BadRequestError{Message: "SearchAttributes are not provided"},
		},
	}
	for _, testCase := range testCases {
		s.Equal(testCase.Expected, handler.AddSearchAttribute(ctx, testCase.Request))
	}
}

func (s *adminHandlerSuite) Test_ConfigStore_NilRequest() {
	ctx := context.Background()
	handler := s.handler

	_, err := handler.GetDynamicConfig(ctx, nil)
	s.Error(err)

	err = handler.UpdateDynamicConfig(ctx, nil)
	s.Error(err)

	err = handler.RestoreDynamicConfig(ctx, nil)
	s.Error(err)
}

func (s *adminHandlerSuite) Test_DescribeShardDistribution() {
	s.mockResource.MembershipResolver.EXPECT().Lookup(service.History, string(rune(0))).
		Return(membership.NewHostInfo("127.0.0.1:1234"), nil)

	res, err := s.handler.DescribeShardDistribution(
		context.Background(),
		&types.DescribeShardDistributionRequest{PageSize: 10},
	)
	s.Require().NoError(err)
	s.Equal(
		&types.DescribeShardDistributionResponse{
			NumberOfShards: 1,
			Shards:         map[int32]string{0: "127.0.0.1:1234"},
		},
		res,
	)
}

func (s *adminHandlerSuite) Test_ConfigStore_InvalidKey() {
	ctx := context.Background()
	handler := s.handler

	_, err := handler.GetDynamicConfig(ctx, &types.GetDynamicConfigRequest{
		ConfigName: "invalid key",
		Filters:    nil,
	})
	s.Error(err)

	err = handler.UpdateDynamicConfig(ctx, &types.UpdateDynamicConfigRequest{
		ConfigName:   "invalid key",
		ConfigValues: nil,
	})
	s.Error(err)

	err = handler.RestoreDynamicConfig(ctx, &types.RestoreDynamicConfigRequest{
		ConfigName: "invalid key",
		Filters:    nil,
	})
	s.Error(err)
}

func (s *adminHandlerSuite) Test_GetDynamicConfig_NoFilter() {
	ctx := context.Background()
	handler := s.handler
	dynamicConfig := dynamicconfig.NewMockClient(s.controller)
	handler.params.DynamicConfig = dynamicConfig

	dynamicConfig.EXPECT().
		GetValue(dynamicconfig.TestGetBoolPropertyKey).
		Return(true, nil).AnyTimes()

	resp, err := handler.GetDynamicConfig(ctx, &types.GetDynamicConfigRequest{
		ConfigName: dynamicconfig.TestGetBoolPropertyKey.String(),
		Filters:    nil,
	})
	s.NoError(err)

	encTrue, err := json.Marshal(true)
	s.NoError(err)
	s.Equal(resp.Value.Data, encTrue)
}

func (s *adminHandlerSuite) Test_GetDynamicConfig_FilterMatch() {
	ctx := context.Background()
	handler := s.handler
	dynamicConfig := dynamicconfig.NewMockClient(s.controller)
	handler.params.DynamicConfig = dynamicConfig

	dynamicConfig.EXPECT().
		GetValueWithFilters(dynamicconfig.TestGetBoolPropertyKey, map[dynamicconfig.Filter]interface{}{
			dynamicconfig.DomainName: "samples_domain",
		}).
		Return(true, nil).AnyTimes()

	encDomainName, err := json.Marshal("samples_domain")
	s.NoError(err)

	resp, err := handler.GetDynamicConfig(ctx, &types.GetDynamicConfigRequest{
		ConfigName: dynamicconfig.TestGetBoolPropertyKey.String(),
		Filters: []*types.DynamicConfigFilter{
			{
				Name: dynamicconfig.DomainName.String(),
				Value: &types.DataBlob{
					EncodingType: types.EncodingTypeJSON.Ptr(),
					Data:         encDomainName,
				},
			},
		},
	})
	s.NoError(err)

	encTrue, err := json.Marshal(true)
	s.NoError(err)
	s.Equal(resp.Value.Data, encTrue)
}

func Test_GetGlobalIsolationGroups(t *testing.T) {

	validResponse := types.GetGlobalIsolationGroupsResponse{
		IsolationGroups: types.IsolationGroupConfiguration{
			"zone-1": types.IsolationGroupPartition{
				Name:  "zone-1",
				State: types.IsolationGroupStateDrained,
			},
			"zone-2": types.IsolationGroupPartition{
				Name:  "zone-2",
				State: types.IsolationGroupStateHealthy,
			},
			"zone-3": types.IsolationGroupPartition{
				Name:  "zone-3",
				State: types.IsolationGroupStateDrained,
			},
		},
	}

	tests := map[string]struct {
		ighandlerAffordance func(mock *isolationgroupapi.MockHandler)
		expectOut           *types.GetGlobalIsolationGroupsResponse
		expectedErr         error
	}{
		"happy-path - no errors and payload is decoded and returned": {
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().GetGlobalState(gomock.Any()).Return(&validResponse, nil)
			},
			expectOut: &validResponse,
		},
		"an error returned": {
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().GetGlobalState(gomock.Any()).Return(nil, assert.AnError)
			},
			expectedErr: &types.InternalServiceError{Message: assert.AnError.Error()},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			goMock := gomock.NewController(t)
			igMock := isolationgroupapi.NewMockHandler(goMock)
			td.ighandlerAffordance(igMock)

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				isolationGroups: igMock,
			}

			res, err := handler.GetGlobalIsolationGroups(context.Background(), &types.GetGlobalIsolationGroupsRequest{})

			assert.Equal(t, td.expectOut, res)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func Test_UpdateGlobalIsolationGroups(t *testing.T) {

	validConfig := types.UpdateGlobalIsolationGroupsRequest{
		IsolationGroups: types.IsolationGroupConfiguration{
			"zone-1": {
				Name:  "zone-1",
				State: types.IsolationGroupStateDrained,
			},
			"zone-2": {
				Name:  "zone-2",
				State: types.IsolationGroupStateHealthy,
			},
			"zone-3": {
				Name:  "zone-3",
				State: types.IsolationGroupStateDrained,
			},
		},
	}

	tests := map[string]struct {
		ighandlerAffordance func(mock *isolationgroupapi.MockHandler)
		input               *types.UpdateGlobalIsolationGroupsRequest
		expectOut           *types.UpdateGlobalIsolationGroupsResponse
		expectedErr         error
	}{
		"happy-path - update to the database": {
			input: &validConfig,
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().UpdateGlobalState(gomock.Any(), validConfig).Return(nil)
			},
			expectOut: &types.UpdateGlobalIsolationGroupsResponse{},
		},
		"happy-path - an error is returned": {
			input: &validConfig,
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().UpdateGlobalState(gomock.Any(), validConfig).Return(assert.AnError)
			},
			expectedErr: &types.InternalServiceError{Message: assert.AnError.Error()},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			igMock := isolationgroupapi.NewMockHandler(goMock)
			td.ighandlerAffordance(igMock)

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				isolationGroups: igMock,
			}

			res, err := handler.UpdateGlobalIsolationGroups(context.Background(), td.input)

			assert.Equal(t, td.expectOut, res)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func Test_GetDomainIsolationGroups(t *testing.T) {

	validResponse := types.GetDomainIsolationGroupsResponse{
		IsolationGroups: types.IsolationGroupConfiguration{
			"zone-1": types.IsolationGroupPartition{
				Name:  "zone-1",
				State: types.IsolationGroupStateDrained,
			},
			"zone-2": types.IsolationGroupPartition{
				Name:  "zone-2",
				State: types.IsolationGroupStateHealthy,
			},
			"zone-3": types.IsolationGroupPartition{
				Name:  "zone-3",
				State: types.IsolationGroupStateDrained,
			},
		},
	}

	tests := map[string]struct {
		ighandlerAffordance func(mock *isolationgroupapi.MockHandler)
		expectOut           *types.GetDomainIsolationGroupsResponse
		expectedErr         error
	}{
		"happy-path - no errors and payload is decoded and returned": {
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().GetDomainState(gomock.Any(), types.GetDomainIsolationGroupsRequest{
					Domain: "domain",
				}).Return(&validResponse, nil)
			},
			expectOut: &validResponse,
		},
		"an error returned": {
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().GetDomainState(gomock.Any(), types.GetDomainIsolationGroupsRequest{
					Domain: "domain",
				}).Return(nil, assert.AnError)
			},
			expectedErr: &types.InternalServiceError{Message: assert.AnError.Error()},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {

			goMock := gomock.NewController(t)
			igMock := isolationgroupapi.NewMockHandler(goMock)
			td.ighandlerAffordance(igMock)

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				isolationGroups: igMock,
			}

			res, err := handler.GetDomainIsolationGroups(context.Background(), &types.GetDomainIsolationGroupsRequest{
				Domain: "domain",
			})

			assert.Equal(t, td.expectOut, res)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func Test_UpdateDomainIsolationGroups(t *testing.T) {

	validConfig := types.UpdateDomainIsolationGroupsRequest{
		Domain: "domain",
		IsolationGroups: types.IsolationGroupConfiguration{
			"zone-1": {
				Name:  "zone-1",
				State: types.IsolationGroupStateDrained,
			},
			"zone-2": {
				Name:  "zone-2",
				State: types.IsolationGroupStateHealthy,
			},
			"zone-3": {
				Name:  "zone-3",
				State: types.IsolationGroupStateDrained,
			},
		},
	}

	tests := map[string]struct {
		ighandlerAffordance func(mock *isolationgroupapi.MockHandler)
		input               *types.UpdateDomainIsolationGroupsRequest
		expectOut           *types.UpdateDomainIsolationGroupsResponse
		expectedErr         error
	}{
		"happy-path - update to the database": {
			input: &validConfig,
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().UpdateDomainState(gomock.Any(), validConfig).Return(nil)
			},
			expectOut: &types.UpdateDomainIsolationGroupsResponse{},
		},
		"happy-path - an error is returned": {
			input: &validConfig,
			ighandlerAffordance: func(mock *isolationgroupapi.MockHandler) {
				mock.EXPECT().UpdateDomainState(gomock.Any(), validConfig).Return(assert.AnError)
			},
			expectedErr: &types.InternalServiceError{Message: assert.AnError.Error()},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			igMock := isolationgroupapi.NewMockHandler(goMock)
			td.ighandlerAffordance(igMock)

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				isolationGroups: igMock,
			}

			res, err := handler.UpdateDomainIsolationGroups(context.Background(), td.input)

			assert.Equal(t, td.expectOut, res)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func Test_GetDomainAsyncWorkflowConfiguraton(t *testing.T) {
	tests := map[string]struct {
		queueCfgHandlerMockFn func(mock *queueconfigapi.MockHandler)
		input                 *types.GetDomainAsyncWorkflowConfiguratonRequest
		wantResp              *types.GetDomainAsyncWorkflowConfiguratonResponse
		wantErr               error
	}{
		"success": {
			input: &types.GetDomainAsyncWorkflowConfiguratonRequest{Domain: "test-domain"},
			queueCfgHandlerMockFn: func(mock *queueconfigapi.MockHandler) {
				mock.EXPECT().GetConfiguraton(gomock.Any(), gomock.Any()).Return(&types.GetDomainAsyncWorkflowConfiguratonResponse{}, nil).Times(1)
			},
			wantResp: &types.GetDomainAsyncWorkflowConfiguratonResponse{},
		},
		"nil request": {
			input:   nil,
			wantErr: validate.ErrRequestNotSet,
		},
		"queue config handler failed": {
			input: &types.GetDomainAsyncWorkflowConfiguratonRequest{Domain: "test-domain"},
			queueCfgHandlerMockFn: func(mock *queueconfigapi.MockHandler) {
				mock.EXPECT().GetConfiguraton(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed")).Times(1)
			},
			wantErr: &types.InternalServiceError{Message: "failed"},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			queueCfgHandlerMock := queueconfigapi.NewMockHandler(goMock)
			if td.queueCfgHandlerMockFn != nil {
				td.queueCfgHandlerMockFn(queueCfgHandlerMock)
			}

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				asyncWFQueueConfigs: queueCfgHandlerMock,
			}

			res, err := handler.GetDomainAsyncWorkflowConfiguraton(context.Background(), td.input)

			assert.Equal(t, td.wantResp, res)
			assert.Equal(t, td.wantErr, err)
		})
	}
}

func Test_UpdateDomainAsyncWorkflowConfiguraton(t *testing.T) {
	tests := map[string]struct {
		queueCfgHandlerMockFn func(mock *queueconfigapi.MockHandler)
		input                 *types.UpdateDomainAsyncWorkflowConfiguratonRequest
		wantResp              *types.UpdateDomainAsyncWorkflowConfiguratonResponse
		wantErr               error
	}{
		"success": {
			input: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{Domain: "test-domain"},
			queueCfgHandlerMockFn: func(mock *queueconfigapi.MockHandler) {
				mock.EXPECT().UpdateConfiguration(gomock.Any(), gomock.Any()).Return(&types.UpdateDomainAsyncWorkflowConfiguratonResponse{}, nil).Times(1)
			},
			wantResp: &types.UpdateDomainAsyncWorkflowConfiguratonResponse{},
		},
		"nil request": {
			input:   nil,
			wantErr: validate.ErrRequestNotSet,
		},
		"queue config handler failed": {
			input: &types.UpdateDomainAsyncWorkflowConfiguratonRequest{Domain: "test-domain"},
			queueCfgHandlerMockFn: func(mock *queueconfigapi.MockHandler) {
				mock.EXPECT().UpdateConfiguration(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed")).Times(1)
			},
			wantErr: &types.InternalServiceError{Message: "failed"},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			queueCfgHandlerMock := queueconfigapi.NewMockHandler(goMock)
			if td.queueCfgHandlerMockFn != nil {
				td.queueCfgHandlerMockFn(queueCfgHandlerMock)
			}

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				asyncWFQueueConfigs: queueCfgHandlerMock,
			}

			res, err := handler.UpdateDomainAsyncWorkflowConfiguraton(context.Background(), td.input)

			assert.Equal(t, td.wantResp, res)
			assert.Equal(t, td.wantErr, err)
		})
	}
}

func Test_RemoveTask(t *testing.T) {
	tests := map[string]struct {
		input         *types.RemoveTaskRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.RemoveTaskRequest{
				ShardID: 1,
				Type:    common.Int32Ptr(2),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().RemoveTask(gomock.Any(), &types.RemoveTaskRequest{
					ShardID: 1,
					Type:    common.Int32Ptr(2),
				}).Return(nil)
			},
			wantErr: false,
		},
		"request failed": {
			input: &types.RemoveTaskRequest{
				ShardID: 1,
				Type:    common.Int32Ptr(2),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().RemoveTask(gomock.Any(), &types.RemoveTaskRequest{
					ShardID: 1,
					Type:    common.Int32Ptr(2),
				}).Return(assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			err := handler.RemoveTask(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_CloseShard(t *testing.T) {
	tests := map[string]struct {
		input         *types.CloseShardRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.CloseShardRequest{
				ShardID: 1,
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().CloseShard(gomock.Any(), &types.CloseShardRequest{
					ShardID: 1,
				}).Return(nil)
			},
			wantErr: false,
		},
		"request failed": {
			input: &types.CloseShardRequest{
				ShardID: 1,
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().CloseShard(gomock.Any(), &types.CloseShardRequest{
					ShardID: 1,
				}).Return(assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			err := handler.CloseShard(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ResetQueue(t *testing.T) {
	tests := map[string]struct {
		input         *types.ResetQueueRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing cluster name request": {
			input: &types.ResetQueueRequest{
				ShardID: 1,
				Type:    common.Int32Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.ResetQueueRequest{
				ShardID:     1,
				ClusterName: "test-cluster",
				Type:        common.Int32Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().ResetQueue(gomock.Any(), &types.ResetQueueRequest{
					ShardID:     1,
					ClusterName: "test-cluster",
					Type:        common.Int32Ptr(1),
				}).Return(nil).Times(1)
			},
			wantErr: false,
		},
		"normal request return error": {
			input: &types.ResetQueueRequest{
				ShardID:     1,
				ClusterName: "test-cluster",
				Type:        common.Int32Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().ResetQueue(gomock.Any(), &types.ResetQueueRequest{
					ShardID:     1,
					ClusterName: "test-cluster",
					Type:        common.Int32Ptr(1),
				}).Return(assert.AnError).Times(1)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			err := handler.ResetQueue(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_DescribeQueue(t *testing.T) {
	tests := map[string]struct {
		input         *types.DescribeQueueRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing cluster name request": {
			input: &types.DescribeQueueRequest{
				ShardID: 1,
				Type:    common.Int32Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.DescribeQueueRequest{
				ShardID:     1,
				ClusterName: "test-cluster",
				Type:        common.Int32Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().DescribeQueue(gomock.Any(), &types.DescribeQueueRequest{
					ShardID:     1,
					ClusterName: "test-cluster",
					Type:        common.Int32Ptr(1),
				}).Return(nil, nil)
			},
			wantErr: false,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			_, err := handler.DescribeQueue(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_DescribeHistoryHost(t *testing.T) {
	tests := map[string]struct {
		input         *types.DescribeHistoryHostRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing host address request": {
			input: &types.DescribeHistoryHostRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing execution": {
			input: &types.DescribeHistoryHostRequest{
				ExecutionForHost: &types.WorkflowExecution{
					WorkflowID: "",
					RunID:      uuid.New(),
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.DescribeHistoryHostRequest{
				ExecutionForHost: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
					RunID:      uuid.New(),
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().DescribeHistoryHost(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			_, err := handler.DescribeHistoryHost(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_DescribeCluster(t *testing.T) {
	goMock := gomock.NewController(t)
	mockResource := resource.NewTest(t, goMock, metrics.Frontend)
	mockResource.VisibilityMgr.On("GetName").Return("test").Once()
	mockResource.HistoryMgr.On("GetName").Return("test").Once()
	mockResource.MembershipResolver.EXPECT().WhoAmI().Return(membership.NewHostInfo("1.0.0.1"), nil).AnyTimes()
	mockResource.MembershipResolver.EXPECT().Members(gomock.Any()).Return([]membership.HostInfo{
		membership.NewHostInfo("1.0.0.1"),
	}, nil).AnyTimes()
	handler := adminHandlerImpl{
		params: &resource.Params{
			ESConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{},
			},
		},
		Resource: mockResource,
	}
	_, err := handler.DescribeCluster(context.Background())
	assert.NoError(t, err)
}

func Test_DescribeCluster_HostError(t *testing.T) {
	goMock := gomock.NewController(t)
	mockResource := resource.NewTest(t, goMock, metrics.Frontend)
	mockResource.VisibilityMgr.On("GetName").Return("test").Once()
	mockResource.HistoryMgr.On("GetName").Return("test").Once()
	mockResource.MembershipResolver.EXPECT().WhoAmI().Return(membership.NewHostInfo("1.0.0.1"), assert.AnError).AnyTimes()
	handler := adminHandlerImpl{
		params: &resource.Params{
			ESConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{},
			},
		},
		Resource: mockResource,
	}
	_, err := handler.DescribeCluster(context.Background())
	assert.Error(t, err)
}

func Test_DescribeCluster_MemberError(t *testing.T) {
	goMock := gomock.NewController(t)
	mockResource := resource.NewTest(t, goMock, metrics.Frontend)
	mockResource.VisibilityMgr.On("GetName").Return("test").Once()
	mockResource.HistoryMgr.On("GetName").Return("test").Once()
	mockResource.MembershipResolver.EXPECT().WhoAmI().Return(membership.NewHostInfo("1.0.0.1"), nil).AnyTimes()
	mockResource.MembershipResolver.EXPECT().Members(gomock.Any()).Return([]membership.HostInfo{
		membership.NewHostInfo("1.0.0.1"),
	}, assert.AnError).AnyTimes()
	handler := adminHandlerImpl{
		params: &resource.Params{
			ESConfig: &config.ElasticSearchConfig{
				Indices: map[string]string{},
			},
		},
		Resource: mockResource,
	}
	_, err := handler.DescribeCluster(context.Background())
	assert.Error(t, err)
}

func TestGetReplicationMessages(t *testing.T) {
	tests := map[string]struct {
		input         *types.GetReplicationMessagesRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing cluster name": {
			input: &types.GetReplicationMessagesRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.GetReplicationMessagesRequest{
				ClusterName: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		"return error": {
			input: &types.GetReplicationMessagesRequest{
				ClusterName: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().GetReplicationMessages(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			_, err := handler.GetReplicationMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_GetDomainReplicationMessages(t *testing.T) {
	tests := map[string]struct {
		input          *types.GetDomainReplicationMessagesRequest
		hcHandlerFunc  func(mock *history.MockClient)
		drqHandlerFunc func(mock *domain.MockReplicationQueue)
		wantErr        bool
	}{
		"nil request": {
			input:          nil,
			hcHandlerFunc:  func(mock *history.MockClient) {},
			drqHandlerFunc: func(mock *domain.MockReplicationQueue) {},
			wantErr:        true,
		},
		"with default last message ID": {
			input: &types.GetDomainReplicationMessagesRequest{
				LastRetrievedMessageID: common.Int64Ptr(-1), // default value
				ClusterName:            "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			drqHandlerFunc: func(mock *domain.MockReplicationQueue) {
				mock.EXPECT().GetAckLevels(context.Background()).Return(map[string]int64{
					"test-cluster": 1,
				}, nil)
				mock.EXPECT().GetReplicationMessages(context.Background(), int64(1), getDomainReplicationMessageBatchSize).Return(nil, int64(2), nil)
				mock.EXPECT().UpdateAckLevel(context.Background(), int64(-1), "test-cluster").Return(nil)
			},
			wantErr: false,
		},
		"with random last message ID but update ack level error": {
			input: &types.GetDomainReplicationMessagesRequest{
				LastRetrievedMessageID: common.Int64Ptr(1),
				LastProcessedMessageID: common.Int64Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			drqHandlerFunc: func(mock *domain.MockReplicationQueue) {
				mock.EXPECT().GetReplicationMessages(context.Background(), int64(1), getDomainReplicationMessageBatchSize).Return(nil, int64(2), nil)
				mock.EXPECT().UpdateAckLevel(context.Background(), int64(1), "").Return(assert.AnError)
			},
			wantErr: false,
		},
		"get replication messages error": {
			input: &types.GetDomainReplicationMessagesRequest{
				LastRetrievedMessageID: common.Int64Ptr(1),
				LastProcessedMessageID: common.Int64Ptr(1),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			drqHandlerFunc: func(mock *domain.MockReplicationQueue) {
				mock.EXPECT().GetReplicationMessages(context.Background(), int64(1), getDomainReplicationMessageBatchSize).Return(nil, int64(2), assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			drqMock := domain.NewMockReplicationQueue(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			if td.drqHandlerFunc != nil {
				td.drqHandlerFunc(drqMock)
			}

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:                 testlogger.New(t),
					MetricsClient:          metrics.NewNoopMetricsClient(),
					DomainReplicationQueue: drqMock,
				},
			}

			resp, err := handler.GetDomainReplicationMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(2), resp.Messages.LastRetrievedMessageID)
			}
		})
	}
}

func Test_GetDLQReplicationMessages(t *testing.T) {
	tests := map[string]struct {
		input         *types.GetDLQReplicationMessagesRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing task info": {
			input: &types.GetDLQReplicationMessagesRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.GetDLQReplicationMessagesRequest{
				TaskInfos: []*types.ReplicationTaskInfo{
					{
						DomainID:     "test-domain-id",
						WorkflowID:   "test-workflow-id",
						RunID:        uuid.New(),
						TaskID:       int64(1),
						TaskType:     1,
						Version:      0,
						FirstEventID: 1,
					},
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		"get dql message error": {
			input: &types.GetDLQReplicationMessagesRequest{
				TaskInfos: []*types.ReplicationTaskInfo{
					{
						DomainID:   "test-domain-id",
						WorkflowID: "test-workflow-id",
					},
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().GetDLQReplicationMessages(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			_, err := handler.GetDLQReplicationMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ReapplyEvents(t *testing.T) {
	tests := map[string]struct {
		input         *types.ReapplyEventsRequest
		hcHandlerFunc func(mock *history.MockClient)
		dcHandlerFunc func(mock *cache.MockDomainCache)
		wantErr       bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"empty domain name": {
			input:   &types.ReapplyEventsRequest{},
			wantErr: true,
		},
		"empty workflow execution": {
			input: &types.ReapplyEventsRequest{
				DomainName: "test-domain",
			},
			wantErr: true,
		},
		"empty workflow ID": {
			input: &types.ReapplyEventsRequest{
				DomainName:        "test-domain",
				WorkflowExecution: &types.WorkflowExecution{},
			},
			wantErr: true,
		},
		"empty events": {
			input: &types.ReapplyEventsRequest{
				DomainName: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
			},
			wantErr: true,
		},
		"get domain error": {
			input: &types.ReapplyEventsRequest{
				DomainName: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
				Events: &types.DataBlob{},
			},
			dcHandlerFunc: func(mock *cache.MockDomainCache) {
				mock.EXPECT().GetDomain("test-domain").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.ReapplyEventsRequest{
				DomainName: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
				Events: &types.DataBlob{
					EncodingType: types.EncodingTypeThriftRW.Ptr(),
					Data:         []byte("test-event-data"),
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().ReapplyEvents(context.Background(), &types.HistoryReapplyEventsRequest{
					DomainUUID: "test-domain-id",
					Request: &types.ReapplyEventsRequest{
						DomainName: "test-domain",
						WorkflowExecution: &types.WorkflowExecution{
							WorkflowID: "test-workflow-id",
						},
						Events: &types.DataBlob{
							EncodingType: types.EncodingTypeThriftRW.Ptr(),
							Data:         []byte("test-event-data"),
						},
					},
				}).Return(nil)
			},
			dcHandlerFunc: func(mock *cache.MockDomainCache) {
				mock.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-domain-id"},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{},
					0,
				), nil)
			},
			wantErr: false,
		},
		"reapply events error": {
			input: &types.ReapplyEventsRequest{
				DomainName: "test-domain",
				WorkflowExecution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
				Events: &types.DataBlob{},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().ReapplyEvents(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			dcHandlerFunc: func(mock *cache.MockDomainCache) {
				mock.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-domain-id"},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{},
					0,
				), nil)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			dcMock := cache.NewMockDomainCache(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			if td.dcHandlerFunc != nil {
				td.dcHandlerFunc(dcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
					DomainCache:   dcMock,
				},
			}

			err := handler.ReapplyEvents(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ReadDLQMessages(t *testing.T) {
	tests := map[string]struct {
		input            *types.ReadDLQMessagesRequest
		hcHandlerFunc    func(mock *history.MockClient)
		retryHandelrFunc func(mock *backoff.ThrottleRetry)
		wantErr          bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"missing type": {
			input:   &types.ReadDLQMessagesRequest{},
			wantErr: true,
		},
		"type DLQTypeReplication": {
			input: &types.ReadDLQMessagesRequest{
				Type: types.DLQTypeReplication.Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().ReadDLQMessages(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		"type DLQTypeDomain": {
			input: &types.ReadDLQMessagesRequest{
				Type: types.DLQTypeDomain.Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			retryHandelrFunc: func(mock *backoff.ThrottleRetry) {
			},
		},
		"default type": {
			input: &types.ReadDLQMessagesRequest{
				Type: types.DLQType(22).Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			if td.retryHandelrFunc != nil {
				policy := backoff.NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetMaximumInterval(5 * time.Millisecond)
				policy.SetMaximumAttempts(5)
				handler.throttleRetry = backoff.NewThrottleRetry(
					backoff.WithRetryPolicy(policy),
					backoff.WithRetryableError(func(_ error) bool { return true }),
				)
				mockDlqHandler := domain.NewMockDLQMessageHandler(goMock)
				mockDlqHandler.EXPECT().Read(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				handler.domainDLQHandler = mockDlqHandler
			}

			_, err := handler.ReadDLQMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_PurgeDLQMessages(t *testing.T) {
	tests := map[string]struct {
		input            *types.PurgeDLQMessagesRequest
		hcHandlerFunc    func(mock *history.MockClient)
		retryHandelrFunc func(mock *backoff.ThrottleRetry)
		wantErr          bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"missing type": {
			input:   &types.PurgeDLQMessagesRequest{},
			wantErr: true,
		},
		"type DLQTypeReplication": {
			input: &types.PurgeDLQMessagesRequest{
				Type: types.DLQTypeReplication.Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().PurgeDLQMessages(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		"type DLQTypeDomain": {
			input: &types.PurgeDLQMessagesRequest{
				Type: types.DLQTypeDomain.Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			retryHandelrFunc: func(mock *backoff.ThrottleRetry) {
			},
		},
		"default type": {
			input: &types.PurgeDLQMessagesRequest{
				Type: types.DLQType(22).Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			if td.retryHandelrFunc != nil {
				policy := backoff.NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetMaximumInterval(5 * time.Millisecond)
				policy.SetMaximumAttempts(5)
				handler.throttleRetry = backoff.NewThrottleRetry(
					backoff.WithRetryPolicy(policy),
					backoff.WithRetryableError(func(_ error) bool { return true }),
				)
				mockDlqHandler := domain.NewMockDLQMessageHandler(goMock)
				mockDlqHandler.EXPECT().Purge(gomock.Any(), gomock.Any()).Return(nil)
				handler.domainDLQHandler = mockDlqHandler
			}

			err := handler.PurgeDLQMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_CountDQLMessages(t *testing.T) {
	tests := map[string]struct {
		input          *types.CountDLQMessagesRequest
		hcHandlerFunc  func(mock *history.MockClient)
		dlqHandlerFunc func(mock *domain.MockDLQMessageHandler)
		wantErr        bool
	}{
		"normal request": {
			input: &types.CountDLQMessagesRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any()).Return(&types.HistoryCountDLQMessagesResponse{
					Entries: map[types.HistoryDLQCountKey]int64{
						{
							ShardID:       1,
							SourceCluster: "test-cluster",
						}: 1,
					},
				}, nil)
			},
			dlqHandlerFunc: func(mock *domain.MockDLQMessageHandler) {
				mock.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			wantErr: false,
		},
		"domain dlq handler error": {
			input: &types.CountDLQMessagesRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			dlqHandlerFunc: func(mock *domain.MockDLQMessageHandler) {
				mock.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int64(1), assert.AnError)
			},
			wantErr: true,
		},
		"history client error": {
			input: &types.CountDLQMessagesRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().CountDLQMessages(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			dlqHandlerFunc: func(mock *domain.MockDLQMessageHandler) {
				mock.EXPECT().Count(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			dlqMock := domain.NewMockDLQMessageHandler(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			if td.dlqHandlerFunc != nil {
				td.dlqHandlerFunc(dlqMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
				domainDLQHandler: dlqMock,
			}

			_, err := handler.CountDLQMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_MergeDLQMessages(t *testing.T) {
	tests := map[string]struct {
		input            *types.MergeDLQMessagesRequest
		hcHandlerFunc    func(mock *history.MockClient)
		retryHandelrFunc func(mock *backoff.ThrottleRetry)
		wantErr          bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"missing type": {
			input:   &types.MergeDLQMessagesRequest{},
			wantErr: true,
		},
		"type DLQTypeReplication": {
			input: &types.MergeDLQMessagesRequest{
				Type: types.DLQTypeReplication.Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().MergeDLQMessages(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		"type DLQTypeDomain": {
			input: &types.MergeDLQMessagesRequest{
				Type: types.DLQTypeDomain.Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			retryHandelrFunc: func(mock *backoff.ThrottleRetry) {
			},
		},
		"default type": {
			input: &types.MergeDLQMessagesRequest{
				Type: types.DLQType(22).Ptr(),
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			if td.retryHandelrFunc != nil {
				policy := backoff.NewExponentialRetryPolicy(1 * time.Millisecond)
				policy.SetMaximumInterval(5 * time.Millisecond)
				policy.SetMaximumAttempts(5)
				handler.throttleRetry = backoff.NewThrottleRetry(
					backoff.WithRetryPolicy(policy),
					backoff.WithRetryableError(func(_ error) bool { return true }),
				)
				mockDlqHandler := domain.NewMockDLQMessageHandler(goMock)
				mockDlqHandler.EXPECT().Merge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				handler.domainDLQHandler = mockDlqHandler
			}

			_, err := handler.MergeDLQMessages(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_RefreshWorkflowTasks(t *testing.T) {
	tests := map[string]struct {
		input         *types.RefreshWorkflowTasksRequest
		hcHandlerFunc func(mock *history.MockClient)
		dcHandlerFunc func(mock *cache.MockDomainCache)
		wantErr       bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"empty workflow execution": {
			input: &types.RefreshWorkflowTasksRequest{
				Domain: "test-domain",
			},
			wantErr: true,
		},
		"check execution error": {
			input: &types.RefreshWorkflowTasksRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
			},
			dcHandlerFunc: func(mock *cache.MockDomainCache) {
				mock.EXPECT().GetDomain("test-domain").Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.RefreshWorkflowTasksRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any()).Return(nil)
			},
			dcHandlerFunc: func(mock *cache.MockDomainCache) {
				mock.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-domain-id"},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{},
					0,
				), nil)
			},
			wantErr: false,
		},
		"refresh workflow tasks error": {
			input: &types.RefreshWorkflowTasksRequest{
				Domain: "test-domain",
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-workflow-id",
				},
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().RefreshWorkflowTasks(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			dcHandlerFunc: func(mock *cache.MockDomainCache) {
				mock.EXPECT().GetDomain("test-domain").Return(cache.NewGlobalDomainCacheEntryForTest(
					&persistence.DomainInfo{ID: "test-domain-id"},
					&persistence.DomainConfig{},
					&persistence.DomainReplicationConfig{},
					0,
				), nil)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			hcMock := history.NewMockClient(ctrl)
			dcMock := cache.NewMockDomainCache(ctrl)
			if tt.hcHandlerFunc != nil {
				tt.hcHandlerFunc(hcMock)
			}
			if tt.dcHandlerFunc != nil {
				tt.dcHandlerFunc(dcMock)
			}

			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
					DomainCache:   dcMock,
				},
			}

			err := handler.RefreshWorkflowTasks(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ResendReplicationTasks(t *testing.T) {
	tests := map[string]struct {
		input         *types.ResendReplicationTasksRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.ResendReplicationTasksRequest{
				RemoteCluster: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			err := handler.ResendReplicationTasks(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_GetCrossClusterTasks(t *testing.T) {
	tests := map[string]struct {
		input         *types.GetCrossClusterTasksRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input: nil,
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"missing cluster name": {
			input: &types.GetCrossClusterTasksRequest{},
			hcHandlerFunc: func(mock *history.MockClient) {
			},
			wantErr: true,
		},
		"normal request": {
			input: &types.GetCrossClusterTasksRequest{
				TargetCluster: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().GetCrossClusterTasks(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		"return error": {
			input: &types.GetCrossClusterTasksRequest{
				TargetCluster: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().GetCrossClusterTasks(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			_, err := handler.GetCrossClusterTasks(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_RespondCrossClusterTasksCompleted(t *testing.T) {
	tests := map[string]struct {
		input         *types.RespondCrossClusterTasksCompletedRequest
		hcHandlerFunc func(mock *history.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"empty target cluster": {
			input:   &types.RespondCrossClusterTasksCompletedRequest{},
			wantErr: true,
		},
		"normal request": {
			input: &types.RespondCrossClusterTasksCompletedRequest{
				TargetCluster: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().RespondCrossClusterTasksCompleted(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
		"respond error": {
			input: &types.RespondCrossClusterTasksCompletedRequest{
				TargetCluster: "test-cluster",
			},
			hcHandlerFunc: func(mock *history.MockClient) {
				mock.EXPECT().RespondCrossClusterTasksCompleted(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			if td.hcHandlerFunc != nil {
				td.hcHandlerFunc(hcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
			}

			_, err := handler.RespondCrossClusterTasksCompleted(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_SerializeRawHistoryToken(t *testing.T) {
	input := getWorkflowRawHistoryV2Token{
		DomainName: "test-domain",
	}
	marshaledToken, _ := json.Marshal(input)
	tests := map[string]struct {
		input              *getWorkflowRawHistoryV2Token
		wantResp           []byte
		wantSerializeErr   bool
		wantDeserializeErr bool
	}{
		"normal request": {
			input: &getWorkflowRawHistoryV2Token{
				DomainName: "test-domain",
			},
			wantResp:           marshaledToken,
			wantSerializeErr:   false,
			wantDeserializeErr: false,
		},
		"nil request": {
			input:              nil,
			wantResp:           nil,
			wantSerializeErr:   false,
			wantDeserializeErr: true,
		},
	}
	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			resp, err := serializeRawHistoryToken(td.input)
			if td.wantSerializeErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, td.wantResp, resp)
			}
			// deserialize the serialized token
			res, err := deserializeRawHistoryToken(resp)
			if td.wantDeserializeErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, td.input, res)
			}
		})
	}
}

func Test_ListDynamicConfig(t *testing.T) {
	tests := map[string]struct {
		input         *types.ListDynamicConfigRequest
		dcHandlerFunc func(mock *dynamicconfig.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			hcMock := history.NewMockClient(goMock)
			dcMock := dynamicconfig.NewMockClient(goMock)
			if td.dcHandlerFunc != nil {
				td.dcHandlerFunc(dcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
					HistoryClient: hcMock,
				},
				params: &resource.Params{
					DynamicConfig: dcMock,
				},
			}

			_, err := handler.ListDynamicConfig(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfigForAdvanceVisibility_Error(t *testing.T) {
	handler := adminHandlerImpl{
		params: &resource.Params{},
	}
	err := handler.validateConfigForAdvanceVisibility()
	assert.Error(t, err, "ES related config not found")
}

func TestRestoreDynamicConfig(t *testing.T) {
	tests := map[string]struct {
		input         *types.RestoreDynamicConfigRequest
		dcHandlerFunc func(mock *dynamicconfig.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"invalid config name": {
			input: &types.RestoreDynamicConfigRequest{
				ConfigName: "invalid",
			},
			wantErr: true,
		},
		"valid config name": {
			input: &types.RestoreDynamicConfigRequest{
				ConfigName: "testGetIntPropertyKey",
				Filters:    nil,
			},
			dcHandlerFunc: func(mock *dynamicconfig.MockClient) {
				mock.EXPECT().RestoreValue(gomock.Any(), nil).Return(nil)
			},
			wantErr: false,
		},
		"valid config with invalid filters": {
			input: &types.RestoreDynamicConfigRequest{
				ConfigName: "testGetIntPropertyKey",
				Filters: []*types.DynamicConfigFilter{
					&types.DynamicConfigFilter{
						Name: "test-filter",
						Value: &types.DataBlob{
							Data: []byte("test-value"),
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			dcMock := dynamicconfig.NewMockClient(goMock)
			if td.dcHandlerFunc != nil {
				td.dcHandlerFunc(dcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				params: &resource.Params{
					DynamicConfig: dcMock,
				},
			}

			err := handler.RestoreDynamicConfig(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListDynamicConfig(t *testing.T) {
	tests := map[string]struct {
		input         *types.ListDynamicConfigRequest
		dcHandlerFunc func(mock *dynamicconfig.MockClient)
		wantErr       bool
	}{
		"nil request": {
			input:   nil,
			wantErr: true,
		},
		"invalid config name": {
			input: &types.ListDynamicConfigRequest{
				ConfigName: "invalid",
			},
			dcHandlerFunc: func(mock *dynamicconfig.MockClient) {
				mock.EXPECT().ListValue(gomock.Any()).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		"valid config name": {
			input: &types.ListDynamicConfigRequest{
				ConfigName: "testGetIntPropertyKey",
			},
			dcHandlerFunc: func(mock *dynamicconfig.MockClient) {
				mock.EXPECT().ListValue(gomock.Any()).Return(nil, nil)
			},
			wantErr: false,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			goMock := gomock.NewController(t)
			dcMock := dynamicconfig.NewMockClient(goMock)
			if td.dcHandlerFunc != nil {
				td.dcHandlerFunc(dcMock)
			}
			handler := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:        testlogger.New(t),
					MetricsClient: metrics.NewNoopMetricsClient(),
				},
				params: &resource.Params{
					DynamicConfig: dcMock,
				},
			}

			_, err := handler.ListDynamicConfig(context.Background(), td.input)
			if td.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateTaskListPartitionConfig(t *testing.T) {
	domainName := "domain-name"
	domainID := "domain-id"
	taskListName := "task-list"
	kind := types.TaskListKindNormal
	taskListType := types.TaskListTypeActivity
	partitionConfig := &types.TaskListPartitionConfig{
		ReadPartitions: map[int]*types.TaskListPartition{
			0: {},
			1: {},
		},
		WritePartitions: map[int]*types.TaskListPartition{
			0: {},
		},
	}

	testCases := []struct {
		name          string
		req           *types.UpdateTaskListPartitionConfigRequest
		setupMocks    func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain:          domainName,
				TaskList:        &types.TaskList{Name: taskListName, Kind: &kind},
				TaskListType:    &taskListType,
				PartitionConfig: partitionConfig,
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
				mockMatchingClient.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), &types.MatchingUpdateTaskListPartitionConfigRequest{
					DomainUUID:      domainID,
					TaskList:        &types.TaskList{Name: taskListName, Kind: &kind},
					TaskListType:    &taskListType,
					PartitionConfig: partitionConfig,
				}).Return(nil, nil)
			},
			expectError: false,
		},
		{
			name: "request not set",
			req:  nil,
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				// no mocks needed as the function should exit early
			},
			expectError:   true,
			expectedError: validate.ErrRequestNotSet.Error(),
		},
		{
			name: "task list not set",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain: domainName,
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: validate.ErrTaskListNotSet.Error(),
		},
		{
			name: "task list kind not set",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain:   domainName,
				TaskList: &types.TaskList{Name: taskListName},
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: "Task list kind not set.",
		},
		{
			name: "invalid task list kind",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain:   domainName,
				TaskList: &types.TaskList{Name: taskListName, Kind: types.TaskListKindSticky.Ptr()},
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: "Only normal tasklist's partition config can be updated.",
		},
		{
			name: "task list type not set",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain:   domainName,
				TaskList: &types.TaskList{Name: taskListName, Kind: &kind},
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: "Task list type not set.",
		},
		{
			name: "partition config not set",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain:       domainName,
				TaskList:     &types.TaskList{Name: taskListName, Kind: &kind},
				TaskListType: &taskListType,
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: "Task list partition config is not set in the request.",
		},
		{
			name: "invalid partition config: write partitions > read partitions",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain: domainName,
				TaskList: &types.TaskList{
					Name: taskListName,
					Kind: &kind,
				},
				TaskListType: &taskListType,
				PartitionConfig: &types.TaskListPartitionConfig{
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
					},
					WritePartitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
				},
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: "The number of write partitions cannot be larger than the number of read partitions.",
		},
		{
			name: "invalid partition config: write partitions <= 0",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain: domainName,
				TaskList: &types.TaskList{
					Name: taskListName,
					Kind: &kind,
				},
				TaskListType: &taskListType,
				PartitionConfig: &types.TaskListPartitionConfig{
					ReadPartitions: map[int]*types.TaskListPartition{
						0: {},
						1: {},
					},
					WritePartitions: nil,
				},
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
			},
			expectError:   true,
			expectedError: "The number of partitions must be larger than 0.",
		},
		{
			name: "domain cache error",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain: domainName,
				TaskList: &types.TaskList{
					Name: taskListName,
					Kind: &kind,
				},
				TaskListType:    &taskListType,
				PartitionConfig: partitionConfig,
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return("", errors.New("domain cache error"))
			},
			expectError:   true,
			expectedError: "domain cache error",
		},
		{
			name: "matching client error",
			req: &types.UpdateTaskListPartitionConfigRequest{
				Domain: domainName,
				TaskList: &types.TaskList{
					Name: taskListName,
					Kind: &kind,
				},
				TaskListType:    &taskListType,
				PartitionConfig: partitionConfig,
			},
			setupMocks: func(mockMatchingClient *matching.MockClient, mockDomainCache *cache.MockDomainCache) {
				mockDomainCache.EXPECT().GetDomainID(domainName).Return(domainID, nil)
				mockMatchingClient.EXPECT().UpdateTaskListPartitionConfig(gomock.Any(), &types.MatchingUpdateTaskListPartitionConfigRequest{
					DomainUUID:      domainID,
					TaskList:        &types.TaskList{Name: taskListName, Kind: &kind},
					TaskListType:    &taskListType,
					PartitionConfig: partitionConfig,
				}).Return(nil, errors.New("matching client error"))
			},
			expectError:   true,
			expectedError: "matching client error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockDomainCache := cache.NewMockDomainCache(ctrl)
			mockMatchingClient := matching.NewMockClient(ctrl)
			tc.setupMocks(mockMatchingClient, mockDomainCache)
			adh := adminHandlerImpl{
				Resource: &resource.Test{
					Logger:         testlogger.New(t),
					MetricsClient:  metrics.NewNoopMetricsClient(),
					DomainCache:    mockDomainCache,
					MatchingClient: mockMatchingClient,
				},
			}

			resp, err := adh.UpdateTaskListPartitionConfig(context.Background(), tc.req)

			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}
