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

package cli

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminDBScanErrorCases(t *testing.T) {
	cases := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "scan type not provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app)
			},
			errContains: "unknown scan type",
		},
		{
			name: "unknown scan type provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "some_unknown_scan_type"),
				)
			},
			errContains: "unknown scan type",
		},
		{
			name: "number of shards not provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
				)
			},
			errContains: "Required flag not found",
		},
		{
			name: "invariant collection not provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
				)
			},
			errContains: "no invariants for scan type \"ConcreteExecutionType\" and collections []",
		},
		{
			name: "invalid invariant collection provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "some_unknown_invariant_collection"),
				)
			},
			errContains: "unknown invariant collection: some_unknown_invariant_collection",
		},
		{
			name: "input file not found",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "CollectionHistory"),
					clitest.StringArgument("input_file", "testdata/non-existant-file.json"),
				)
			},
			errContains: "Input file not found",
		},
		{
			name: "input file is empty",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "CollectionHistory"),
					clitest.StringArgument("input_file", "testdata/scan_input_empty.json"),
				)
			},
			errContains: "Input file contained no data to scan",
		},
		{
			name: "bad data in input file",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "CollectionHistory"),
					clitest.StringArgument("input_file", "testdata/scan_input_bad_data.json"),
				)
			},
			errContains: "Error decoding input file",
		},
		{
			name: "execution manager initialization error",
			testSetup: func(td *cliTestData) *cli.Context {
				td.mockManagerFactory.EXPECT().
					initializeExecutionManager(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)

				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "CollectionHistory"),
					clitest.StringArgument("input_file", "testdata/scan_input.json"),
				)
			},
			errContains: "Execution check failed: initialize execution manager: assert.AnError general error for testing",
		},
		{
			name: "historyV2 manager initialization error",
			testSetup: func(td *cliTestData) *cli.Context {
				shardID1 := common.WorkflowIDToHistoryShard("test-workflow-id1", 16384)
				mockExecutionManager := persistence.NewMockExecutionManager(td.ctrl)
				mockExecutionManager.EXPECT().Close()
				td.mockManagerFactory.EXPECT().
					initializeExecutionManager(gomock.Any(), shardID1).
					Return(mockExecutionManager, nil)

				td.mockManagerFactory.EXPECT().
					initializeHistoryManager(gomock.Any()).
					Return(nil, assert.AnError)

				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "CollectionHistory"),
					clitest.StringArgument("input_file", "testdata/scan_input.json"),
				)
			},
			errContains: "Execution check failed: initialize history manager: assert.AnError general error for testing",
		},
		{
			name: "failed to fetch execution",
			testSetup: func(td *cliTestData) *cli.Context {
				shardID1 := common.WorkflowIDToHistoryShard("test-workflow-id1", 16384)
				mockExecutionManager := persistence.NewMockExecutionManager(td.ctrl)
				mockExecutionManager.EXPECT().Close()
				td.mockManagerFactory.EXPECT().
					initializeExecutionManager(gomock.Any(), shardID1).
					Return(mockExecutionManager, nil)

				mockHistoryManager := persistence.NewMockHistoryManager(td.ctrl)
				mockHistoryManager.EXPECT().Close()
				td.mockManagerFactory.EXPECT().
					initializeHistoryManager(gomock.Any()).
					Return(mockHistoryManager, nil)

				mockExecutionManager.EXPECT().GetWorkflowExecution(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)

				return clitest.NewCLIContext(t, td.app,
					clitest.StringArgument("scan_type", "ConcreteExecutionType"),
					clitest.IntArgument("number_of_shards", 16384),
					clitest.StringSliceArgument("invariant_collection", "CollectionHistory"),
					clitest.StringArgument("input_file", "testdata/scan_input.json"),
				)
			},
			errContains: "Execution check failed: fetching execution: assert.AnError general error for testing",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			td := newCLITestData(t)
			testCtx := tc.testSetup(td)
			err := AdminDBScan(testCtx)
			if tc.errContains != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAdminDBScan(t *testing.T) {
	td := newCLITestData(t)

	expectWorkFlow(td, "test-workflow-id1")
	expectWorkFlow(td, "test-workflow-id2")
	expectWorkFlow(td, "test-workflow-id3")

	cliCtx := clitest.NewCLIContext(t, td.app,
		clitest.StringArgument("scan_type", "CurrentExecutionType"),
		clitest.IntArgument("number_of_shards", 16384),
		clitest.StringSliceArgument("invariant_collection", "CollectionMutableState"),
		clitest.StringArgument("input_file", "testdata/scan_input.json"),
	)

	err := AdminDBScan(cliCtx)
	assert.NoError(t, err)

	assert.Equal(t, expectedAdminDBScanOutput, td.ioHandler.outputBytes.String())
}

// The expected output does not have any newlines or tabs
// so we use strings.Join(strings.Fields()) to remove them
var expectedAdminDBScanOutput = strings.Join(strings.Fields(`
{
	"Execution":{
		"CurrentRunID":"test-run-id1",
		"ShardID":8946,
		"DomainID":"test-domain-id1",
		"WorkflowID":"test-workflow-id1",
		"RunID":"test-run-id1",
		"State":2
	},
	"Result":{
		"CheckResultType":"healthy",
		"DeterminingInvariantType":null,
		"CheckResults":[{"CheckResultType":"healthy","InvariantName":"concrete_execution_exists","Info":"","InfoDetails":""}]
	}
}{
	"Execution":{
		"CurrentRunID":"test-run-id1",
		"ShardID":14767,
		"DomainID":"test-domain-id2",
		"WorkflowID":"test-workflow-id2",
		"RunID":"test-run-id1",
		"State":2
	},
	"Result":{
		"CheckResultType":"healthy",
		"DeterminingInvariantType":null,
		"CheckResults":[{"CheckResultType":"healthy","InvariantName":"concrete_execution_exists","Info":"","InfoDetails":""}]
	}
}{
	"Execution":{
		"CurrentRunID":"test-run-id1",
		"ShardID":14582,
		"DomainID":"test-domain-id3",
		"WorkflowID":"test-workflow-id3",
		"RunID":"test-run-id1","State":2
	},
	"Result":{
		"CheckResultType":"healthy",
		"DeterminingInvariantType":null,
		"CheckResults":[{"CheckResultType":"healthy","InvariantName":"concrete_execution_exists","Info":"","InfoDetails":""}]
	}
}`,
), "")

func expectWorkFlow(td *cliTestData, workflowID string) {
	shardID1 := common.WorkflowIDToHistoryShard(workflowID, 16384)
	mockExecutionManager := persistence.NewMockExecutionManager(td.ctrl)
	mockExecutionManager.EXPECT().Close().Times(1)
	td.mockManagerFactory.EXPECT().
		initializeExecutionManager(gomock.Any(), shardID1).
		Return(mockExecutionManager, nil).
		Times(1)

	mockHistoryManager := persistence.NewMockHistoryManager(td.ctrl)
	mockHistoryManager.EXPECT().Close().Times(1)
	td.mockManagerFactory.EXPECT().
		initializeHistoryManager(gomock.Any()).
		Return(mockHistoryManager, nil).
		Times(1)

	mockExecutionManager.EXPECT().GetCurrentExecution(gomock.Any(), gomock.Any()).
		Return(&persistence.GetCurrentExecutionResponse{
			RunID: "test-run-id1",
			State: persistence.WorkflowStateCompleted,
		}, nil).
		Times(1)
	mockExecutionManager.EXPECT().GetShardID().Return(shardID1).Times(1)
	mockExecutionManager.EXPECT().IsWorkflowExecutionExists(gomock.Any(), gomock.Any()).
		Return(&persistence.IsWorkflowExecutionExistsResponse{
			Exists: true,
		}, nil).
		Times(1)
}

func TestAdminDBScanUnsupportedWorkflow(t *testing.T) {
	td := newCLITestData(t)

	outPutFile := createTempFileWithContent(t, "")

	expectShard(td, 123)
	expectShard(td, 124)
	expectShard(td, 125)

	cliCtx := clitest.NewCLIContext(t, td.app,
		clitest.StringArgument("output_filename", outPutFile),
		clitest.IntArgument("lower_shard_bound", 123),
		clitest.IntArgument("upper_shard_bound", 125),
	)

	err := AdminDBScanUnsupportedWorkflow(cliCtx)
	assert.NoError(t, err)

	actual, err := os.ReadFile(outPutFile)
	require.NoError(t, err)
	assert.Equal(t, expectedAdminDBScanUnsupportedOutput, string(actual))
}

func expectShard(td *cliTestData, shardID int) {
	mockExecutionManager := persistence.NewMockExecutionManager(td.ctrl)
	mockExecutionManager.EXPECT().Close().Times(1)

	// Return 2 executions in the first call and a page token
	mockExecutionManager.EXPECT().ListConcreteExecutions(gomock.Any(),
		&persistence.ListConcreteExecutionsRequest{
			PageSize:  1000,
			PageToken: nil,
		},
	).
		Return(&persistence.ListConcreteExecutionsResponse{
			Executions: []*persistence.ListConcreteExecutionsEntity{
				createListConcreteExecutionsEntity(1, shardID),
				createListConcreteExecutionsEntity(2, shardID),
			},
			PageToken: []byte("some-next-page-token"),
		}, nil).Times(1)

	// Return 1 execution in the second call and no page token
	mockExecutionManager.EXPECT().ListConcreteExecutions(gomock.Any(),
		&persistence.ListConcreteExecutionsRequest{
			PageSize:  1000,
			PageToken: []byte("some-next-page-token"),
		},
	).
		Return(&persistence.ListConcreteExecutionsResponse{
			Executions: []*persistence.ListConcreteExecutionsEntity{
				createListConcreteExecutionsEntity(3, shardID),
			},
			PageToken: nil,
		}, nil).Times(1)

	td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), shardID).
		Return(mockExecutionManager, nil).Times(1)
}

func createListConcreteExecutionsEntity(number int, shardID int) *persistence.ListConcreteExecutionsEntity {
	return &persistence.ListConcreteExecutionsEntity{
		ExecutionInfo: &persistence.WorkflowExecutionInfo{
			DomainID:   fmt.Sprintf("%d-test-domain-id%d", shardID, number),
			WorkflowID: fmt.Sprintf("%d-test-workflow-id%d", shardID, number),
			RunID:      fmt.Sprintf("%d-test-run-id%d", shardID, number),
		},
		VersionHistories: nil,
	}
}

const expectedAdminDBScanUnsupportedOutput = `cadence --address <host>:<port> --domain <123-test-domain-id1> workflow reset --wid 123-test-workflow-id1 --rid 123-test-run-id1 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <123-test-domain-id2> workflow reset --wid 123-test-workflow-id2 --rid 123-test-run-id2 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <123-test-domain-id3> workflow reset --wid 123-test-workflow-id3 --rid 123-test-run-id3 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <124-test-domain-id1> workflow reset --wid 124-test-workflow-id1 --rid 124-test-run-id1 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <124-test-domain-id2> workflow reset --wid 124-test-workflow-id2 --rid 124-test-run-id2 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <124-test-domain-id3> workflow reset --wid 124-test-workflow-id3 --rid 124-test-run-id3 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <125-test-domain-id1> workflow reset --wid 125-test-workflow-id1 --rid 125-test-run-id1 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <125-test-domain-id2> workflow reset --wid 125-test-workflow-id2 --rid 125-test-run-id2 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
cadence --address <host>:<port> --domain <125-test-domain-id3> workflow reset --wid 125-test-workflow-id3 --rid 125-test-run-id3 --reset_type LastDecisionCompleted --reason 'release 0.16 upgrade'
`
