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

package decision

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest/observer"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	commonconstants "github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/execution"
)

type (
	attrValidatorSuite struct {
		suite.Suite
		*require.Assertions

		controller      *gomock.Controller
		mockDomainCache *cache.MockDomainCache

		validator *attrValidator

		testDomainID       string
		testTargetDomainID string

		testActivityMaxScheduleToStartTimeoutForRetryInSeconds int32
	}
)

func TestAttrValidatorSuite(t *testing.T) {
	s := new(attrValidatorSuite)
	suite.Run(t, s)
}

func (s *attrValidatorSuite) SetupSuite() {
	s.testDomainID = "test domain ID"
	s.testTargetDomainID = "test target domain ID"
	s.testActivityMaxScheduleToStartTimeoutForRetryInSeconds = 1800
}

func (s *attrValidatorSuite) TearDownSuite() {
}

func (s *attrValidatorSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockDomainCache = cache.NewMockDomainCache(s.controller)
	config := &config.Config{
		MaxIDLengthWarnLimit:              dynamicproperties.GetIntPropertyFn(128),
		DomainNameMaxLength:               dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		IdentityMaxLength:                 dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		WorkflowIDMaxLength:               dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		SignalNameMaxLength:               dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		WorkflowTypeMaxLength:             dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		RequestIDMaxLength:                dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		TaskListNameMaxLength:             dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		ActivityIDMaxLength:               dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		ActivityTypeMaxLength:             dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		MarkerNameMaxLength:               dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		TimerIDMaxLength:                  dynamicproperties.GetIntPropertyFilteredByDomain(1000),
		ValidSearchAttributes:             dynamicproperties.GetMapPropertyFn(definition.GetDefaultIndexedKeys()),
		EnableQueryAttributeValidation:    dynamicproperties.GetBoolPropertyFn(true),
		SearchAttributesNumberOfKeysLimit: dynamicproperties.GetIntPropertyFilteredByDomain(100),
		SearchAttributesSizeOfValueLimit:  dynamicproperties.GetIntPropertyFilteredByDomain(2 * 1024),
		SearchAttributesTotalSizeLimit:    dynamicproperties.GetIntPropertyFilteredByDomain(40 * 1024),
		ActivityMaxScheduleToStartTimeoutForRetry: dynamicproperties.GetDurationPropertyFnFilteredByDomain(
			time.Duration(s.testActivityMaxScheduleToStartTimeoutForRetryInSeconds) * time.Second,
		),
		EnableCrossClusterOperationsForDomain: dynamicproperties.GetBoolPropertyFnFilteredByDomain(false),
	}
	s.validator = newAttrValidator(
		s.mockDomainCache,
		metrics.NewNoopMetricsClient(),
		config,
		log.NewNoop(),
	)
}

func (s *attrValidatorSuite) TearDownTest() {
	s.controller.Finish()
}

func (s *attrValidatorSuite) TestValidateSignalExternalWorkflowExecutionAttributes() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).AnyTimes()
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).AnyTimes()

	var attributes *types.SignalExternalWorkflowExecutionDecisionAttributes

	err := s.validator.validateSignalExternalWorkflowExecutionAttributes(s.testDomainID, s.testTargetDomainID, attributes, metrics.HistoryRespondDecisionTaskCompletedScope)
	s.EqualError(err, "SignalExternalWorkflowExecutionDecisionAttributes is not set on decision.")

	attributes = &types.SignalExternalWorkflowExecutionDecisionAttributes{}
	err = s.validator.validateSignalExternalWorkflowExecutionAttributes(s.testDomainID, s.testTargetDomainID, attributes, metrics.HistoryRespondDecisionTaskCompletedScope)
	s.EqualError(err, "Execution is nil on decision.")

	attributes.Execution = &types.WorkflowExecution{}
	attributes.Execution.WorkflowID = "workflow-id"
	err = s.validator.validateSignalExternalWorkflowExecutionAttributes(s.testDomainID, s.testTargetDomainID, attributes, metrics.HistoryRespondDecisionTaskCompletedScope)
	s.EqualError(err, "SignalName is not set on decision.")

	attributes.Execution.RunID = "run-id"
	err = s.validator.validateSignalExternalWorkflowExecutionAttributes(s.testDomainID, s.testTargetDomainID, attributes, metrics.HistoryRespondDecisionTaskCompletedScope)
	s.EqualError(err, "Invalid RunId set on decision.")
	attributes.Execution.RunID = constants.TestRunID

	attributes.SignalName = "my signal name"
	err = s.validator.validateSignalExternalWorkflowExecutionAttributes(s.testDomainID, s.testTargetDomainID, attributes, metrics.HistoryRespondDecisionTaskCompletedScope)
	s.NoError(err)

	attributes.Input = []byte("test input")
	err = s.validator.validateSignalExternalWorkflowExecutionAttributes(s.testDomainID, s.testTargetDomainID, attributes, metrics.HistoryRespondDecisionTaskCompletedScope)
	s.NoError(err)
}

func (s *attrValidatorSuite) TestValidateUpsertWorkflowSearchAttributes() {
	domainName := "testDomain"
	var attributes *types.UpsertWorkflowSearchAttributesDecisionAttributes

	err := s.validator.validateUpsertWorkflowSearchAttributes(domainName, attributes)
	s.EqualError(err, "UpsertWorkflowSearchAttributesDecisionAttributes is not set on decision.")

	attributes = &types.UpsertWorkflowSearchAttributesDecisionAttributes{}
	err = s.validator.validateUpsertWorkflowSearchAttributes(domainName, attributes)
	s.EqualError(err, "SearchAttributes is not set on decision.")

	attributes.SearchAttributes = &types.SearchAttributes{}
	err = s.validator.validateUpsertWorkflowSearchAttributes(domainName, attributes)
	s.EqualError(err, "IndexedFields is empty on decision.")

	attributes.SearchAttributes.IndexedFields = map[string][]byte{"CustomKeywordField": []byte(`"bytes"`)}
	err = s.validator.validateUpsertWorkflowSearchAttributes(domainName, attributes)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_LocalToLocal() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_LocalToEffectiveLocal_SameCluster() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_LocalToEffectiveLocal_DiffCluster() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestAlternativeClusterName}},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_LocalToGlobal() {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_EffectiveLocalToLocal_SameCluster() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}},
		},
		1234,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_EffectiveLocalToLocal_DiffCluster() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestAlternativeClusterName}},
		},
		1234,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_EffectiveLocalToEffectiveLocal_SameCluster() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}},
		},
		1234,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}},
		},
		5678,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_EffectiveLocalToEffectiveLocal_DiffCluster() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestCurrentClusterName}},
		},
		1234,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters:          []*persistence.ClusterReplicationConfig{{ClusterName: cluster.TestAlternativeClusterName}},
		},
		5678,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_EffectiveLocalToGlobal() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
		5678,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_GlobalToLocal() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_GlobalToEffectiveLocal() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		5678,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_GlobalToGlobal_SameDomain() {
	targetDomainID := s.testDomainID

	err := s.validator.validateCrossDomainCall(s.testDomainID, targetDomainID)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_GlobalToGlobal_DiffDomain_SameCluster() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestAlternativeClusterName},
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
		1234,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(2)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(2)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)

	s.validator.config.EnableCrossClusterOperationsForDomain = dynamicproperties.GetBoolPropertyFnFilteredByDomain(true)
	err = s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.Nil(err)
}

func (s *attrValidatorSuite) TestValidateCrossDomainCall_GlobalToGlobal_DiffDomain_DiffCluster() {
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestAlternativeClusterName},
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: "cluster name for s.testDomainID"},
			},
		},
		1234,
	)
	targetDomainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
				{ClusterName: "cluster name for s.testTargetDomainID"},
			},
		},
		1234,
	)

	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateCrossDomainCall(s.testDomainID, s.testTargetDomainID)
	s.IsType(&types.BadRequestError{}, err)
}

func (s *attrValidatorSuite) TestValidateTaskListName() {
	taskList := func(name string) *types.TaskList {
		kind := types.TaskListKindNormal
		return &types.TaskList{Name: name, Kind: &kind}
	}
	ephemeralTasklist := func(name string) *types.TaskList {
		kind := types.TaskListKindEphemeral
		return &types.TaskList{Name: name, Kind: &kind}
	}

	testCases := []struct {
		defaultVal  *types.TaskList
		input       *types.TaskList
		output      *types.TaskList
		isOutputErr bool
	}{
		{taskList("tl-1"), nil, taskList("tl-1"), false},
		{taskList(""), taskList("tl-1"), taskList("tl-1"), false},
		{taskList("tl-1"), taskList("tl-1"), taskList("tl-1"), false},
		{taskList(""), taskList("/tl-1"), taskList("/tl-1"), false},
		{ephemeralTasklist("tl-1"), nil, ephemeralTasklist("tl-1"), false},
		{ephemeralTasklist("tl-1"), taskList("tl-1"), ephemeralTasklist("tl-1"), false},
		{ephemeralTasklist("tl-1"), taskList("tl-2"), ephemeralTasklist("tl-2"), false},
		{ephemeralTasklist("tl-1"), taskList(""), ephemeralTasklist("tl-1"), false},
		{taskList(""), taskList("/__cadence_sys"), taskList("/__cadence_sys"), false},
		{taskList(""), nil, &types.TaskList{}, true},
		{taskList(""), taskList(""), taskList(""), true},
		{taskList(""), taskList(commonconstants.ReservedTaskListPrefix), taskList(commonconstants.ReservedTaskListPrefix), true},
		{taskList("tl-1"), taskList(commonconstants.ReservedTaskListPrefix), taskList(commonconstants.ReservedTaskListPrefix), true},
		{taskList(""), taskList(commonconstants.ReservedTaskListPrefix + "tl-1"), taskList(commonconstants.ReservedTaskListPrefix + "tl-1"), true},
		{taskList("tl-1"), taskList(commonconstants.ReservedTaskListPrefix + "tl-1"), taskList(commonconstants.ReservedTaskListPrefix + "tl-1"), true},
	}

	for _, tc := range testCases {
		key := tc.defaultVal.Name + "," + tc.defaultVal.Kind.String() + "#"
		if tc.input != nil {
			key += tc.input.GetName()
		} else {
			key += "nil"
		}
		s.Run(key, func() {
			output, err := s.validator.validatedTaskList(tc.input, tc.defaultVal, metrics.HistoryRespondDecisionTaskCompletedScope, "domain_name")
			if tc.isOutputErr {
				s.Error(err)
			} else {
				s.NoError(err)
			}
			s.EqualValues(tc.output, output)
		})
	}
}

func (s *attrValidatorSuite) TestValidateActivityScheduleAttributes_NoRetryPolicy() {
	wfTimeout := int32(5)
	attributes := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID: "some random activityID",
		ActivityType: &types.ActivityType{
			Name: "some random activity type",
		},
		Domain: s.testDomainID,
		TaskList: &types.TaskList{
			Name: "some random task list",
		},
		Input:                         []byte{1, 2, 3},
		ScheduleToCloseTimeoutSeconds: nil, // not set
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(3),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(3),  // ScheduleToStart + StartToClose > wfTimeout
		HeartbeatTimeoutSeconds:       common.Int32Ptr(10), // larger then wfTimeout
	}

	expectedAttributesAfterValidation := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    attributes.ActivityID,
		ActivityType:                  attributes.ActivityType,
		Domain:                        attributes.Domain,
		TaskList:                      attributes.TaskList,
		Input:                         attributes.Input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(wfTimeout),
		ScheduleToStartTimeoutSeconds: attributes.ScheduleToStartTimeoutSeconds,
		StartToCloseTimeoutSeconds:    attributes.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       common.Int32Ptr(wfTimeout),
	}

	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	executionInfo := &persistence.WorkflowExecutionInfo{
		WorkflowTimeout: wfTimeout,
	}
	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateActivityScheduleAttributes(
		s.testDomainID,
		s.testTargetDomainID,
		attributes,
		executionInfo,
		metrics.HistoryRespondDecisionTaskCompletedScope,
	)
	s.Nil(err)
	s.Equal(expectedAttributesAfterValidation, attributes)
}

func (s *attrValidatorSuite) TestValidateActivityScheduleAttributes_WithRetryPolicy_ScheduleToStartRetryable() {
	s.mockDomainCache.EXPECT().GetDomainName(s.testDomainID).Return("some random domain name", nil).Times(1)

	wfTimeout := int32(3000)
	attributes := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID: "some random activityID",
		ActivityType: &types.ActivityType{
			Name: "some random activity type",
		},
		Domain: s.testDomainID,
		TaskList: &types.TaskList{
			Name: "some random task list",
		},
		Input:                         []byte{1, 2, 3},
		ScheduleToCloseTimeoutSeconds: nil, // not set
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(3),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(500), // extended ScheduleToStart + StartToClose > wfTimeout
		HeartbeatTimeoutSeconds:       common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.1,
			ExpirationIntervalInSeconds: s.testActivityMaxScheduleToStartTimeoutForRetryInSeconds + 1000, // larger than maximumScheduleToStartTimeoutForRetryInSeconds
			NonRetriableErrorReasons:    []string{"non-retryable error"},
		},
	}

	expectedAttributesAfterValidation := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    attributes.ActivityID,
		ActivityType:                  attributes.ActivityType,
		Domain:                        attributes.Domain,
		TaskList:                      attributes.TaskList,
		Input:                         attributes.Input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(attributes.RetryPolicy.ExpirationIntervalInSeconds),
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(s.testActivityMaxScheduleToStartTimeoutForRetryInSeconds),
		StartToCloseTimeoutSeconds:    attributes.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       attributes.HeartbeatTimeoutSeconds,
		RetryPolicy:                   attributes.RetryPolicy,
	}

	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	executionInfo := &persistence.WorkflowExecutionInfo{
		WorkflowTimeout: wfTimeout,
	}
	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateActivityScheduleAttributes(
		s.testDomainID,
		s.testTargetDomainID,
		attributes,
		executionInfo,
		metrics.HistoryRespondDecisionTaskCompletedScope,
	)
	s.Nil(err)
	s.Equal(expectedAttributesAfterValidation, attributes)
}

func (s *attrValidatorSuite) TestValidateActivityScheduleAttributes_WithRetryPolicy_ScheduleToStartNonRetryable() {
	wfTimeout := int32(1000)
	attributes := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID: "some random activityID",
		ActivityType: &types.ActivityType{
			Name: "some random activity type",
		},
		Domain: s.testDomainID,
		TaskList: &types.TaskList{
			Name: "some random task list",
		},
		Input:                         []byte{1, 2, 3},
		ScheduleToCloseTimeoutSeconds: nil, // not set
		ScheduleToStartTimeoutSeconds: common.Int32Ptr(3),
		StartToCloseTimeoutSeconds:    common.Int32Ptr(500), // extended ScheduleToStart + StartToClose > wfTimeout
		HeartbeatTimeoutSeconds:       common.Int32Ptr(1),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1.1,
			ExpirationIntervalInSeconds: s.testActivityMaxScheduleToStartTimeoutForRetryInSeconds + 1000, // larger than wfTimeout and maximumScheduleToStartTimeoutForRetryInSeconds
			NonRetriableErrorReasons:    []string{"cadenceInternal:Timeout SCHEDULE_TO_START"},
		},
	}

	expectedAttributesAfterValidation := &types.ScheduleActivityTaskDecisionAttributes{
		ActivityID:                    attributes.ActivityID,
		ActivityType:                  attributes.ActivityType,
		Domain:                        attributes.Domain,
		TaskList:                      attributes.TaskList,
		Input:                         attributes.Input,
		ScheduleToCloseTimeoutSeconds: common.Int32Ptr(wfTimeout),
		ScheduleToStartTimeoutSeconds: attributes.ScheduleToStartTimeoutSeconds,
		StartToCloseTimeoutSeconds:    attributes.StartToCloseTimeoutSeconds,
		HeartbeatTimeoutSeconds:       attributes.HeartbeatTimeoutSeconds,
		RetryPolicy:                   attributes.RetryPolicy,
	}

	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	targetDomainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{Name: s.testTargetDomainID},
		nil,
		cluster.TestCurrentClusterName,
	)
	executionInfo := &persistence.WorkflowExecutionInfo{
		WorkflowTimeout: wfTimeout,
	}
	s.mockDomainCache.EXPECT().GetDomainByID(s.testDomainID).Return(domainEntry, nil).Times(1)
	s.mockDomainCache.EXPECT().GetDomainByID(s.testTargetDomainID).Return(targetDomainEntry, nil).Times(1)

	err := s.validator.validateActivityScheduleAttributes(
		s.testDomainID,
		s.testTargetDomainID,
		attributes,
		executionInfo,
		metrics.HistoryRespondDecisionTaskCompletedScope,
	)
	s.Nil(err)
	s.Equal(expectedAttributesAfterValidation, attributes)
}

const (
	testDomainID   = "test-domain-id"
	testDomainName = "test-domain"
	testWorkflowID = "test-workflow-id"
	testRunID      = "test-run-id"
)

func TestWorkflowSizeChecker_failWorkflowIfBlobSizeExceedsLimit(t *testing.T) {
	var (
		testDecisionTag = metrics.DecisionTypeTag(types.DecisionTypeCompleteWorkflowExecution.String())
		testEventID     = int64(1)
		testMessage     = "test"
	)

	for name, tc := range map[string]struct {
		blobSizeLimitWarn    int
		blobSizeLimitError   int
		blob                 []byte
		assertLogsAndMetrics func(*testing.T, *observer.ObservedLogs, tally.TestScope)
		expectFail           bool
	}{
		"no errors": {
			blobSizeLimitWarn:  10,
			blobSizeLimitError: 20,
			blob:               []byte("test"),
			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				assert.Empty(t, logs.All())
				// ensure metrics with the size is emitted.
				timerData := slices.Collect(maps.Values(scope.Snapshot().Timers()))
				assert.Len(t, timerData, 2)
				assert.Equal(t, "test.event_blob_size", timerData[0].Name())
			},
		},
		"warn": {
			blobSizeLimitWarn:  10,
			blobSizeLimitError: 20,
			blob:               []byte("should-warn"),
			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				logEntries := logs.All()
				require.Len(t, logEntries, 1)
				assert.Equal(t, "Blob size close to the limit.", logEntries[0].Message)
			},
		},
		"fail": {
			blobSizeLimitWarn:  5,
			blobSizeLimitError: 10,
			blob:               []byte("should-fail"),
			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				logEntries := logs.All()
				require.Len(t, logEntries, 1)
				assert.Equal(t, "Blob size exceeds limit.", logEntries[0].Message)
			},
			expectFail: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mutableState := execution.NewMockMutableState(ctrl)
			logger, logs := testlogger.NewObserved(t)
			metricsScope := tally.NewTestScope("test", nil)
			checker := &workflowSizeChecker{
				blobSizeLimitWarn:  tc.blobSizeLimitWarn,
				blobSizeLimitError: tc.blobSizeLimitError,
				completedID:        testEventID,
				mutableState:       mutableState,
				logger:             logger,
				metricsScope:       metrics.NewClient(metricsScope, metrics.History, metrics.HistogramMigration{}).Scope(metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DomainTag(testDomainName)),
			}
			mutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
				DomainID:   testDomainID,
				WorkflowID: testWorkflowID,
				RunID:      testRunID,
			}).Times(1)
			if tc.expectFail {
				mutableState.EXPECT().AddFailWorkflowEvent(testEventID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonDecisionBlobSizeExceedsLimit),
					Details: []byte(testMessage),
				}).Return(nil, nil).Times(1)
			}
			failed, err := checker.failWorkflowIfBlobSizeExceedsLimit(testDecisionTag, tc.blob, testMessage)
			require.NoError(t, err)
			if tc.assertLogsAndMetrics != nil {
				tc.assertLogsAndMetrics(t, logs, metricsScope)
			}
			assert.Equal(t, tc.expectFail, failed)
		})
	}

}

func TestWorkflowSizeChecker_failWorkflowSizeExceedsLimit(t *testing.T) {
	var (
		testEventID = int64(1)
	)

	for name, tc := range map[string]struct {
		historyCount           int
		historyCountLimitWarn  int
		historyCountLimitError int

		historySize           int
		historySizeLimitWarn  int
		historySizeLimitError int

		noExecutionCall bool

		assertLogsAndMetrics func(*testing.T, *observer.ObservedLogs, tally.TestScope)
		expectFail           bool
	}{
		"no errors": {
			historyCount:           1,
			historyCountLimitWarn:  10,
			historyCountLimitError: 20,
			historySize:            1,
			historySizeLimitWarn:   10,
			historySizeLimitError:  20,
			noExecutionCall:        true,
			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				assert.Empty(t, logs.All())
				// ensure metrics with the size is emitted.
				timerData := slices.Collect(maps.Values(scope.Snapshot().Timers()))
				assert.Len(t, timerData, 4)
				timerNames := make([]string, 0, 4)
				for _, timer := range timerData {
					timerNames = append(timerNames, timer.Name())
				}
				slices.Sort(timerNames)

				// timers are duplicated for specific domain and domain: all
				assert.Equal(t, []string{"test.history_count", "test.history_count", "test.history_size", "test.history_size"}, timerNames)
			},
		},
		"count warn": {
			historyCount:           15,
			historyCountLimitWarn:  10,
			historyCountLimitError: 20,

			historySize:           1,
			historySizeLimitWarn:  10,
			historySizeLimitError: 20,

			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				logEntries := logs.All()
				require.Len(t, logEntries, 1)
				assert.Equal(t, "history size exceeds warn limit.", logEntries[0].Message)
			},
		},
		"count error": {
			historyCount:           25,
			historyCountLimitWarn:  10,
			historyCountLimitError: 20,

			historySize:           1,
			historySizeLimitWarn:  10,
			historySizeLimitError: 20,

			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				logEntries := logs.All()
				require.Len(t, logEntries, 1)
				assert.Equal(t, "history size exceeds error limit.", logEntries[0].Message)
			},
			expectFail: true,
		},
		"size warn": {
			historyCount:           1,
			historyCountLimitWarn:  10,
			historyCountLimitError: 20,

			historySize:           15,
			historySizeLimitWarn:  10,
			historySizeLimitError: 20,

			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				logEntries := logs.All()
				require.Len(t, logEntries, 1)
				assert.Equal(t, "history size exceeds warn limit.", logEntries[0].Message)
			},
		},
		"size error": {
			historyCount:           1,
			historyCountLimitWarn:  10,
			historyCountLimitError: 20,

			historySize:           25,
			historySizeLimitWarn:  10,
			historySizeLimitError: 20,

			assertLogsAndMetrics: func(t *testing.T, logs *observer.ObservedLogs, scope tally.TestScope) {
				logEntries := logs.All()
				require.Len(t, logEntries, 1)
				assert.Equal(t, "history size exceeds error limit.", logEntries[0].Message)
			},
			expectFail: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mutableState := execution.NewMockMutableState(ctrl)
			logger, logs := testlogger.NewObserved(t)
			metricsScope := tally.NewTestScope("test", nil)

			mutableState.EXPECT().GetNextEventID().Return(int64(tc.historyCount + 1)).Times(1)
			if !tc.noExecutionCall {
				mutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
					DomainID:   testDomainID,
					WorkflowID: testWorkflowID,
					RunID:      testRunID,
				}).Times(1)
			}
			if tc.expectFail {
				mutableState.EXPECT().AddFailWorkflowEvent(testEventID, &types.FailWorkflowExecutionDecisionAttributes{
					Reason:  common.StringPtr(common.FailureReasonSizeExceedsLimit),
					Details: []byte("Workflow history size / count exceeds limit."),
				}).Return(nil, nil).Times(1)
			}

			checker := &workflowSizeChecker{
				completedID:            testEventID,
				historyCountLimitWarn:  tc.historyCountLimitWarn,
				historyCountLimitError: tc.historyCountLimitError,
				historySizeLimitWarn:   tc.historySizeLimitWarn,
				historySizeLimitError:  tc.historySizeLimitError,
				mutableState:           mutableState,
				executionStats: &persistence.ExecutionStats{
					HistorySize: int64(tc.historySize),
				},
				logger:       logger,
				metricsScope: metrics.NewClient(metricsScope, metrics.History, metrics.HistogramMigration{}).Scope(metrics.HistoryRespondDecisionTaskCompletedScope, metrics.DomainTag(testDomainName)),
			}
			failed, err := checker.failWorkflowSizeExceedsLimit()
			require.NoError(t, err)
			if tc.assertLogsAndMetrics != nil {
				tc.assertLogsAndMetrics(t, logs, metricsScope)
			}
			assert.Equal(t, tc.expectFail, failed)
		})
	}
}
