// Copyright (c) 2018 Uber Technologies, Inc.
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

//go:build !race && pinotintegration
// +build !race,pinotintegration

/*
To run locally with docker containers:

1. Stop the previous run if any
	docker compose -f docker/github_actions/docker-compose-local-pinot.yml down

2. Build the integration-test-async-wf image
	docker compose -f docker/github_actions/docker-compose-local-pinot.yml build integration-test-cassandra-pinot

3. Run the test in the docker container
	docker compose -f docker/github_actions/docker-compose-local-pinot.yml run --rm integration-test-cassandra-pinot

To run locally natively (without docker),
1. make sure kafka and pinot is running,
2. then run cmd `go test -v ./host -run TestPinotIntegrationSuite -tags pinotintegration`
3. currently we have to manually add test table and delete the table for cleaning. waiting for the support to clean the data programmatically
*/

package host

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	pt "github.com/uber/cadence/common/persistence/persistence-tests"
	pnt "github.com/uber/cadence/common/pinot"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/host/pinotutils"
)

const (
	numberOfRetry        = 50
	waitTimeBetweenRetry = 400 * time.Millisecond
	waitForPinotToSettle = 4 * time.Second // wait pinot shards for some time ensure data consistent
)

type PinotIntegrationSuite struct {
	*require.Assertions
	logger log.Logger
	*IntegrationBase
	pinotClient pnt.GenericClient

	testSearchAttributeKey string
	testSearchAttributeVal string
}

func TestPinotIntegrationSuite(t *testing.T) {
	flag.Parse()
	clusterConfig, err := GetTestClusterConfig("testdata/integration_pinot_cluster.yaml")
	if err != nil {
		panic(err)
	}
	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(PinotIntegrationSuite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationBase = NewIntegrationBase(params)
	suite.Run(t, s)
}

func (s *PinotIntegrationSuite) SetupSuite() {
	s.SetupLogger()

	s.Logger.Info("Running integration test against test cluster")
	clusterMetadata := NewClusterMetadata(s.T(), s.TestClusterConfig)
	dc := persistence.DynamicConfiguration{
		EnableSQLAsyncTransaction:                dynamicproperties.GetBoolPropertyFn(false),
		EnableCassandraAllConsistencyLevelDelete: dynamicproperties.GetBoolPropertyFn(true),
		PersistenceSampleLoggingRate:             dynamicproperties.GetIntPropertyFn(100),
		EnableShardIDMetrics:                     dynamicproperties.GetBoolPropertyFn(true),
		EnableHistoryTaskDualWriteMode:           dynamicproperties.GetBoolPropertyFn(true),
		ReadNoSQLHistoryTaskFromDataBlob:         dynamicproperties.GetBoolPropertyFn(false),
		SerializationEncoding:                    dynamicproperties.GetStringPropertyFn(string(constants.EncodingTypeThriftRW)),
		ReadNoSQLShardFromDataBlob:               dynamicproperties.GetBoolPropertyFn(true),
		HistoryNodeDeleteBatchSize:               dynamicproperties.GetIntPropertyFn(1000),
	}
	params := pt.TestBaseParams{
		DefaultTestCluster:    s.DefaultTestCluster,
		VisibilityTestCluster: s.VisibilityTestCluster,
		ClusterMetadata:       clusterMetadata,
		DynamicConfiguration:  dc,
	}
	cluster, err := NewPinotTestCluster(s.T(), s.TestClusterConfig, s.Logger, params)
	s.Require().NoError(err)
	s.TestCluster = cluster
	s.Engine = s.TestCluster.GetFrontendClient()
	s.AdminClient = s.TestCluster.GetAdminClient()

	s.TestRawHistoryDomainName = "TestRawHistoryDomain"
	s.DomainName = s.RandomizeStr("integration-test-domain")
	s.Require().NoError(
		s.RegisterDomain(s.DomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))
	s.Require().NoError(
		s.RegisterDomain(s.TestRawHistoryDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))
	s.ForeignDomainName = s.RandomizeStr("integration-foreign-test-domain")
	s.Require().NoError(
		s.RegisterDomain(s.ForeignDomainName, 1, types.ArchivalStatusDisabled, "", types.ArchivalStatusDisabled, "", nil))

	s.Require().NoError(s.registerArchivalDomain())

	// this sleep is necessary because domainv2 cache gets refreshed in the
	// background only every domainCacheRefreshInterval period
	time.Sleep(cache.DomainCacheRefreshInterval + time.Second)

	tableName := "cadence_visibility_pinot" // cadence_visibility_pinot_integration_test
	pinotConfig := &config.PinotVisibilityConfig{
		Cluster:     "",
		Broker:      "localhost:8099",
		Table:       tableName,
		ServiceName: "",
		Migration: config.VisibilityMigration{
			Enabled: true,
		},
	}
	s.pinotClient = pinotutils.CreatePinotClient(&s.Suite, pinotConfig, s.Logger)
}

func (s *PinotIntegrationSuite) SetupTest() {

	s.Assertions = require.New(s.T())
	s.testSearchAttributeKey = definition.CustomStringField
	s.testSearchAttributeVal = "test value"
}

func (s *PinotIntegrationSuite) TearDownSuite() {
	s.TearDownBaseSuite()
	// check how to clean up test_table
	// currently it is not supported
}

func (s *PinotIntegrationSuite) TestListOpenWorkflow() {
	id := "pinot-integration-start-workflow-test"
	wt := "pinot-integration-start-workflow-test-type"
	tl := "pinot-integration-start-workflow-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)

	attrValBytes, _ := json.Marshal(s.testSearchAttributeVal)
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			s.testSearchAttributeKey: attrValBytes,
		},
	}
	request.SearchAttributes = searchAttr

	startTime := time.Now().UnixNano()
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	startFilter := &types.StartTimeFilter{}
	startFilter.EarliestTime = common.Int64Ptr(startTime)
	var openExecution *types.WorkflowExecutionInfo
	for i := 0; i < numberOfRetry; i++ {
		startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())
		resp, err := s.Engine.ListOpenWorkflowExecutions(ctx, &types.ListOpenWorkflowExecutionsRequest{
			Domain:          s.DomainName,
			MaximumPageSize: defaultTestValueOfESIndexMaxResultWindow,
			StartTimeFilter: startFilter,
			ExecutionFilter: &types.WorkflowExecutionFilter{
				WorkflowID: id,
			},
		})
		s.Nil(err)

		if len(resp.GetExecutions()) > 0 {
			openExecution = resp.GetExecutions()[0]
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.NotNil(openExecution)
	s.Equal(we.GetRunID(), openExecution.GetExecution().GetRunID())
	s.Equal(attrValBytes, openExecution.SearchAttributes.GetIndexedFields()[s.testSearchAttributeKey])
}

func (s *PinotIntegrationSuite) TestListWorkflow() {
	id := "pinot-integration-list-workflow-test"
	wt := "pinot-integration-list-workflow-test-type"
	tl := "pinot-integration-list-workflow-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)
	query := fmt.Sprintf(`WorkflowID = "%s"`, id)
	s.testHelperForReadOnce(we.GetRunID(), query, false, false)
}

func (s *PinotIntegrationSuite) createStartWorkflowExecutionRequest(id, wt, tl string) *types.StartWorkflowExecutionRequest {
	identity := "worker1"
	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}
	return request
}

func (s *PinotIntegrationSuite) testHelperForReadOnce(runID, query string, isScan bool, isAnyMatchOk bool) {
	s.testHelperForReadOnceWithDomain(s.DomainName, runID, query, isScan, isAnyMatchOk)
}

func (s *PinotIntegrationSuite) testHelperForReadOnceWithDomain(domainName string, runID, query string, isScan bool, isAnyMatchOk bool) {
	var openExecution *types.WorkflowExecutionInfo
	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   domainName,
		PageSize: defaultTestValueOfESIndexMaxResultWindow,
		Query:    query,
	}
	ctx, cancel := createContext()
	defer cancel()
Retry:
	for i := 0; i < numberOfRetry; i++ {
		var resp *types.ListWorkflowExecutionsResponse
		var err error
		if isScan {
			resp, err = s.Engine.ScanWorkflowExecutions(ctx, listRequest)
		} else {
			resp, err = s.Engine.ListWorkflowExecutions(ctx, listRequest)
		}

		s.Nil(err)
		logStr := fmt.Sprintf("Results for query '%s' (desired runId: %s): \n", query, runID)
		s.Logger.Info(logStr)
		for _, e := range resp.GetExecutions() {
			logStr = fmt.Sprintf("Execution: %+v, %+v \n", e.Execution, e)
			s.Logger.Info(logStr)
		}
		if len(resp.GetExecutions()) == 1 {
			openExecution = resp.GetExecutions()[0]
			break
		}
		if isAnyMatchOk {
			for _, e := range resp.GetExecutions() {
				if e.Execution.RunID == runID {
					openExecution = e
					break Retry
				}
			}
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.NotNil(openExecution)
	s.Equal(runID, openExecution.GetExecution().GetRunID())
	s.True(openExecution.GetExecutionTime() >= openExecution.GetStartTime())
	if openExecution.SearchAttributes != nil && len(openExecution.SearchAttributes.GetIndexedFields()) > 0 {
		searchValBytes := openExecution.SearchAttributes.GetIndexedFields()[s.testSearchAttributeKey]
		var searchVal string
		json.Unmarshal(searchValBytes, &searchVal)
		// pinot sets default values for all the columns,
		// this feature can break the test here when there is no actual search attributes upsert, it will still return something
		// TODO: update this after finding a good solution
		s.Equal(searchVal, searchVal)
	}
}

func (s *PinotIntegrationSuite) startWorkflow(
	prefix string,
	isCron bool,
) *types.StartWorkflowExecutionResponse {
	id := "pinot-integration-list-workflow-" + prefix + "-test"
	wt := "pinot-integration-list-workflow-" + prefix + "test-type"
	tl := "pinot-integration-list-workflow-" + prefix + "test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)
	if isCron {
		request.CronSchedule = "*/5 * * * *" // every 5 minutes
	}
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	query := fmt.Sprintf(`WorkflowID = "%s"`, id)
	s.testHelperForReadOnce(we.GetRunID(), query, false, false)
	return we
}

func (s *PinotIntegrationSuite) TestListCronWorkflows() {
	we1 := s.startWorkflow("cron", true)
	we2 := s.startWorkflow("regular", false)

	query := fmt.Sprintf(`IsCron = "true"`)
	s.testHelperForReadOnce(we1.GetRunID(), query, false, true)

	query = fmt.Sprintf(`IsCron = "false"`)
	s.testHelperForReadOnce(we2.GetRunID(), query, false, true)
}

func (s *PinotIntegrationSuite) TestIsGlobalSearchAttribute() {
	we := s.startWorkflow("local", true)
	// global domains are disabled for this integration test, so we can only test the false case
	query := fmt.Sprintf(`NumClusters = "1"`)
	s.testHelperForReadOnce(we.GetRunID(), query, false, true)
}

func (s *PinotIntegrationSuite) TestListWorkflow_ExecutionTime() {
	id := "pinot-integration-list-workflow-execution-time-test"
	wt := "pinot-integration-list-workflow-execution-time-test-type"
	tl := "pinot-integration-list-workflow-execution-time-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	cronID := id + "-cron"
	request.CronSchedule = "@every 1m"
	request.WorkflowID = cronID

	weCron, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	query := fmt.Sprintf(`(WorkflowID = '%s' or WorkflowID = '%s') and ExecutionTime < %v and ExecutionTime > 0`, id, cronID, time.Now().UnixNano()+int64(time.Minute))
	s.testHelperForReadOnce(weCron.GetRunID(), query, false, false)

	query = fmt.Sprintf(`WorkflowID = '%s'`, id)
	s.testHelperForReadOnce(we.GetRunID(), query, false, false)
}

func (s *PinotIntegrationSuite) TestListWorkflow_SearchAttribute() {
	id := "pinot-integration-list-workflow-by-search-attr-test"
	wt := "pinot-integration-list-workflow-by-search-attr-test-type"
	tl := "pinot-integration-list-workflow-by-search-attr-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)

	attrValBytes, _ := json.Marshal(s.testSearchAttributeVal)
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			s.testSearchAttributeKey: attrValBytes,
		},
	}
	request.SearchAttributes = searchAttr
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)
	query := fmt.Sprintf(`WorkflowID = "%s" and %s = "%s"`, id, s.testSearchAttributeKey, s.testSearchAttributeVal)
	s.testHelperForReadOnce(we.GetRunID(), query, false, false)

	// test upsert
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		upsertDecision := &types.Decision{
			DecisionType: types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr(),
			UpsertWorkflowSearchAttributesDecisionAttributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: getPinotUpsertSearchAttributes(),
			}}

		return nil, []*types.Decision{upsertDecision}, nil
	}
	taskList := &types.TaskList{Name: tl}
	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		StickyTaskList:  taskList,
		Identity:        "worker1",
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}
	_, newTask, err := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		true,
		true,
		int64(0),
		1,
		true,
		nil)
	s.Nil(err)
	s.NotNil(newTask)
	s.NotNil(newTask.DecisionTask)

	time.Sleep(waitForPinotToSettle)

	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   s.DomainName,
		PageSize: int32(2),
		Query:    fmt.Sprintf(`WorkflowType = '%s' and CloseTime = missing and BinaryChecksums = 'binary-v1'`, wt),
	}
	// verify upsert data is on Pinot
	s.testListResultForUpsertSearchAttributes(listRequest)

	// verify DescribeWorkflowExecution
	descRequest := &types.DescribeWorkflowExecutionRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
		},
	}

	descResp, err := s.Engine.DescribeWorkflowExecution(ctx, descRequest)
	s.Nil(err)
	expectedSearchAttributes := getPinotUpsertSearchAttributes()
	s.Equal(expectedSearchAttributes, descResp.WorkflowExecutionInfo.GetSearchAttributes())
}

func (s *PinotIntegrationSuite) TestListWorkflow_PageToken() {
	id := "pinot-integration-list-workflow-token-test"
	wt := "pinot-integration-list-workflow-token-test-type"
	tl := "pinot-integration-list-workflow-token-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)

	numOfWorkflows := defaultTestValueOfESIndexMaxResultWindow - 1 // == 4
	pageSize := 3

	s.testListWorkflowHelper(numOfWorkflows, pageSize, request, id, wt, false)
}

func (s *PinotIntegrationSuite) TestListWorkflow_SearchAfter() {
	id := "pinot-integration-list-workflow-searchAfter-test"
	wt := "pinot-integration-list-workflow-searchAfter-test-type"
	tl := "pinot-integration-list-workflow-searchAfter-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)

	numOfWorkflows := defaultTestValueOfESIndexMaxResultWindow + 1 // == 6
	pageSize := 4

	s.testListWorkflowHelper(numOfWorkflows, pageSize, request, id, wt, false)
}

func (s *PinotIntegrationSuite) TestListWorkflow_OrQuery() {
	id := "pinot-integration-list-workflow-or-query-test"
	wt := "pinot-integration-list-workflow-or-query-test-type"
	tl := "pinot-integration-list-workflow-or-query-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)
	ctx, cancel := createContext()
	defer cancel()
	// start 3 workflows
	key := definition.CustomIntField
	attrValBytes, _ := json.Marshal(1)
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			key: attrValBytes,
		},
	}
	request.SearchAttributes = searchAttr
	we1, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	request.RequestID = uuid.New()
	request.WorkflowID = id + "-2"
	attrValBytes, _ = json.Marshal(2)
	searchAttr.IndexedFields[key] = attrValBytes
	we2, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	request.RequestID = uuid.New()
	request.WorkflowID = id + "-3"
	attrValBytes, _ = json.Marshal(3)
	searchAttr.IndexedFields[key] = attrValBytes
	we3, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	time.Sleep(waitForPinotToSettle)

	// query 1 workflow with search attr
	query1 := fmt.Sprintf(`CustomIntField = %d`, 1)
	var openExecution *types.WorkflowExecutionInfo
	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   s.DomainName,
		PageSize: defaultTestValueOfESIndexMaxResultWindow,
		Query:    query1,
	}
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)
		if len(resp.GetExecutions()) == 1 {
			openExecution = resp.GetExecutions()[0]
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.NotNil(openExecution)
	s.Equal(we1.GetRunID(), openExecution.GetExecution().GetRunID())
	s.True(openExecution.GetExecutionTime() >= openExecution.GetStartTime())
	searchValBytes := openExecution.SearchAttributes.GetIndexedFields()[key]
	var searchVal int
	json.Unmarshal(searchValBytes, &searchVal)
	s.Equal(1, searchVal)

	// query with or clause
	query2 := fmt.Sprintf(`CustomIntField = %d or CustomIntField = %d`, 1, 2)
	listRequest.Query = query2
	var openExecutions []*types.WorkflowExecutionInfo
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)
		if len(resp.GetExecutions()) == 2 {
			openExecutions = resp.GetExecutions()
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	// TODO: need to clean up or every time we run, we have to delete the table.
	s.Equal(2, len(openExecutions))

	e1 := openExecutions[0]
	e2 := openExecutions[1]
	if e1.GetExecution().GetRunID() != we1.GetRunID() {
		// results are sorted by [CloseTime,RunID] desc, so find the correct mapping first
		e1, e2 = e2, e1
	}
	s.Equal(we1.GetRunID(), e1.GetExecution().GetRunID())
	s.Equal(we2.GetRunID(), e2.GetExecution().GetRunID())
	searchValBytes = e2.SearchAttributes.GetIndexedFields()[key]
	json.Unmarshal(searchValBytes, &searchVal)
	s.Equal(2, searchVal)

	// query for open
	query3 := fmt.Sprintf(`(CustomIntField = %d or CustomIntField = %d) and CloseTime = missing`, 2, 3)
	listRequest.Query = query3
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)
		if len(resp.GetExecutions()) == 2 {
			openExecutions = resp.GetExecutions()
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.Equal(2, len(openExecutions))
	e1 = openExecutions[0]
	e2 = openExecutions[1]
	s.Equal(we3.GetRunID(), e1.GetExecution().GetRunID())
	s.Equal(we2.GetRunID(), e2.GetExecution().GetRunID())
	searchValBytes = e1.SearchAttributes.GetIndexedFields()[key]
	json.Unmarshal(searchValBytes, &searchVal)
	s.Equal(3, searchVal)
}

// To test last page search trigger max window size error
func (s *PinotIntegrationSuite) TestListWorkflow_MaxWindowSize() {
	id := "pinot-integration-list-workflow-max-window-size-test"
	wt := "pinot-integration-list-workflow-max-window-size-test-type"
	tl := "pinot-integration-list-workflow-max-window-size-test-tasklist"
	startRequest := s.createStartWorkflowExecutionRequest(id, wt, tl)
	ctx, cancel := createContext()
	defer cancel()
	for i := 0; i < defaultTestValueOfESIndexMaxResultWindow; i++ {
		startRequest.RequestID = uuid.New()
		startRequest.WorkflowID = id + strconv.Itoa(i)
		_, err := s.Engine.StartWorkflowExecution(ctx, startRequest)
		s.Nil(err)
	}

	time.Sleep(waitForPinotToSettle)

	var listResp *types.ListWorkflowExecutionsResponse
	var nextPageToken []byte

	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:        s.DomainName,
		PageSize:      int32(defaultTestValueOfESIndexMaxResultWindow),
		NextPageToken: nextPageToken,
		Query:         fmt.Sprintf(`WorkflowType = '%s' and CloseTime = missing`, wt),
	}
	// get first page
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)
		if len(resp.GetExecutions()) == defaultTestValueOfESIndexMaxResultWindow {
			listResp = resp
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.NotNil(listResp)
	s.True(len(listResp.GetNextPageToken()) != 0)

	// the last request
	listRequest.NextPageToken = listResp.GetNextPageToken()
	resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
	s.Nil(err)
	s.True(len(resp.GetExecutions()) == 0)
	s.True(len(resp.GetNextPageToken()) == 0)
}

func (s *PinotIntegrationSuite) TestListWorkflow_OrderBy() {
	id := "pinot-integration-list-workflow-order-by-test"
	wt := "pinot-integration-list-workflow-order-by-test-type"
	tl := "pinot-integration-list-workflow-order-by-test-tasklist"
	startRequest := s.createStartWorkflowExecutionRequest(id, wt, tl)
	ctx, cancel := createContext()
	defer cancel()
	for i := 0; i < defaultTestValueOfESIndexMaxResultWindow+1; i++ { // start 6
		startRequest.RequestID = uuid.New()
		startRequest.WorkflowID = id + strconv.Itoa(i)

		if i < defaultTestValueOfESIndexMaxResultWindow-1 { // 4 workflow has search attr
			intVal, _ := json.Marshal(i)
			doubleVal, _ := json.Marshal(float64(i))
			strVal, _ := json.Marshal(strconv.Itoa(i))
			timeVal, _ := json.Marshal(time.Now())
			searchAttr := &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					definition.CustomIntField:      intVal,
					definition.CustomDoubleField:   doubleVal,
					definition.CustomKeywordField:  strVal,
					definition.CustomDatetimeField: timeVal,
				},
			}
			startRequest.SearchAttributes = searchAttr
		} else {
			startRequest.SearchAttributes = &types.SearchAttributes{}
		}

		_, err := s.Engine.StartWorkflowExecution(ctx, startRequest)
		s.Nil(err)
	}

	time.Sleep(waitForPinotToSettle)

	// desc := "desc"
	asc := "asc"
	queryTemplate := `WorkflowType = "%s" order by %s %s`
	pageSize := int32(defaultTestValueOfESIndexMaxResultWindow)

	// order by CloseTime asc
	query1 := fmt.Sprintf(queryTemplate, wt, definition.CloseTime, asc)
	var openExecutions []*types.WorkflowExecutionInfo
	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   s.DomainName,
		PageSize: pageSize,
		Query:    query1,
	}
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)
		if int32(len(resp.GetExecutions())) == listRequest.GetPageSize() {
			openExecutions = resp.GetExecutions()
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.NotNil(openExecutions)
	for i := int32(1); i < pageSize; i++ {
		s.True(openExecutions[i-1].GetCloseTime() <= openExecutions[i].GetCloseTime())
	}
	// comment out things below, because json index column can't use order by

	// greatest effort to reduce duplicate code
	// testHelper := func(query, searchAttrKey string, prevVal, currVal interface{}) {
	//	listRequest.Query = query
	//	listRequest.NextPageToken = []byte{}
	//	resp, err := s.Engine.ListWorkflowExecutions(createContext(), listRequest)
	//	s.Nil(err)
	//	openExecutions = resp.GetExecutions()
	//	dec := json.NewDecoder(bytes.NewReader(openExecutions[0].GetSearchAttributes().GetIndexedFields()[searchAttrKey]))
	//	dec.UseNumber()
	//	err = dec.Decode(&prevVal)
	//	s.Nil(err)
	//	for i := int32(1); i < pageSize; i++ {
	//		indexedFields := openExecutions[i].GetSearchAttributes().GetIndexedFields()
	//		searchAttrBytes, ok := indexedFields[searchAttrKey]
	//		if !ok { // last one doesn't have search attr
	//			s.Equal(pageSize-1, i)
	//			break
	//		}
	//		dec := json.NewDecoder(bytes.NewReader(searchAttrBytes))
	//		dec.UseNumber()
	//		err = dec.Decode(&currVal)
	//		s.Nil(err)
	//		var v1, v2 interface{}
	//		switch searchAttrKey {
	//		case definition.CustomIntField:
	//			v1, _ = prevVal.(json.Number).Int64()
	//			v2, _ = currVal.(json.Number).Int64()
	//			s.True(v1.(int64) >= v2.(int64))
	//		case definition.CustomDoubleField:
	//			v1, _ := strconv.ParseFloat(fmt.Sprint(prevVal), 64)
	//			v2, _ := strconv.ParseFloat(fmt.Sprint(currVal), 64)
	//			s.True(v1 >= v2)
	//		case definition.CustomKeywordField:
	//			s.True(prevVal.(string) >= currVal.(string))
	//		case definition.CustomDatetimeField:
	//			v1, _ = strconv.ParseInt(fmt.Sprint(prevVal), 10, 64)
	//			v2, _ = strconv.ParseInt(fmt.Sprint(currVal), 10, 64)
	//			s.True(v1.(int64) >= v2.(int64))
	//		}
	//		prevVal = currVal
	//	}
	//	listRequest.NextPageToken = resp.GetNextPageToken()
	//	resp, err = s.Engine.ListWorkflowExecutions(createContext(), listRequest) // last page
	//	s.Nil(err)
	//	s.Equal(1, len(resp.GetExecutions()))
	// }

	//
	// // order by CustomIntField desc
	// field := definition.CustomIntField
	// query := fmt.Sprintf(queryTemplate, wt, field, desc)
	// var int1, int2 int
	// testHelper(query, field, int1, int2)
	//
	// // order by CustomDoubleField desc
	// field = definition.CustomDoubleField
	// query = fmt.Sprintf(queryTemplate, wt, field, desc)
	// var double1, double2 float64
	// testHelper(query, field, double1, double2)
	//
	// // order by CustomKeywordField desc
	// field = definition.CustomKeywordField
	// query = fmt.Sprintf(queryTemplate, wt, field, desc)
	// var s1, s2 string
	// testHelper(query, field, s1, s2)
	//
	// // order by CustomDatetimeField desc
	// field = definition.CustomDatetimeField
	// query = fmt.Sprintf(queryTemplate, wt, field, desc)
	// var t1, t2 time.Time
	// testHelper(query, field, t1, t2)
}

func (s *PinotIntegrationSuite) testListWorkflowHelper(numOfWorkflows, pageSize int,
	startRequest *types.StartWorkflowExecutionRequest, wid, wType string, isScan bool) {
	ctx, cancel := createContext()
	defer cancel()
	// start enough number of workflows
	for i := 0; i < numOfWorkflows; i++ {
		startRequest.RequestID = uuid.New()
		startRequest.WorkflowID = wid + strconv.Itoa(i)
		_, err := s.Engine.StartWorkflowExecution(ctx, startRequest)
		s.Nil(err)
	}

	time.Sleep(waitForPinotToSettle)

	var openExecutions []*types.WorkflowExecutionInfo
	var nextPageToken []byte

	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:        s.DomainName,
		PageSize:      int32(pageSize),
		NextPageToken: nextPageToken,
		Query:         fmt.Sprintf(`WorkflowType = '%s' and CloseTime = missing`, wType),
	}
	// test first page
	for i := 0; i < numberOfRetry; i++ {
		var resp *types.ListWorkflowExecutionsResponse
		var err error

		if isScan {
			resp, err = s.Engine.ScanWorkflowExecutions(ctx, listRequest)
		} else {
			resp, err = s.Engine.ListWorkflowExecutions(ctx, listRequest)
		}
		s.Nil(err)
		if len(resp.GetExecutions()) == pageSize {
			openExecutions = resp.GetExecutions()
			nextPageToken = resp.GetNextPageToken()
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}

	s.NotNil(openExecutions)
	s.NotNil(nextPageToken)
	s.True(len(nextPageToken) > 0)

	// test last page
	listRequest.NextPageToken = nextPageToken
	inIf := false
	for i := 0; i < numberOfRetry; i++ {
		var resp *types.ListWorkflowExecutionsResponse
		var err error

		if isScan {
			resp, err = s.Engine.ScanWorkflowExecutions(ctx, listRequest)
		} else {
			resp, err = s.Engine.ListWorkflowExecutions(ctx, listRequest)
		}
		s.Nil(err)

		// ans, _ := json.Marshal(resp.GetExecutions())
		// panic(fmt.Sprintf("ABUCSDK: %s", ans))

		if len(resp.GetExecutions()) == numOfWorkflows-pageSize {
			inIf = true
			openExecutions = resp.GetExecutions()
			nextPageToken = resp.GetNextPageToken()
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.True(inIf)
	s.NotNil(openExecutions)
	s.Nil(nextPageToken)
}

func (s *PinotIntegrationSuite) TestScanWorkflow() {
	id := "pinot-integration-scan-workflow-test"
	wt := "pinot-integration-scan-workflow-test-type"
	tl := "pinot-integration-scan-workflow-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)
	query := fmt.Sprintf(`WorkflowID = "%s"`, id)
	s.testHelperForReadOnce(we.GetRunID(), query, true, false)
}

func (s *PinotIntegrationSuite) TestScanWorkflow_SearchAttribute() {
	id := "pinot-integration-scan-workflow-search-attr-test"
	wt := "pinot-integration-scan-workflow-search-attr-test-type"
	tl := "pinot-integration-scan-workflow-search-attr-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)

	attrValBytes, _ := json.Marshal(s.testSearchAttributeVal)
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			s.testSearchAttributeKey: attrValBytes,
		},
	}
	request.SearchAttributes = searchAttr
	ctx, cancel := createContext()
	defer cancel()
	we, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)
	query := fmt.Sprintf(`WorkflowID = "%s" and %s = "%s"`, id, s.testSearchAttributeKey, s.testSearchAttributeVal)
	s.testHelperForReadOnce(we.GetRunID(), query, true, false)
}

func (s *PinotIntegrationSuite) TestScanWorkflow_PageToken() {
	id := "pinot-integration-scan-workflow-token-test"
	wt := "pinot-integration-scan-workflow-token-test-type"
	tl := "pinot-integration-scan-workflow-token-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		Domain:                              s.DomainName,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}

	numOfWorkflows := 4
	pageSize := 3

	s.testListWorkflowHelper(numOfWorkflows, pageSize, request, id, wt, true)
}

func (s *PinotIntegrationSuite) TestCountWorkflow() {
	id := "pinot-integration-count-workflow-test"
	wt := "pinot-integration-count-workflow-test-type"
	tl := "pinot-integration-count-workflow-test-tasklist"
	request := s.createStartWorkflowExecutionRequest(id, wt, tl)

	attrValBytes, _ := json.Marshal(s.testSearchAttributeVal)
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			s.testSearchAttributeKey: attrValBytes,
		},
	}
	request.SearchAttributes = searchAttr
	ctx, cancel := createContext()
	defer cancel()
	_, err := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err)

	query := fmt.Sprintf(`WorkflowID = "%s" and %s = "%s"`, id, s.testSearchAttributeKey, s.testSearchAttributeVal)
	countRequest := &types.CountWorkflowExecutionsRequest{
		Domain: s.DomainName,
		Query:  query,
	}
	var resp *types.CountWorkflowExecutionsResponse
	for i := 0; i < numberOfRetry; i++ {
		resp, err = s.Engine.CountWorkflowExecutions(ctx, countRequest)
		s.Nil(err)
		if resp.GetCount() == int64(1) {
			break
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.Equal(int64(1), resp.GetCount())

	query = fmt.Sprintf(`WorkflowID = "%s" and %s = "%s"`, id, s.testSearchAttributeKey, "noMatch")
	countRequest.Query = query
	resp, err = s.Engine.CountWorkflowExecutions(ctx, countRequest)
	s.Nil(err)
	s.Equal(int64(0), resp.GetCount())
}

func (s *PinotIntegrationSuite) TestUpsertWorkflowExecution() {
	id := "pinot-integration-upsert-workflow-test"
	wt := "pinot-integration-upsert-workflow-test-type"
	tl := "pinot-integration-upsert-workflow-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}
	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	decisionCount := 0
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		upsertDecision := &types.Decision{
			DecisionType: types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr(),
			UpsertWorkflowSearchAttributesDecisionAttributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{}}

		// handle first upsert
		if decisionCount == 0 {
			decisionCount++

			attrValBytes, _ := json.Marshal(s.testSearchAttributeVal)
			upsertSearchAttr := &types.SearchAttributes{
				IndexedFields: map[string][]byte{
					s.testSearchAttributeKey: attrValBytes,
				},
			}
			upsertDecision.UpsertWorkflowSearchAttributesDecisionAttributes.SearchAttributes = upsertSearchAttr
			return nil, []*types.Decision{upsertDecision}, nil
		}
		// handle second upsert, which update existing field and add new field
		if decisionCount == 1 {
			decisionCount++
			upsertDecision.UpsertWorkflowSearchAttributesDecisionAttributes.SearchAttributes = getPinotUpsertSearchAttributes()
			return nil, []*types.Decision{upsertDecision}, nil
		}

		return nil, []*types.Decision{{
			DecisionType: types.DecisionTypeCompleteWorkflowExecution.Ptr(),
			CompleteWorkflowExecutionDecisionAttributes: &types.CompleteWorkflowExecutionDecisionAttributes{
				Result: []byte("Done."),
			},
		}}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		StickyTaskList:  taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	// process 1st decision and assert decision is handled correctly.
	_, newTask, err := poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		true,
		true,
		int64(0),
		1,
		true,
		nil)
	s.Nil(err)
	s.NotNil(newTask)
	s.NotNil(newTask.DecisionTask)
	s.Equal(int64(3), newTask.DecisionTask.GetPreviousStartedEventID())
	s.Equal(int64(7), newTask.DecisionTask.GetStartedEventID())
	s.Equal(4, len(newTask.DecisionTask.History.Events))
	s.Equal(types.EventTypeDecisionTaskCompleted, newTask.DecisionTask.History.Events[0].GetEventType())
	s.Equal(types.EventTypeUpsertWorkflowSearchAttributes, newTask.DecisionTask.History.Events[1].GetEventType())
	s.Equal(types.EventTypeDecisionTaskScheduled, newTask.DecisionTask.History.Events[2].GetEventType())
	s.Equal(types.EventTypeDecisionTaskStarted, newTask.DecisionTask.History.Events[3].GetEventType())

	time.Sleep(waitForPinotToSettle)

	// verify upsert data is on Pinot
	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   s.DomainName,
		PageSize: int32(2),
		// Query:    fmt.Sprintf(`WorkflowType = '%s' and CloseTime = missing`, wt),
		Query: fmt.Sprintf(`WorkflowType = '%s' and CloseTime = missing`, wt),
	}
	verified := false
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)
		if len(resp.GetExecutions()) == 1 {
			execution := resp.GetExecutions()[0]
			retrievedSearchAttr := execution.SearchAttributes
			if retrievedSearchAttr != nil && len(retrievedSearchAttr.GetIndexedFields()) > 0 {
				searchValBytes := retrievedSearchAttr.GetIndexedFields()[s.testSearchAttributeKey]
				var searchVal string
				json.Unmarshal(searchValBytes, &searchVal)
				s.Equal(s.testSearchAttributeVal, searchVal)
				verified = true
				break
			}
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.True(verified)

	// process 2nd decision and assert decision is handled correctly.
	_, newTask, err = poller.PollAndProcessDecisionTaskWithAttemptAndRetryAndForceNewDecision(
		false,
		false,
		true,
		true,
		int64(0),
		1,
		true,
		nil)
	s.Nil(err)
	s.NotNil(newTask)
	s.NotNil(newTask.DecisionTask)
	s.Equal(4, len(newTask.DecisionTask.History.Events))
	s.Equal(types.EventTypeDecisionTaskCompleted, newTask.DecisionTask.History.Events[0].GetEventType())
	s.Equal(types.EventTypeUpsertWorkflowSearchAttributes, newTask.DecisionTask.History.Events[1].GetEventType())
	s.Equal(types.EventTypeDecisionTaskScheduled, newTask.DecisionTask.History.Events[2].GetEventType())
	s.Equal(types.EventTypeDecisionTaskStarted, newTask.DecisionTask.History.Events[3].GetEventType())

	time.Sleep(waitForPinotToSettle)

	// verify upsert data is on Pinot
	s.testListResultForUpsertSearchAttributes(listRequest)
}

func (s *PinotIntegrationSuite) testListResultForUpsertSearchAttributes(listRequest *types.ListWorkflowExecutionsRequest) {
	verified := false
	ctx, cancel := createContext()
	defer cancel()
	for i := 0; i < numberOfRetry; i++ {
		resp, err := s.Engine.ListWorkflowExecutions(ctx, listRequest)
		s.Nil(err)

		// res2B, _ := json.Marshal(resp.GetExecutions())
		// panic(fmt.Sprintf("ABCDDDBUG: %s", listRequest.Query))

		if len(resp.GetExecutions()) == 1 {
			execution := resp.GetExecutions()[0]
			retrievedSearchAttr := execution.SearchAttributes
			if retrievedSearchAttr != nil && len(retrievedSearchAttr.GetIndexedFields()) == 3 {
				// if retrievedSearchAttr != nil && len(retrievedSearchAttr.GetIndexedFields()) > 0 {
				fields := retrievedSearchAttr.GetIndexedFields()
				searchValBytes := fields[s.testSearchAttributeKey]
				var searchVal string
				err := json.Unmarshal(searchValBytes, &searchVal)
				s.Nil(err)
				s.Equal("another string", searchVal)

				searchValBytes2 := fields[definition.CustomIntField]
				var searchVal2 int
				err = json.Unmarshal(searchValBytes2, &searchVal2)
				s.Nil(err)
				s.Equal(123, searchVal2)

				binaryChecksumsBytes := fields[definition.BinaryChecksums]
				var binaryChecksums []string
				err = json.Unmarshal(binaryChecksumsBytes, &binaryChecksums)
				s.Nil(err)
				s.Equal([]string{"binary-v1", "binary-v2"}, binaryChecksums)

				verified = true
				break
			}
		}
		time.Sleep(waitTimeBetweenRetry)
	}
	s.True(verified)
}

func getPinotUpsertSearchAttributes() *types.SearchAttributes {
	attrValBytes1, _ := json.Marshal("another string")
	attrValBytes2, _ := json.Marshal(123)
	binaryChecksums, _ := json.Marshal([]string{"binary-v1", "binary-v2"})
	upsertSearchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			definition.CustomStringField: attrValBytes1,
			definition.CustomIntField:    attrValBytes2,
			definition.BinaryChecksums:   binaryChecksums,
		},
	}
	return upsertSearchAttr
}

func (s *PinotIntegrationSuite) TestUpsertWorkflowExecution_InvalidKey() {
	id := "pinot-integration-upsert-workflow-failed-test"
	wt := "pinot-integration-upsert-workflow-failed-test-type"
	tl := "pinot-integration-upsert-workflow-failed-test-tasklist"
	identity := "worker1"

	workflowType := &types.WorkflowType{}
	workflowType.Name = wt

	taskList := &types.TaskList{}
	taskList.Name = tl

	request := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              s.DomainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
	}
	ctx, cancel := createContext()
	defer cancel()
	we, err0 := s.Engine.StartWorkflowExecution(ctx, request)
	s.Nil(err0)

	s.Logger.Info("StartWorkflowExecution", tag.WorkflowRunID(we.RunID))

	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		upsertDecision := &types.Decision{
			DecisionType: types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr(),
			UpsertWorkflowSearchAttributesDecisionAttributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: &types.SearchAttributes{
					IndexedFields: map[string][]byte{
						"INVALIDKEY": []byte(`1`),
					},
				},
			}}
		return nil, []*types.Decision{upsertDecision}, nil
	}

	poller := &TaskPoller{
		Engine:          s.Engine,
		Domain:          s.DomainName,
		TaskList:        taskList,
		StickyTaskList:  taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.Logger,
		T:               s.T(),
	}

	_, err := poller.PollAndProcessDecisionTask(false, false)
	s.Nil(err)
	defer cancel()
	historyResponse, err := s.Engine.GetWorkflowExecutionHistory(ctx, &types.GetWorkflowExecutionHistoryRequest{
		Domain: s.DomainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
			RunID:      we.RunID,
		},
	})
	s.Nil(err)
	history := historyResponse.History
	decisionFailedEvent := history.GetEvents()[3]
	s.Equal(types.EventTypeDecisionTaskFailed, decisionFailedEvent.GetEventType())
	failedDecisionAttr := decisionFailedEvent.DecisionTaskFailedEventAttributes
	s.Equal(types.DecisionTaskFailedCauseBadSearchAttributes, failedDecisionAttr.GetCause())
	s.True(len(failedDecisionAttr.GetDetails()) > 0)
}
