// Copyright (c) 2019 Uber Technologies, Inc.
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

package ndc

import (
	ctx "context"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"golang.org/x/net/context"

	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/shard"
)

type (
	conflictResolverSuite struct {
		suite.Suite
		*require.Assertions

		controller       *gomock.Controller
		mockShard        *shard.TestContext
		mockContext      *execution.MockContext
		mockMutableState *execution.MockMutableState
		mockStateBuilder *execution.MockStateRebuilder

		logger log.Logger

		domainID   string
		domainName string
		workflowID string
		runID      string

		nDCConflictResolver *conflictResolverImpl
	}
)

func TestConflictResolverSuite(t *testing.T) {
	s := new(conflictResolverSuite)
	suite.Run(t, s)
}

func (s *conflictResolverSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())
	s.mockContext = execution.NewMockContext(s.controller)
	s.mockMutableState = execution.NewMockMutableState(s.controller)
	s.mockStateBuilder = execution.NewMockStateRebuilder(s.controller)

	s.mockShard = shard.NewTestContext(
		s.T(),
		s.controller,
		&persistence.ShardInfo{
			ShardID:          10,
			RangeID:          1,
			TransferAckLevel: 0,
		},
		config.NewForTest(),
	)

	s.logger = s.mockShard.GetLogger()

	s.domainID = uuid.New()
	s.domainName = "some random domain name"
	s.workflowID = "some random workflow ID"
	s.runID = uuid.New()

	s.nDCConflictResolver = newConflictResolver(
		s.mockShard, s.mockContext, s.mockMutableState, s.logger,
	).(*conflictResolverImpl)
	s.nDCConflictResolver.stateRebuilder = s.mockStateBuilder
}

func (s *conflictResolverSuite) TearDownTest() {
	s.controller.Finish()
	s.mockShard.Finish(s.T())
}

func (s *conflictResolverSuite) TestRebuild() {
	ctx := ctx.Background()
	updateCondition := int64(59)
	requestID := uuid.New()
	version := int64(12)
	historySize := int64(12345)

	branchToken0 := []byte("some random branch token")
	lastEventID0 := int64(5)
	versionHistory0 := persistence.NewVersionHistory(
		branchToken0,
		[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(lastEventID0, version)},
	)
	branchToken1 := []byte("other random branch token")
	lastEventID1 := int64(2)
	versionHistory1 := persistence.NewVersionHistory(
		branchToken1,
		[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(lastEventID1, version)},
	)
	versionHistories := persistence.NewVersionHistories(versionHistory0)
	_, _, err := versionHistories.AddVersionHistory(versionHistory1)
	s.NoError(err)

	s.mockMutableState.EXPECT().GetUpdateCondition().Return(updateCondition).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   s.domainID,
		WorkflowID: s.workflowID,
		RunID:      s.runID,
	}).AnyTimes()

	workflowIdentifier := definition.NewWorkflowIdentifier(
		s.domainID,
		s.workflowID,
		s.runID,
	)
	mockRebuildMutableState := execution.NewMockMutableState(s.controller)
	mockRebuildMutableState.EXPECT().GetVersionHistories().Return(
		persistence.NewVersionHistories(
			persistence.NewVersionHistory(
				branchToken1,
				[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(lastEventID1, version)},
			),
		),
	).Times(1)
	mockRebuildMutableState.EXPECT().SetVersionHistories(versionHistories).Return(nil).Times(1)
	mockRebuildMutableState.EXPECT().SetUpdateCondition(updateCondition).Times(1)

	s.mockStateBuilder.EXPECT().Rebuild(
		ctx,
		gomock.Any(),
		workflowIdentifier,
		branchToken1,
		lastEventID1,
		version,
		workflowIdentifier,
		gomock.Any(),
		requestID,
	).DoAndReturn(func(ctx context.Context, now time.Time, baseWorkflowIdentifier definition.WorkflowIdentifier, baseBranchToken []byte, baseRebuildLastEventID int64, baseRebuildLastEventVersion int64, targetWorkflowIdentifier definition.WorkflowIdentifier, targetBranchFn func() ([]byte, error), requestID string) (execution.MutableState, int64, error) {
		targetBranchToken, err := targetBranchFn()
		s.NoError(err)
		s.Equal(branchToken1, targetBranchToken)

		return mockRebuildMutableState, historySize, nil
	}).Times(1)

	s.mockContext.EXPECT().Clear().Times(1)
	s.mockContext.EXPECT().SetHistorySize(historySize).Times(1)
	rebuiltMutableState, err := s.nDCConflictResolver.rebuild(ctx, 1, requestID)
	s.NoError(err)
	s.NotNil(rebuiltMutableState)
	s.Equal(1, versionHistories.GetCurrentVersionHistoryIndex())
}

func (s *conflictResolverSuite) TestPrepareMutableState_NoRebuild() {
	branchToken := []byte("some random branch token")
	lastEventID := int64(2)
	version := int64(12)
	versionHistoryItem := persistence.NewVersionHistoryItem(lastEventID, version)
	versionHistory := persistence.NewVersionHistory(
		branchToken,
		[]*persistence.VersionHistoryItem{versionHistoryItem},
	)
	versionHistories := persistence.NewVersionHistories(versionHistory)
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()

	rebuiltMutableState, isRebuilt, err := s.nDCConflictResolver.prepareMutableState(ctx.Background(), 0, version)
	s.NoError(err)
	s.False(isRebuilt)
	s.Equal(s.mockMutableState, rebuiltMutableState)
}

func (s *conflictResolverSuite) TestPrepareMutableState_Rebuild() {
	ctx := ctx.Background()
	updateCondition := int64(59)
	version := int64(12)
	incomingVersion := version + 1
	historySize := int64(12345)

	// current branch
	branchToken0 := []byte("some random branch token")
	lastEventID0 := int64(2)

	versionHistoryItem0 := persistence.NewVersionHistoryItem(lastEventID0, version)
	versionHistory0 := persistence.NewVersionHistory(
		branchToken0,
		[]*persistence.VersionHistoryItem{versionHistoryItem0},
	)

	// stale branch, used for rebuild
	branchToken1 := []byte("other random branch token")
	lastEventID1 := lastEventID0 - 1
	versionHistoryItem1 := persistence.NewVersionHistoryItem(lastEventID1, version)
	versionHistory1 := persistence.NewVersionHistory(
		branchToken1,
		[]*persistence.VersionHistoryItem{versionHistoryItem1},
	)

	versionHistories := persistence.NewVersionHistories(versionHistory0)
	_, _, err := versionHistories.AddVersionHistory(versionHistory1)
	s.Nil(err)

	s.mockMutableState.EXPECT().GetUpdateCondition().Return(updateCondition).AnyTimes()
	s.mockMutableState.EXPECT().GetVersionHistories().Return(versionHistories).AnyTimes()
	s.mockMutableState.EXPECT().GetExecutionInfo().Return(&persistence.WorkflowExecutionInfo{
		DomainID:   s.domainID,
		WorkflowID: s.workflowID,
		RunID:      s.runID,
	}).AnyTimes()

	workflowIdentifier := definition.NewWorkflowIdentifier(
		s.domainID,
		s.workflowID,
		s.runID,
	)
	mockRebuildMutableState := execution.NewMockMutableState(s.controller)
	mockRebuildMutableState.EXPECT().GetVersionHistories().Return(
		persistence.NewVersionHistories(
			persistence.NewVersionHistory(
				branchToken1,
				[]*persistence.VersionHistoryItem{persistence.NewVersionHistoryItem(lastEventID1, version)},
			),
		),
	).Times(1)
	mockRebuildMutableState.EXPECT().SetVersionHistories(versionHistories).Return(nil).Times(1)
	mockRebuildMutableState.EXPECT().SetUpdateCondition(updateCondition).Times(1)

	s.mockStateBuilder.EXPECT().Rebuild(
		ctx,
		gomock.Any(),
		workflowIdentifier,
		branchToken1,
		lastEventID1,
		version,
		workflowIdentifier,
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(ctx context.Context, now time.Time, baseWorkflowIdentifier definition.WorkflowIdentifier, baseBranchToken []byte, baseRebuildLastEventID int64, baseRebuildLastEventVersion int64, targetWorkflowIdentifier definition.WorkflowIdentifier, targetBranchFn func() ([]byte, error), requestID string) (execution.MutableState, int64, error) {
		targetBranchToken, err := targetBranchFn()
		s.NoError(err)
		s.Equal(branchToken1, targetBranchToken)

		return mockRebuildMutableState, historySize, nil
	}).Times(1)

	s.mockContext.EXPECT().Clear().Times(1)
	s.mockContext.EXPECT().SetHistorySize(int64(historySize)).Times(1)
	rebuiltMutableState, isRebuilt, err := s.nDCConflictResolver.prepareMutableState(ctx, 1, incomingVersion)
	s.NoError(err)
	s.NotNil(rebuiltMutableState)
	s.True(isRebuilt)
}
