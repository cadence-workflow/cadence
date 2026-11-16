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

package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/olivere/elastic"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

type (
	cliAppSuite struct {
		suite.Suite
		app                  *cli.App
		mockCtrl             *gomock.Controller
		serverFrontendClient *frontend.MockClient
		serverAdminClient    *admin.MockClient
		testIOHandler        *testIOHandler
	}

	testcase struct {
		name    string
		command string
		err     string
		mock    func()
	}
)

var _ ClientFactory = (*clientFactoryMock)(nil)

type clientFactoryMock struct {
	serverFrontendClient frontend.Client
	serverAdminClient    admin.Client
	config               *config.Config
}

func (m *clientFactoryMock) ServerFrontendClient(c *cli.Context) (frontend.Client, error) {
	return m.serverFrontendClient, nil
}

func (m *clientFactoryMock) ServerAdminClient(c *cli.Context) (admin.Client, error) {
	return m.serverAdminClient, nil
}

func (m *clientFactoryMock) ServerFrontendClientForMigration(c *cli.Context) (frontend.Client, error) {
	panic("not implemented")
}

func (m *clientFactoryMock) ServerAdminClientForMigration(c *cli.Context) (admin.Client, error) {
	panic("not implemented")
}

func (m *clientFactoryMock) ElasticSearchClient(c *cli.Context) (*elastic.Client, error) {
	panic("not implemented")
}

func (m *clientFactoryMock) ServerConfig(c *cli.Context) (*config.Config, error) {
	if m.config != nil {
		return m.config, nil
	}
	return nil, fmt.Errorf("config not set")
}

var commands = []string{
	"domain", "d",
	"workflow", "wf",
	"tasklist", "tl",
}

var domainName = "cli-test-domain"

// Implements IOHandler to be used for validation in tests
type testIOHandler struct {
	outputBytes bytes.Buffer
}

func (t *testIOHandler) Input() io.Reader {
	return os.Stdin
}

func (t *testIOHandler) Output() io.Writer {
	return &t.outputBytes
}

func (t *testIOHandler) Progress() io.Writer {
	return os.Stdout
}

func TestCLIAppSuite(t *testing.T) {
	s := new(cliAppSuite)
	suite.Run(t, s)
}

func (s *cliAppSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.serverFrontendClient = frontend.NewMockClient(s.mockCtrl)
	s.serverAdminClient = admin.NewMockClient(s.mockCtrl)
	s.testIOHandler = &testIOHandler{}
	s.app = NewCliApp(&clientFactoryMock{
		serverFrontendClient: s.serverFrontendClient,
		serverAdminClient:    s.serverAdminClient,
	}, WithIOHandler(s.testIOHandler))
}

func (s *cliAppSuite) TearDownTest() {
	s.mockCtrl.Finish() // assert mock’s expectations
}

func (s *cliAppSuite) SetupSubTest() {
	// this is required to re-establish mocks on subtest-level
	// otherwise you will confusingly see expectations from previous test-cases on errors
	s.SetupTest()
}

func (s *cliAppSuite) TearDownSubTest() {
	s.TearDownTest()
}

func (s *cliAppSuite) runTestCase(tt testcase) {
	if tt.mock != nil {
		tt.mock()
	}

	err := clitest.RunCommandLine(s.T(), s.app, tt.command)
	if tt.err != "" {
		assert.ErrorContains(s.T(), err, tt.err)
	} else {
		assert.NoError(s.T(), err)
	}
}

func (s *cliAppSuite) TestAppCommands() {
	for _, test := range commands {
		cmd := s.app.Command(test)
		s.NotNil(cmd)
	}
}

var describeDomainResponseServer = &types.DescribeDomainResponse{
	DomainInfo: &types.DomainInfo{
		Name:        "test-domain",
		Description: "a test domain",
		OwnerEmail:  "test@uber.com",
	},
	Configuration: &types.DomainConfiguration{
		WorkflowExecutionRetentionPeriodInDays: 3,
		EmitMetric:                             true,
	},
	ReplicationConfiguration: &types.DomainReplicationConfiguration{
		ActiveClusterName: "active",
		Clusters: []*types.ClusterReplicationConfiguration{
			{
				ClusterName: "active",
			},
			{
				ClusterName: "standby",
			},
		},
	},
}

// This is used to mock the domain deletion confirmation
func (s *cliAppSuite) mockStdinInput(input string) func() {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	s.Require().NoError(err)
	os.Stdin = r

	go func() {
		_, wrErr := w.Write([]byte(input + "\n"))
		s.Require().NoError(wrErr)
		err = w.Close()
		s.Require().NoError(err)
	}()

	return func() {
		os.Stdin = oldStdin
		r.Close()
	}
}

func (s *cliAppSuite) TestDomainDelete() {
	resp := describeDomainResponseServer
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil)
	s.serverFrontendClient.EXPECT().DeleteDomain(gomock.Any(), gomock.Any()).Return(nil)

	defer s.mockStdinInput("Yes")()
	err := s.app.Run([]string{"", "--do", domainName, "domain", "delete", "--st", "test-token"})
	s.Nil(err)
}

func (s *cliAppSuite) TestDomainDelete_DomainNotExist() {
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(nil, &types.EntityNotExistsError{})
	s.Error(s.app.Run([]string{"", "--do", domainName, "domain", "delete", "--st", "test-token"}))
}

func (s *cliAppSuite) TestDomainDelete_Failed() {
	resp := describeDomainResponseServer
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil)
	s.serverFrontendClient.EXPECT().DeleteDomain(gomock.Any(), gomock.Any()).Return(&types.BadRequestError{"mock error"})

	defer s.mockStdinInput("Yes")()
	s.Error(s.app.Run([]string{"", "--do", domainName, "domain", "delete", "--st", "test-token"}))
}

func (s *cliAppSuite) TestDomainDelete_Cancelled() {
	resp := describeDomainResponseServer
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	defer s.mockStdinInput("No")()
	err := s.app.Run([]string{"", "--do", domainName, "domain", "delete", "--st", "test-token"})
	s.Nil(err)

	w.Close()
	var stdoutBuf bytes.Buffer
	io.Copy(&stdoutBuf, r)

	s.Contains(stdoutBuf.String(), "Domain deletion cancelled")
}

func (s *cliAppSuite) TestDomainDeprecate() {
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.StartWorkflowExecutionResponse{
		RunID: "run-id-example",
	}, nil)
	err := s.app.Run([]string{"", "--do", domainName, "domain", "deprecate", "--st", "secretToken"})
	s.Nil(err)
}

func (s *cliAppSuite) TestDomainDeprecate_FailedToStartDeprecationWorkflow() {
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.BadRequestError{"mock error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "domain", "deprecate", "--st", "secretToken"}))
}

func (s *cliAppSuite) TestDomainDescribe() {
	resp := describeDomainResponseServer
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "domain", "describe"})
	s.Nil(err)
}

func (s *cliAppSuite) TestDomainDescribe_DomainNotExist() {
	resp := describeDomainResponseServer
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, &types.EntityNotExistsError{})
	s.Error(s.app.Run([]string{"", "--do", domainName, "domain", "describe"}))
}

func (s *cliAppSuite) TestDomainDescribe_Failed() {
	resp := describeDomainResponseServer
	s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(resp, &types.BadRequestError{"faked error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "domain", "describe"}))
}

var (
	eventType = types.EventTypeWorkflowExecutionStarted

	getWorkflowExecutionHistoryResponse = &types.GetWorkflowExecutionHistoryResponse{
		History: &types.History{
			Events: []*types.HistoryEvent{
				{
					EventType: &eventType,
					WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
						WorkflowType:                        &types.WorkflowType{Name: "TestWorkflow"},
						TaskList:                            &types.TaskList{Name: "taskList"},
						ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
						TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
						Identity:                            "tester",
					},
				},
			},
		},
		NextPageToken: nil,
	}
)

func (s *cliAppSuite) TestShowHistory() {
	resp := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(resp, nil)
	describeResp := &types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}
	s.serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(describeResp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "show", "-w", "wid"})
	s.Nil(err)
}

func (s *cliAppSuite) TestShowHistoryWithID() {
	resp := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(resp, nil)
	describeResp := &types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}
	s.serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(describeResp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "showid", "wid"})
	s.Nil(err)
}

func (s *cliAppSuite) TestShowHistoryWithQueryConsistencyLevel() {
	resp := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().
		GetWorkflowExecutionHistory(gomock.Any(), &types.GetWorkflowExecutionHistoryRequest{
			Domain: domainName,
			Execution: &types.WorkflowExecution{
				WorkflowID: "wid",
			},
			HistoryEventFilterType: types.HistoryEventFilterTypeAllEvent.Ptr(),
			QueryConsistencyLevel:  types.QueryConsistencyLevelStrong.Ptr(),
		}).
		Return(resp, nil)
	describeResp := &types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}
	s.serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(describeResp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "show", "-w", "wid", "--query_consistency_level", "strong"})
	s.Nil(err)
}

func (s *cliAppSuite) TestShowHistoryWithInvalidQueryConsistencyLevel() {
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "show", "-w", "wid", "--query_consistency_level", "invalid"})
	s.Error(err)
	s.Contains(err.Error(), "invalid query consistency level")
}

func (s *cliAppSuite) TestShowHistory_PrintRawTime() {
	resp := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(resp, nil)
	describeResp := &types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}
	s.serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(describeResp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "show", "-w", "wid", "-prt"})
	s.Nil(err)
}

func (s *cliAppSuite) TestShowHistory_PrintDateTime() {
	resp := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(resp, nil)
	describeResp := &types.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &types.WorkflowExecutionInfo{},
	}
	s.serverFrontendClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(describeResp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "show", "-w", "wid", "-pdt"})
	s.Nil(err)
}

func (s *cliAppSuite) TestRestartWorkflow() {
	resp := &types.RestartWorkflowExecutionResponse{RunID: uuid.New()}
	s.serverFrontendClient.EXPECT().RestartWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "restart", "-w", "wid"})
	s.Nil(err)
}

func (s *cliAppSuite) TestRestartWorkflow_Failed() {
	resp := &types.RestartWorkflowExecutionResponse{RunID: uuid.New()}
	s.serverFrontendClient.EXPECT().RestartWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, &types.BadRequestError{"faked error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "restart", "-w", "wid"}))
}

func (s *cliAppSuite) TestDiagnoseWorkflow() {
	resp := &types.DiagnoseWorkflowExecutionResponse{Domain: "test", DiagnosticWorkflowExecution: &types.WorkflowExecution{WorkflowID: "123", RunID: uuid.New()}}
	s.serverFrontendClient.EXPECT().DiagnoseWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil).Times(1)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "diagnose", "-w", "wid", "-r", "rid"})
	s.Nil(err)
}

func (s *cliAppSuite) TestDiagnoseWorkflow_Failed() {
	s.serverFrontendClient.EXPECT().DiagnoseWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.BadRequestError{"faked error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "diagnose", "-w", "wid", "-r", "rid"}))
}

func (s *cliAppSuite) TestStartWorkflow() {
	resp := &types.StartWorkflowExecutionResponse{RunID: uuid.New()}
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil).Times(2)
	// start with wid
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "start", "-tl", "testTaskList", "-wt", "testWorkflowType", "-et", "60", "-w", "wid", "wrp", "2"})
	s.Nil(err)
	// start without wid
	err = s.app.Run([]string{"", "--do", domainName, "workflow", "start", "-tl", "testTaskList", "-wt", "testWorkflowType", "-et", "60", "wrp", "2"})
	s.Nil(err)
}

func (s *cliAppSuite) TestStartWorkflow_Failed() {
	resp := &types.StartWorkflowExecutionResponse{RunID: uuid.New()}
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, &types.BadRequestError{"faked error"})
	// start with wid
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "start", "-tl", "testTaskList", "-wt", "testWorkflowType", "-et", "60", "-w", "wid"}))
}

func (s *cliAppSuite) TestRunWorkflow() {
	resp := &types.StartWorkflowExecutionResponse{RunID: uuid.New()}
	history := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil).Times(2)
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(history, nil).Times(2)
	// start with wid
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "run", "-tl", "testTaskList", "-wt", "testWorkflowType", "-et", "60", "-w", "wid", "wrp", "2"})
	s.Nil(err)
	// start without wid
	err = s.app.Run([]string{"", "--do", domainName, "workflow", "run", "-tl", "testTaskList", "-wt", "testWorkflowType", "-et", "60", "wrp", "2"})
	s.Nil(err)
}

func (s *cliAppSuite) TestRunWorkflow_Failed() {
	resp := &types.StartWorkflowExecutionResponse{RunID: uuid.New()}
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, &types.BadRequestError{"faked error"})
	// start with wid
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "run", "-tl", "testTaskList", "-wt", "testWorkflowType", "-et", "60", "-w", "wid"}))
}

func (s *cliAppSuite) TestTerminateWorkflow() {
	s.serverFrontendClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "terminate", "-w", "wid"})
	s.Nil(err)
}

func (s *cliAppSuite) TestTerminateWorkflow_Failed() {
	s.serverFrontendClient.EXPECT().TerminateWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.BadRequestError{"faked error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "terminate", "-w", "wid"}))
}

func (s *cliAppSuite) TestCancelWorkflow() {
	s.serverFrontendClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "cancel", "-w", "wid"})
	s.Nil(err)
}

func (s *cliAppSuite) TestCancelWorkflow_Failed() {
	s.serverFrontendClient.EXPECT().RequestCancelWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.BadRequestError{"faked error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "cancel", "-w", "wid"}))
}

func (s *cliAppSuite) TestSignalWorkflow() {
	s.serverFrontendClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "signal", "-w", "wid", "-n", "signal-name"})
	s.Nil(err)
}

func (s *cliAppSuite) TestSignalWorkflow_Failed() {
	s.serverFrontendClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any()).Return(&types.BadRequestError{"faked error"})
	s.Error(s.app.Run([]string{"", "--do", domainName, "workflow", "signal", "-w", "wid", "-n", "signal-name"}))
}

func (s *cliAppSuite) TestQueryWorkflowUsingStackTrace() {
	resp := &types.QueryWorkflowResponse{
		QueryResult: []byte("query-result"),
	}
	s.serverFrontendClient.EXPECT().QueryWorkflow(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "stack", "-w", "wid"})
	s.Nil(err)
}

var (
	closeStatus = types.WorkflowExecutionCloseStatusCompleted

	listClosedWorkflowExecutionsResponse = &types.ListClosedWorkflowExecutionsResponse{
		Executions: []*types.WorkflowExecutionInfo{
			{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-list-workflow-id",
					RunID:      uuid.New(),
				},
				Type: &types.WorkflowType{
					Name: "test-list-workflow-type",
				},
				StartTime:     common.Int64Ptr(time.Now().UnixNano()),
				CloseTime:     common.Int64Ptr(time.Now().Add(time.Hour).UnixNano()),
				CloseStatus:   &closeStatus,
				HistoryLength: 12,
			},
		},
	}

	listOpenWorkflowExecutionsResponse = &types.ListOpenWorkflowExecutionsResponse{
		Executions: []*types.WorkflowExecutionInfo{
			{
				Execution: &types.WorkflowExecution{
					WorkflowID: "test-list-open-workflow-id",
					RunID:      uuid.New(),
				},
				Type: &types.WorkflowType{
					Name: "test-list-open-workflow-type",
				},
				StartTime:     common.Int64Ptr(time.Now().UnixNano()),
				CloseTime:     common.Int64Ptr(time.Now().Add(time.Hour).UnixNano()),
				HistoryLength: 12,
			},
		},
	}
)

func (s *cliAppSuite) TestListWorkflow() {
	resp := listClosedWorkflowExecutionsResponse
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_WithWorkflowID() {
	resp := &types.ListClosedWorkflowExecutionsResponse{}
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-wid", "nothing"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_WithWorkflowType() {
	resp := &types.ListClosedWorkflowExecutionsResponse{}
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-wt", "no-type"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_PrintDateTime() {
	resp := listClosedWorkflowExecutionsResponse
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-pdt"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_PrintRawTime() {
	resp := listClosedWorkflowExecutionsResponse
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-prt"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_Open() {
	resp := listOpenWorkflowExecutionsResponse
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-op"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_Open_WithWorkflowID() {
	resp := &types.ListOpenWorkflowExecutionsResponse{}
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-op", "-wid", "nothing"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListWorkflow_Open_WithWorkflowType() {
	resp := &types.ListOpenWorkflowExecutionsResponse{}
	countWorkflowResp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(countWorkflowResp, nil)
	s.serverFrontendClient.EXPECT().ListOpenWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "list", "-op", "-wt", "no-type"})
	s.Nil(err)
}

func (s *cliAppSuite) TestListArchivedWorkflow() {
	resp := &types.ListArchivedWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().ListArchivedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "listarchived", "-q", "some query string", "--ps", "200", "--all"})
	s.Nil(err)
}

func (s *cliAppSuite) TestCountWorkflow() {
	resp := &types.CountWorkflowExecutionsResponse{}
	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "count"})
	s.Nil(err)

	s.serverFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(resp, nil)
	err = s.app.Run([]string{"", "--do", domainName, "workflow", "count", "-q", "'CloseTime = missing'"})
	s.Nil(err)
}

var describeTaskListResponse = &types.DescribeTaskListResponse{
	Pollers: []*types.PollerInfo{
		{
			LastAccessTime: common.Int64Ptr(time.Now().UnixNano()),
			Identity:       "tester",
		},
	},
}

func (s *cliAppSuite) TestAdminDescribeWorkflow() {
	resp := &types.AdminDescribeWorkflowExecutionResponse{
		ShardID:                "test-shard-id",
		HistoryAddr:            "ip:port",
		MutableStateInDatabase: "{\"ExecutionInfo\":{\"BranchToken\":\"WQsACgAAACQ2MzI5YzEzMi1mMGI0LTQwZmUtYWYxMS1hODVmMDA3MzAzODQLABQAAAAkOWM5OWI1MjItMGEyZi00NTdmLWEyNDgtMWU0OTA0ZDg4YzVhDwAeDAAAAAAA\"}}",
	}

	s.serverAdminClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "admin", "wf", "describe", "-w", "test-wf-id"})
	s.Nil(err)
}

func (s *cliAppSuite) TestAdminDescribeWorkflow_Failed() {
	s.serverAdminClient.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, &types.BadRequestError{"faked error"})
	s.Error(s.app.Run(([]string{"", "--do", domainName, "admin", "wf", "describe", "-w", "test-wf-id"})))
}

func (s *cliAppSuite) TestAdminAddSearchAttribute() {
	var promptMsg string
	promptFn = func(msg string) {
		promptMsg = msg
	}
	request := &types.AddSearchAttributeRequest{
		SearchAttribute: map[string]types.IndexedValueType{
			"testKey": types.IndexedValueType(1),
		},
	}
	s.serverAdminClient.EXPECT().AddSearchAttribute(gomock.Any(), request).Return(nil)

	err := s.app.Run([]string{"", "--do", domainName, "admin", "cl", "asa", "--search_attr_key", "testKey", "--search_attr_type", "1"})
	s.Equal("Are you trying to add key [testKey] with Type [Keyword]? y/N", promptMsg)
	s.Nil(err)
}

func (s *cliAppSuite) TestAdminFailover() {
	resp := &types.StartWorkflowExecutionResponse{RunID: uuid.New()}
	s.serverFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).Return(resp, nil)
	s.serverFrontendClient.EXPECT().SignalWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	err := s.app.Run([]string{"", "admin", "cl", "fo", "start", "--tc", "standby", "--sc", "active"})
	s.Nil(err)
}

func (s *cliAppSuite) TestDescribeTaskList() {
	resp := describeTaskListResponse
	s.serverFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "tasklist", "describe", "-tl", "test-taskList"})
	s.Nil(err)
}

func (s *cliAppSuite) TestDescribeTaskList_Activity() {
	resp := describeTaskListResponse
	s.serverFrontendClient.EXPECT().DescribeTaskList(gomock.Any(), gomock.Any()).Return(resp, nil)
	err := s.app.Run([]string{"", "--do", domainName, "tasklist", "describe", "-tl", "test-taskList", "-tlt", "activity"})
	s.Nil(err)
}

func (s *cliAppSuite) TestObserveWorkflow() {
	history := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(history, nil).Times(2)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "observe", "-w", "wid"})
	s.Nil(err)
	err = s.app.Run([]string{"", "--do", domainName, "workflow", "observe", "-w", "wid", "-sd"})
	s.Nil(err)
}

func (s *cliAppSuite) TestObserveWorkflowWithID() {
	history := getWorkflowExecutionHistoryResponse
	s.serverFrontendClient.EXPECT().GetWorkflowExecutionHistory(gomock.Any(), gomock.Any()).Return(history, nil).Times(2)
	err := s.app.Run([]string{"", "--do", domainName, "workflow", "observeid", "wid"})
	s.Nil(err)
	err = s.app.Run([]string{"", "--do", domainName, "workflow", "observeid", "wid", "-sd"})
	s.Nil(err)
}

// TestParseTime tests the parsing of date argument in UTC and UnixNano formats
func (s *cliAppSuite) TestParseTime() {
	pt, err := parseTime("", 100)
	s.NoError(err)
	s.Equal(int64(100), pt)
	pt, err = parseTime("2018-06-07T15:04:05+00:00", 0)
	s.Equal(int64(1528383845000000000), pt)
	pt, err = parseTime("1528383845000000000", 0)
	s.Equal(int64(1528383845000000000), pt)
}

// TestParseTimeDateRange tests the parsing of date argument in time range format, N<duration>
// where N is the integral multiplier, and duration can be second/minute/hour/day/week/month/year
func (s *cliAppSuite) TestParseTimeDateRange() {
	tests := []struct {
		timeStr  string // input
		defVal   int64  // input
		expected int64  // expected unix nano (approx)
	}{
		{
			timeStr:  "1s",
			defVal:   int64(0),
			expected: time.Now().Add(-time.Second).UnixNano(),
		},
		{
			timeStr:  "100second",
			defVal:   int64(0),
			expected: time.Now().Add(-100 * time.Second).UnixNano(),
		},
		{
			timeStr:  "2m",
			defVal:   int64(0),
			expected: time.Now().Add(-2 * time.Minute).UnixNano(),
		},
		{
			timeStr:  "200minute",
			defVal:   int64(0),
			expected: time.Now().Add(-200 * time.Minute).UnixNano(),
		},
		{
			timeStr:  "3h",
			defVal:   int64(0),
			expected: time.Now().Add(-3 * time.Hour).UnixNano(),
		},
		{
			timeStr:  "1000hour",
			defVal:   int64(0),
			expected: time.Now().Add(-1000 * time.Hour).UnixNano(),
		},
		{
			timeStr:  "5d",
			defVal:   int64(0),
			expected: time.Now().Add(-5 * day).UnixNano(),
		},
		{
			timeStr:  "25day",
			defVal:   int64(0),
			expected: time.Now().Add(-25 * day).UnixNano(),
		},
		{
			timeStr:  "5w",
			defVal:   int64(0),
			expected: time.Now().Add(-5 * week).UnixNano(),
		},
		{
			timeStr:  "52week",
			defVal:   int64(0),
			expected: time.Now().Add(-52 * week).UnixNano(),
		},
		{
			timeStr:  "3M",
			defVal:   int64(0),
			expected: time.Now().Add(-3 * month).UnixNano(),
		},
		{
			timeStr:  "6month",
			defVal:   int64(0),
			expected: time.Now().Add(-6 * month).UnixNano(),
		},
		{
			timeStr:  "1y",
			defVal:   int64(0),
			expected: time.Now().Add(-year).UnixNano(),
		},
		{
			timeStr:  "7year",
			defVal:   int64(0),
			expected: time.Now().Add(-7 * year).UnixNano(),
		},
		{
			timeStr:  "100y", // epoch time will be returned as that's the minimum unix timestamp possible
			defVal:   int64(0),
			expected: time.Unix(0, 0).UnixNano(),
		},
	}
	delta := int64(50 * time.Millisecond)
	for _, te := range tests {
		pt, err := parseTime(te.timeStr, te.defVal)
		s.NoError(err)
		s.True(te.expected <= pt)
		pt, err = parseTime(te.timeStr, te.defVal)
		s.True(te.expected+delta >= pt)
	}
}

func (s *cliAppSuite) TestBreakLongWords() {
	s.Equal("111 222 333 4", breakLongWords("1112223334", 3))
	s.Equal("111 2 223", breakLongWords("1112 223", 3))
	s.Equal("11 122 23", breakLongWords("11 12223", 3))
	s.Equal("111", breakLongWords("111", 3))
	s.Equal("", breakLongWords("", 3))
	s.Equal("111  222", breakLongWords("111 222", 3))
}

func (s *cliAppSuite) TestAnyToString() {
	arg := strings.Repeat("LongText", 80)
	event := &types.HistoryEvent{
		ID:        1,
		EventType: &eventType,
		WorkflowExecutionStartedEventAttributes: &types.WorkflowExecutionStartedEventAttributes{
			WorkflowType:                        &types.WorkflowType{Name: "code.uber.internal/devexp/cadence-samples.git/cmd/samples/recipes/helloworld.Workflow"},
			TaskList:                            &types.TaskList{Name: "taskList"},
			ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(60),
			TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(10),
			Identity:                            "tester",
			Input:                               []byte(arg),
		},
	}
	res := anyToString(event, false, defaultMaxFieldLength)
	ss, l := tablewriter.WrapString(res, 10)
	s.Equal(9, len(ss))
	s.Equal(147, l)
}

func (s *cliAppSuite) TestAnyToString_DecodeMapValues() {
	fields := map[string][]byte{
		"TestKey": []byte("testValue"),
	}
	execution := &types.WorkflowExecutionInfo{
		Memo: &types.Memo{Fields: fields},
	}
	s.Equal("{HistoryLength:0, Memo:{Fields:map{TestKey:testValue}}, IsCron:false, PartitionConfig:map{}}", anyToString(execution, true, 0))

	fields["TestKey2"] = []byte(`anotherTestValue`)
	execution.Memo = &types.Memo{Fields: fields}
	got := anyToString(execution, true, 0)
	expected := got == "{HistoryLength:0, Memo:{Fields:map{TestKey2:anotherTestValue, TestKey:testValue}}, IsCron:false, PartitionConfig:map{}}" ||
		got == "{HistoryLength:0, Memo:{Fields:map{TestKey:testValue, TestKey2:anotherTestValue}}, IsCron:false, PartitionConfig:map{}}"
	s.True(expected)
}

func (s *cliAppSuite) TestIsAttributeName() {
	s.True(isAttributeName("WorkflowExecutionStartedEventAttributes"))
	s.False(isAttributeName("workflowExecutionStartedEventAttributes"))
}

func (s *cliAppSuite) TestGetWorkflowIdReusePolicy() {
	res, err := getWorkflowIDReusePolicy(2)
	s.NoError(err)
	s.Equal(res.String(), types.WorkflowIDReusePolicyRejectDuplicate.String())
}

func (s *cliAppSuite) TestGetWorkflowIdReusePolicy_Failed_ExceedRange() {
	_, err := getWorkflowIDReusePolicy(2147483647)
	s.Error(err)
}

func (s *cliAppSuite) TestGetWorkflowIdReusePolicy_Failed_Negative() {
	_, err := getWorkflowIDReusePolicy(-1)
	s.Error(err)
}

func (s *cliAppSuite) TestGetSearchAttributes() {
	resp := &types.GetSearchAttributesResponse{}
	s.serverFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(resp, nil).Times(2)
	err := s.app.Run([]string{"", "cluster", "get-search-attr"})
	s.Nil(err)
	err = s.app.Run([]string{"", "--do", domainName, "cluster", "get-search-attr"})
	s.Nil(err)
}

func (s *cliAppSuite) TestParseBool() {
	res, err := parseBool("true")
	s.NoError(err)
	s.True(res)

	res, err = parseBool("false")
	s.NoError(err)
	s.False(res)

	for _, v := range []string{"True, TRUE, False, FALSE, T, F"} {
		res, err = parseBool(v)
		s.Error(err)
		s.False(res)
	}
}

func (s *cliAppSuite) TestConvertStringToRealType() {
	var res interface{}

	// int
	res = convertStringToRealType("1")
	s.Equal(int64(1), res)

	// bool
	res = convertStringToRealType("true")
	s.Equal(true, res)
	res = convertStringToRealType("false")
	s.Equal(false, res)

	// double
	res = convertStringToRealType("1.0")
	s.Equal(float64(1.0), res)

	// datetime
	res = convertStringToRealType("2019-01-01T01:01:01Z")
	s.Equal(time.Date(2019, 1, 1, 1, 1, 1, 0, time.UTC), res)

	// array
	res = convertStringToRealType(`["a", "b", "c"]`)
	s.Equal([]interface{}{"a", "b", "c"}, res)

	// string
	res = convertStringToRealType("test string")
	s.Equal("test string", res)
}

func (s *cliAppSuite) TestConvertArray() {
	t1, _ := time.Parse(defaultDateTimeFormat, "2019-06-07T16:16:34-08:00")
	t2, _ := time.Parse(defaultDateTimeFormat, "2019-06-07T17:16:34-08:00")
	testCases := []struct {
		name     string
		input    string
		expected interface{}
	}{
		{
			name:     "string",
			input:    `["a", "b", "c"]`,
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "int",
			input:    `[1, 2, 3]`,
			expected: []interface{}{"1", "2", "3"},
		},
		{
			name:     "double",
			input:    `[1.1, 2.2, 3.3]`,
			expected: []interface{}{"1.1", "2.2", "3.3"},
		},
		{
			name:     "bool",
			input:    `["true", "false"]`,
			expected: []interface{}{"true", "false"},
		},
		{
			name:     "datetime",
			input:    `["2019-06-07T16:16:34-08:00", "2019-06-07T17:16:34-08:00"]`,
			expected: []interface{}{t1, t2},
		},
	}
	for _, testCase := range testCases {
		res, err := parseArray(testCase.input)
		s.Nil(err)
		s.Equal(testCase.expected, res)
	}

	testCases2 := []struct {
		name     string
		input    string
		expected error
	}{
		{
			name:  "not array",
			input: "normal string",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "not json array",
			input: "[a, b, c]",
		},
	}
	for _, testCase := range testCases2 {
		res, err := parseArray(testCase.input)
		s.NotNil(err)
		s.Nil(res)
	}
}

func TestWithManagerFactory(t *testing.T) {
	tests := []struct {
		name           string
		app            *cli.App
		expectFactory  bool
		expectNilMeta  bool
		expectDepsType bool
	}{
		{
			name: "Valid Metadata with deps key",
			app: &cli.App{
				Metadata: map[string]interface{}{
					depsKey: &deps{},
				},
			},
			expectFactory:  true,
			expectNilMeta:  false,
			expectDepsType: true,
		},
		{
			name:           "No Metadata",
			app:            &cli.App{Metadata: nil},
			expectFactory:  false,
			expectNilMeta:  true,
			expectDepsType: false,
		},
		{
			name: "Incorrect deps key type",
			app: &cli.App{
				Metadata: map[string]interface{}{
					depsKey: "invalid_type",
				},
			},
			expectFactory:  false,
			expectNilMeta:  false,
			expectDepsType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFactory := NewMockManagerFactory(ctrl)

			option := WithManagerFactory(mockFactory)
			option(tt.app)

			if tt.expectNilMeta {
				assert.Nil(t, tt.app.Metadata, "Expected Metadata to be nil")
			} else {
				d, ok := tt.app.Metadata[depsKey].(*deps)
				if tt.expectDepsType {
					assert.True(t, ok, "Expected depsKey to be of type *deps")
					if tt.expectFactory {
						assert.Equal(t, mockFactory, d.ManagerFactory, "Expected ManagerFactory to be set correctly")
					}
				} else {
					assert.False(t, ok, "Expected depsKey not to be of type *deps")
				}
			}
		})
	}
}

func TestWithIOHandler(t *testing.T) {
	tests := []struct {
		name           string
		app            *cli.App
		expectHandler  bool
		expectNilMeta  bool
		expectDepsType bool
	}{
		{
			name: "Valid Metadata with deps key",
			app: &cli.App{
				Metadata: map[string]interface{}{
					depsKey: &deps{},
				},
			},
			expectHandler:  true,
			expectNilMeta:  false,
			expectDepsType: true,
		},
		{
			name:           "No Metadata",
			app:            &cli.App{Metadata: nil},
			expectHandler:  false,
			expectNilMeta:  true,
			expectDepsType: false,
		},
		{
			name: "Incorrect deps key type",
			app: &cli.App{
				Metadata: map[string]interface{}{
					depsKey: "invalid_type",
				},
			},
			expectHandler:  false,
			expectNilMeta:  false,
			expectDepsType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHandler := testIOHandler{}

			option := WithIOHandler(&testHandler)
			option(tt.app)

			if tt.expectNilMeta {
				assert.Nil(t, tt.app.Metadata, "Expected Metadata to be nil")
			} else {
				d, ok := tt.app.Metadata[depsKey].(*deps)
				if tt.expectDepsType {
					assert.True(t, ok, "Expected depsKey to be of type *deps")
					if tt.expectHandler {
						assert.Equal(t, &testHandler, d.IOHandler, "Expected IOHandler to be set correctly")
					}
				} else {
					assert.False(t, ok, "Expected depsKey not to be of type *deps")
				}
			}
		})
	}
}
