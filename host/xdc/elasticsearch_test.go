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

//go:build !race && esintegration
// +build !race,esintegration

// to run locally, make sure kafka and es is running,
// then run cmd `go test -v ./host/xdc -run TestESCrossDCTestSuite -tags esintegration`
package xdc

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/environment"
	"github.com/uber/cadence/host"
	"github.com/uber/cadence/host/esutils"
)

const (
	numOfRetry        = 100
	waitTimeInMs      = 400
	waitForESToSettle = 4 * time.Second // wait es shards for some time ensure data consistent
)

type esCrossDCTestSuite struct {
	// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
	// not merely log an error
	*require.Assertions
	suite.Suite
	cluster1       *host.TestCluster
	cluster2       *host.TestCluster
	logger         log.Logger
	clusterConfigs []*host.TestClusterConfig
	esClient       esutils.ESClient

	testSearchAttributeKey string
	testSearchAttributeVal string
}

func TestESCrossDCTestSuite(t *testing.T) {
	flag.Parse()
	suite.Run(t, new(esCrossDCTestSuite))
}

var (
	clusterNameES              = []string{"active-es", "standby-es"}
	clusterReplicationConfigES = []*types.ClusterReplicationConfiguration{
		{
			ClusterName: clusterNameES[0],
		},
		{
			ClusterName: clusterNameES[1],
		},
	}
)

func (s *esCrossDCTestSuite) SetupSuite() {
	s.logger = testlogger.New(s.T())

	fileName := "../testdata/xdc_integration_es_clusters.yaml"
	if host.TestFlags.TestClusterConfigFile != "" {
		fileName = host.TestFlags.TestClusterConfigFile
	}
	s.Require().NoError(environment.SetupEnv())
	confContent, err := ioutil.ReadFile(fileName)
	s.Require().NoError(err)
	confContent = []byte(os.ExpandEnv(string(confContent)))

	var clusterConfigs []*host.TestClusterConfig
	s.Require().NoError(yaml.Unmarshal(confContent, &clusterConfigs))
	s.clusterConfigs = clusterConfigs

	c, err := host.NewCluster(s.T(), clusterConfigs[0], s.logger.WithTags(tag.ClusterName(clusterNameES[0])))
	s.Require().NoError(err)
	s.cluster1 = c

	c, err = host.NewCluster(s.T(), clusterConfigs[1], s.logger.WithTags(tag.ClusterName(clusterNameES[1])))
	s.Require().NoError(err)
	s.cluster2 = c

	s.esClient = esutils.CreateESClient(s.Suite, s.clusterConfigs[0].ESConfig.URL.String(), "v6")
	// TODO Do we also want to run v7 test here?
	s.esClient.PutIndexTemplate(s.Suite, "../testdata/es_index_v6_template.json", "test-visibility-template")
	s.esClient.CreateIndex(s.Suite, s.clusterConfigs[0].ESConfig.Indices[constants.VisibilityAppName])
	s.esClient.CreateIndex(s.Suite, s.clusterConfigs[1].ESConfig.Indices[constants.VisibilityAppName])

	s.testSearchAttributeKey = definition.CustomStringField
	s.testSearchAttributeVal = "test value"
}

func (s *esCrossDCTestSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

func (s *esCrossDCTestSuite) TearDownSuite() {
	s.cluster1.TearDownCluster()
	s.cluster2.TearDownCluster()
	s.esClient.DeleteIndex(s.Suite, s.clusterConfigs[0].ESConfig.Indices[constants.VisibilityAppName])
	s.esClient.DeleteIndex(s.Suite, s.clusterConfigs[1].ESConfig.Indices[constants.VisibilityAppName])
}

func (s *esCrossDCTestSuite) TestSearchAttributes() {
	domainName := "test-xdc-search-attr-" + common.GenerateRandomString(5)
	client1 := s.cluster1.GetFrontendClient() // active
	regReq := &types.RegisterDomainRequest{
		Name:                                   domainName,
		Clusters:                               clusterReplicationConfigES,
		ActiveClusterName:                      clusterNameES[0],
		IsGlobalDomain:                         true,
		WorkflowExecutionRetentionPeriodInDays: 1,
	}
	err := client1.RegisterDomain(createContext(), regReq)
	s.NoError(err)

	descReq := &types.DescribeDomainRequest{
		Name: common.StringPtr(domainName),
	}
	resp, err := client1.DescribeDomain(createContext(), descReq)
	s.NoError(err)
	s.NotNil(resp)
	// Wait for domain cache to pick the change
	time.Sleep(cacheRefreshInterval)

	client2 := s.cluster2.GetFrontendClient() // standby
	resp2, err := client2.DescribeDomain(createContext(), descReq)
	s.NoError(err)
	s.NotNil(resp2)
	s.Equal(resp, resp2)

	// start a workflow
	id := "xdc-search-attr-test-" + uuid.New()
	wt := "xdc-search-attr-test-type"
	tl := "xdc-search-attr-test-tasklist"
	identity := "worker1"
	workflowType := &types.WorkflowType{Name: wt}
	taskList := &types.TaskList{Name: tl}
	attrValBytes, _ := json.Marshal(s.testSearchAttributeVal)
	searchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			s.testSearchAttributeKey: attrValBytes,
		},
	}
	startReq := &types.StartWorkflowExecutionRequest{
		RequestID:                           uuid.New(),
		Domain:                              domainName,
		WorkflowID:                          id,
		WorkflowType:                        workflowType,
		TaskList:                            taskList,
		Input:                               nil,
		ExecutionStartToCloseTimeoutSeconds: common.Int32Ptr(100),
		TaskStartToCloseTimeoutSeconds:      common.Int32Ptr(1),
		Identity:                            identity,
		SearchAttributes:                    searchAttr,
	}
	startTime := time.Now().UnixNano()
	we, err := client1.StartWorkflowExecution(createContext(), startReq)
	s.Nil(err)
	s.NotNil(we.GetRunID())

	s.logger.Info("StartWorkflowExecution \n", tag.WorkflowRunID(we.GetRunID()))

	startFilter := &types.StartTimeFilter{}
	startFilter.EarliestTime = common.Int64Ptr(startTime)
	query := fmt.Sprintf(`WorkflowID = "%s" and %s = "%s"`, id, s.testSearchAttributeKey, s.testSearchAttributeVal)
	listRequest := &types.ListWorkflowExecutionsRequest{
		Domain:   domainName,
		PageSize: 5,
		Query:    query,
	}

	testListResult := func(client host.FrontendClient) {
		var openExecution *types.WorkflowExecutionInfo
		for i := 0; i < numOfRetry; i++ {
			startFilter.LatestTime = common.Int64Ptr(time.Now().UnixNano())

			resp, err := client.ListWorkflowExecutions(createContext(), listRequest)
			s.Nil(err)
			if len(resp.GetExecutions()) == 1 {
				openExecution = resp.GetExecutions()[0]
				break
			}
			time.Sleep(waitTimeInMs * time.Millisecond)
		}
		s.NotNil(openExecution)
		s.Equal(we.GetRunID(), openExecution.GetExecution().GetRunID())
		searchValBytes := openExecution.SearchAttributes.GetIndexedFields()[s.testSearchAttributeKey]
		var searchVal string
		json.Unmarshal(searchValBytes, &searchVal)
		s.Equal(s.testSearchAttributeVal, searchVal)
	}

	// List workflow in active
	engine1 := s.cluster1.GetFrontendClient()
	testListResult(engine1)

	// List workflow in standby
	engine2 := s.cluster2.GetFrontendClient()
	testListResult(engine2)

	// upsert search attributes
	dtHandler := func(execution *types.WorkflowExecution, wt *types.WorkflowType,
		previousStartedEventID, startedEventID int64, history *types.History) ([]byte, []*types.Decision, error) {

		upsertDecision := &types.Decision{
			DecisionType: types.DecisionTypeUpsertWorkflowSearchAttributes.Ptr(),
			UpsertWorkflowSearchAttributesDecisionAttributes: &types.UpsertWorkflowSearchAttributesDecisionAttributes{
				SearchAttributes: getUpsertSearchAttributes(),
			}}

		return nil, []*types.Decision{upsertDecision}, nil
	}

	poller := host.TaskPoller{
		Engine:          client1,
		Domain:          domainName,
		TaskList:        taskList,
		Identity:        identity,
		DecisionHandler: dtHandler,
		Logger:          s.logger,
		T:               s.T(),
	}

	_, err = poller.PollAndProcessDecisionTask(false, false)
	s.logger.Info("PollAndProcessDecisionTask", tag.Error(err))
	s.Nil(err)

	time.Sleep(waitForESToSettle)

	listRequest = &types.ListWorkflowExecutionsRequest{
		Domain:   domainName,
		PageSize: int32(2),
		Query:    fmt.Sprintf(`WorkflowType = '%s' and CloseTime = missing`, wt),
	}

	testListResult = func(client host.FrontendClient) {
		verified := false
		for i := 0; i < numOfRetry; i++ {
			resp, err := client.ListWorkflowExecutions(createContext(), listRequest)
			s.Nil(err)
			if len(resp.GetExecutions()) == 1 {
				execution := resp.GetExecutions()[0]
				retrievedSearchAttr := execution.SearchAttributes
				if retrievedSearchAttr != nil && len(retrievedSearchAttr.GetIndexedFields()) == 2 {
					fields := retrievedSearchAttr.GetIndexedFields()
					searchValBytes := fields[s.testSearchAttributeKey]
					var searchVal string
					json.Unmarshal(searchValBytes, &searchVal)
					s.Equal("another string", searchVal)

					searchValBytes2 := fields[definition.CustomIntField]
					var searchVal2 int
					json.Unmarshal(searchValBytes2, &searchVal2)
					s.Equal(123, searchVal2)

					verified = true
					break
				}
			}
			time.Sleep(waitTimeInMs * time.Millisecond)
		}
		s.True(verified)
	}

	// test upsert result in active
	testListResult(engine1)

	// terminate workflow
	terminateReason := "force terminate to make sure standby process tasks"
	terminateDetails := []byte("terminate details.")
	err = client1.TerminateWorkflowExecution(createContext(), &types.TerminateWorkflowExecutionRequest{
		Domain: domainName,
		WorkflowExecution: &types.WorkflowExecution{
			WorkflowID: id,
		},
		Reason:   terminateReason,
		Details:  terminateDetails,
		Identity: identity,
	})
	s.Nil(err)

	// check terminate done
	executionTerminated := false
	getHistoryReq := &types.GetWorkflowExecutionHistoryRequest{
		Domain: domainName,
		Execution: &types.WorkflowExecution{
			WorkflowID: id,
		},
	}
GetHistoryLoop:
	for i := 0; i < 10; i++ {
		historyResponse, err := client1.GetWorkflowExecutionHistory(createContext(), getHistoryReq)
		s.Nil(err)
		history := historyResponse.History

		lastEvent := history.Events[len(history.Events)-1]
		if *lastEvent.EventType != types.EventTypeWorkflowExecutionTerminated {
			s.logger.Warn("Execution not terminated yet.")
			time.Sleep(100 * time.Millisecond)
			continue GetHistoryLoop
		}

		terminateEventAttributes := lastEvent.WorkflowExecutionTerminatedEventAttributes
		s.Equal(terminateReason, terminateEventAttributes.Reason)
		s.Equal(terminateDetails, terminateEventAttributes.Details)
		s.Equal(identity, terminateEventAttributes.Identity)
		executionTerminated = true
		break GetHistoryLoop
	}
	s.True(executionTerminated)

	// check history replicated to the other cluster
	var historyResponse *types.GetWorkflowExecutionHistoryResponse
	eventsReplicated := false
GetHistoryLoop2:
	for i := 0; i < numOfRetry; i++ {
		historyResponse, err = client2.GetWorkflowExecutionHistory(createContext(), getHistoryReq)
		if err == nil {
			history := historyResponse.History
			lastEvent := history.Events[len(history.Events)-1]
			if *lastEvent.EventType == types.EventTypeWorkflowExecutionTerminated {
				terminateEventAttributes := lastEvent.WorkflowExecutionTerminatedEventAttributes
				s.Equal(terminateReason, terminateEventAttributes.Reason)
				s.Equal(terminateDetails, terminateEventAttributes.Details)
				s.Equal(identity, terminateEventAttributes.Identity)
				eventsReplicated = true
				break GetHistoryLoop2
			}
		}
		time.Sleep(waitTimeInMs * time.Millisecond)
	}
	s.Nil(err)
	s.True(eventsReplicated)

	// test upsert result in standby
	testListResult(engine2)
}

func getUpsertSearchAttributes() *types.SearchAttributes {
	attrValBytes1, _ := json.Marshal("another string")
	attrValBytes2, _ := json.Marshal(123)
	upsertSearchAttr := &types.SearchAttributes{
		IndexedFields: map[string][]byte{
			definition.CustomStringField: attrValBytes1,
			definition.CustomIntField:    attrValBytes2,
		},
	}
	return upsertSearchAttr
}
