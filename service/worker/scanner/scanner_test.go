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

package scanner

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/cadence/client"
	"go.uber.org/cadence/mocks"
	"go.uber.org/cadence/worker"
	"go.uber.org/goleak"
	"go.uber.org/zap"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/metrics"
	commonResource "github.com/uber/cadence/common/resource"
	"github.com/uber/cadence/service/history/resource"
	"github.com/uber/cadence/service/worker/scanner/shardscanner"
	worker2 "github.com/uber/cadence/service/worker/worker"
)

type scannerTestSuite struct {
	suite.Suite
	mockCtrl     *gomock.Controller
	mockResource *resource.Test
	mockScope    tally.Scope
	scanner      *Scanner
	mockWorker   *worker2.MockWorker
	mockClient   *mocks.Client
}

func TestScannerSuite(t *testing.T) {
	suite.Run(t, new(scannerTestSuite))
}

func (s *scannerTestSuite) SetupTest() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockResource = resource.NewTest(s.T(), s.mockCtrl, metrics.Worker)
	s.mockScope = tally.NoopScope
	s.mockWorker = worker2.NewMockWorker(s.mockCtrl)
	s.mockClient = new(mocks.Client)

	s.scanner = &Scanner{
		context: scannerContext{
			resource: s.mockResource,
			cfg:      Config{},
		},
		tallyScope: s.mockScope,
		zapLogger:  zap.NewNop(),
		startWorkflowWithRetryFn: func(_ string, _ time.Duration, _ commonResource.Resource, _ func(client client.Client) error) error {
			return nil
		},
		newWorkerFn: func(_ workflowserviceclient.Interface, _ string, _ string, _ worker.Options) worker.Worker {
			return s.mockWorker
		},
	}
}

func (s *scannerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func (s *scannerTestSuite) TestNew() {
	params := &BootstrapParams{
		Config:     Config{},
		TallyScope: s.mockScope,
	}

	s.NotNil(New(s.mockResource, params))
}

func (s *scannerTestSuite) TestStart() {
	testCases := []struct {
		name       string
		cfg        Config
		setupMocks func()
		err        error
	}{
		{
			name: "with shard scanners",
			cfg: Config{
				ShardScanners: []*shardscanner.ScannerConfig{
					{
						ScannerWFTypeName: "testScannerWF1",
						DynamicParams: shardscanner.DynamicParams{
							ScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
								return true
							},
							FixerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
								return true
							},
						},
						StartWorkflowOptions: client.StartWorkflowOptions{
							TaskList: "shard-scanner-task-list-1",
						},
						ScannerHooks: func() *shardscanner.ScannerHooks {
							return &shardscanner.ScannerHooks{
								GetScannerConfig: func(scanner shardscanner.ScannerContext) shardscanner.CustomScannerConfig {
									return map[string]string{}
								},
							}
						},
						FixerHooks: func() *shardscanner.FixerHooks {
							return &shardscanner.FixerHooks{
								GetFixerConfig: func(fixer shardscanner.FixerContext) shardscanner.CustomScannerConfig {
									return map[string]string{}
								},
							}
						},
					},
				},
				Persistence: &config.Persistence{
					DefaultStore: "nosql",
					DataStores: map[string]config.DataStore{
						"nosql": {
							NoSQL: &config.NoSQL{},
						},
					},
				},
				HistoryScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return false
				},
			},
			setupMocks: func() {
				// this is mocking the worker being instantiated and started
				// in here the mockWorker is being started twice, but in reality
				// each worker gets started only once
				s.mockWorker.EXPECT().Start().Return(nil).Times(1)
				s.mockWorker.EXPECT().Start().Return(nil).Times(1)
			},
		},
		{
			name: "with default store type SQL with TaskListScanner enabled",
			cfg: Config{
				Persistence: &config.Persistence{
					DefaultStore: "sql",
					DataStores: map[string]config.DataStore{
						"sql": {
							SQL: &config.SQL{},
						},
					},
				},
				TaskListScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return true
				},
				HistoryScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return false
				},
			},
			setupMocks: func() {
				s.mockWorker.EXPECT().Start().Return(nil).Times(1)
			},
		},
		{
			name: "with HistoryScanner enabled",
			cfg: Config{
				Persistence: &config.Persistence{
					DefaultStore: "nosql",
					DataStores: map[string]config.DataStore{
						"nosql": {
							NoSQL: &config.NoSQL{},
						},
					},
				},
				TaskListScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return false
				},
				HistoryScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return true
				},
			},
			setupMocks: func() {
				s.mockWorker.EXPECT().Start().Return(nil).Times(1)
			},
		},
		{
			name: "failed to start worker",
			cfg: Config{
				Persistence: &config.Persistence{
					DefaultStore: "nosql",
					DataStores: map[string]config.DataStore{
						"nosql": {
							NoSQL: &config.NoSQL{},
						},
					},
				},
				TaskListScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return false
				},
				HistoryScannerEnabled: func(opts ...dynamicconfig.FilterOption) bool {
					return true
				},
			},
			setupMocks: func() {
				s.mockWorker.EXPECT().Start().Return(errors.New("some new error")).Times(1)
			},
			err: errors.New("some new error"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			defer goleak.VerifyNone(s.T())

			s.scanner.context.cfg = tc.cfg
			tc.setupMocks()

			err := s.scanner.Start()

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *scannerTestSuite) Test__startWorkflow() {
	testCases := []struct {
		name         string
		options      client.StartWorkflowOptions
		workflowType string
		args         interface{}
		setupMocks   func(client.StartWorkflowOptions, string, interface{})
		err          error
	}{
		{
			name: "success with args",
			options: client.StartWorkflowOptions{
				ID: "testIDWithArgs",
			},
			workflowType: "workflowTypeWithArgs",
			args:         "testArgs",
			setupMocks: func(options client.StartWorkflowOptions, workflowType string, args interface{}) {
				s.mockClient.On("StartWorkflow", mock.Anything, options, workflowType, args).Return(nil, nil)
			},
		},
		{
			name: "success with no args",
			options: client.StartWorkflowOptions{
				ID: "testIDWithNoArgs",
			},
			workflowType: "workflowTypeWithArgs",
			setupMocks: func(options client.StartWorkflowOptions, workflowType string, args interface{}) {
				s.mockClient.On("StartWorkflow", mock.Anything, options, workflowType).Return(nil, nil)
			},
		},
		{
			name: "WorkflowExecutionAlreadyStarted error",
			options: client.StartWorkflowOptions{
				ID: "testIDWorkflowExecutionAlreadyStarted",
			},
			workflowType: "workflowTypeWorkflowExecutionAlreadyStarted",
			setupMocks: func(options client.StartWorkflowOptions, workflowType string, args interface{}) {
				s.mockClient.On("StartWorkflow", mock.Anything, options, workflowType).Return(nil, &shared.WorkflowExecutionAlreadyStartedError{})
			},
			err: nil,
		},
		{
			name: "another error",
			options: client.StartWorkflowOptions{
				ID: "testIDAnotherError",
			},
			workflowType: "workflowTypeAnotherError",
			setupMocks: func(options client.StartWorkflowOptions, workflowType string, args interface{}) {
				s.mockClient.On("StartWorkflow", mock.Anything, options, workflowType).Return(nil, errors.New("some random error"))
			},
			err: errors.New("some random error"),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			tc.setupMocks(tc.options, tc.workflowType, tc.args)

			err := s.scanner.startWorkflow(s.mockClient, tc.options, tc.workflowType, tc.args)

			if tc.err != nil {
				s.Error(err)
				s.Equal(tc.err, err)
			} else {
				s.NoError(err)
			}
		})
	}
}
