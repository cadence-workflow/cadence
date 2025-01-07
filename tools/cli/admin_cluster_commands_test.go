// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/visibility"
	"github.com/uber/cadence/service/worker/failovermanager"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminAddSearchAttribute_isValueTypeValid(t *testing.T) {
	testCases := []struct {
		name     string
		input    int
		expected bool
	}{
		{
			name:     "negative",
			input:    -1,
			expected: false,
		},
		{
			name:     "valid",
			input:    0,
			expected: true,
		},
		{
			name:     "valid",
			input:    5,
			expected: true,
		},
		{
			name:     "unknown",
			input:    6,
			expected: false,
		},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expected, isValueTypeValid(testCase.input))
	}
}

func TestAdminFailover(t *testing.T) {
	var listDomainsResponse = &types.ListDomainsResponse{
		Domains: []*types.DescribeDomainResponse{
			{
				DomainInfo: &types.DomainInfo{
					Name:        "test-domain",
					Description: "a test domain",
					OwnerEmail:  "test@uber.com",
					Data: map[string]string{
						common.DomainDataKeyForManagedFailover: "true",
					},
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
			},
		},
	}

	t.Run("standby cluster", func(t *testing.T) {
		td := newCLITestData(t)
		cliCtx := clitest.NewCLIContext(
			t,
			td.app,
			clitest.StringArgument(FlagActiveClusterName, "standby"),
		)

		td.mockFrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(listDomainsResponse, nil).Times(1)
		td.mockFrontendClient.EXPECT().UpdateDomain(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)

		domainCLI := &domainCLIImpl{
			frontendClient: td.mockFrontendClient,
		}

		succeed, failed, err := domainCLI.failoverDomains(cliCtx)
		assert.Equal(t, []string{"test-domain"}, succeed)
		assert.Equal(t, 0, len(failed))
		assert.NoError(t, err)
	})

	t.Run("active cluster", func(t *testing.T) {
		td := newCLITestData(t)
		cliCtx := clitest.NewCLIContext(
			t,
			td.app,
			clitest.StringArgument(FlagActiveClusterName, "active"),
		)

		td.mockFrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(listDomainsResponse, nil).Times(1)

		domainCLI := &domainCLIImpl{
			frontendClient: td.mockFrontendClient,
		}

		succeed, failed, err := domainCLI.failoverDomains(cliCtx)
		assert.Equal(t, 0, len(succeed))
		assert.Equal(t, 0, len(failed))
		assert.NoError(t, err)
	})
}

func TestValidSearchAttributeKey(t *testing.T) {
	assert.NoError(t, visibility.ValidateSearchAttributeKey("city"))
	assert.NoError(t, visibility.ValidateSearchAttributeKey("cityId"))
	assert.NoError(t, visibility.ValidateSearchAttributeKey("paymentProfileUUID"))
	assert.NoError(t, visibility.ValidateSearchAttributeKey("job_type"))

	assert.Error(t, visibility.ValidateSearchAttributeKey("payments-biling-invoices-TransactionUUID"))
	assert.Error(t, visibility.ValidateSearchAttributeKey("9lives"))
	assert.Error(t, visibility.ValidateSearchAttributeKey("tax%"))
}

func TestAdminDescribeCluster(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(td *cliTestData)
		expectedError  string
		expectedOutput string
	}{
		{
			name: "Success",
			mockSetup: func(td *cliTestData) {
				// Expected response from DescribeCluster
				expectedResponse := &types.DescribeClusterResponse{
					SupportedClientVersions: &types.SupportedClientVersions{
						GoSdk: "1.5.0",
					},
				}

				td.mockAdminClient.EXPECT().DescribeCluster(gomock.Any()).Return(expectedResponse, nil).Times(1)
			},
			expectedOutput: `{
  "supportedClientVersions": {
    "goSdk": "1.5.0"
  }
}
`,
			expectedError: "",
		},
		{
			name: "DescribeClusterError",
			mockSetup: func(td *cliTestData) {
				// Mock DescribeCluster to return an error
				td.mockAdminClient.EXPECT().DescribeCluster(gomock.Any()).Return(nil, fmt.Errorf("DescribeCluster failed")).Times(1)
			},
			expectedOutput: "",
			expectedError:  "Operation DescribeCluster failed.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			// Set up mock based on the specific test case
			tt.mockSetup(td)
			cliCtx := clitest.NewCLIContext(t, td.app)

			err := AdminDescribeCluster(cliCtx)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminRebalanceStart(t *testing.T) {
	tests := []struct {
		name           string
		mockSetup      func(td *cliTestData)
		expectedError  string
		expectedOutput string
	}{
		{
			name: "Success",
			mockSetup: func(td *cliTestData) {
				mockResponse := &types.StartWorkflowExecutionResponse{
					RunID: "test-run-id",
				}

				td.mockFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(mockResponse, nil).Times(1)
			},
			expectedOutput: "Rebalance workflow started\nwid: cadence-rebalance-workflow\nrid: test-run-id\n",
			expectedError:  "",
		},
		{
			name: "StartWorkflowExecutionError",
			mockSetup: func(td *cliTestData) {
				// Mock StartWorkflowExecution to return an error
				td.mockFrontendClient.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed to start workflow")).Times(1)
			},
			expectedOutput: "",
			expectedError:  "Failed to start failover workflow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			tt.mockSetup(td)

			cliCtx := clitest.NewCLIContext(t, td.app, clitest.StringArgument(FlagDomain, testDomain))

			err := AdminRebalanceStart(cliCtx)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestIntValTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		valType  int
		expected string
	}{
		{
			name:     "StringType",
			valType:  0,
			expected: "String",
		},
		{
			name:     "KeywordType",
			valType:  1,
			expected: "Keyword",
		},
		{
			name:     "IntType",
			valType:  2,
			expected: "Int",
		},
		{
			name:     "DoubleType",
			valType:  3,
			expected: "Double",
		},
		{
			name:     "BoolType",
			valType:  4,
			expected: "Bool",
		},
		{
			name:     "DatetimeType",
			valType:  5,
			expected: "Datetime",
		},
		{
			name:     "UnknownType",
			valType:  999,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function and check the result
			result := intValTypeToString(tt.valType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAdminRebalanceList(t *testing.T) {
	tests := []struct {
		name           string
		prepareEnv     func(td *cliTestData) *cli.Context
		expectedError  string
		expectedOutput string
	}{
		{
			name: "Success",
			prepareEnv: func(td *cliTestData) *cli.Context {
				// Mock successful ListWorkflow call
				td.mockFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.CountWorkflowExecutionsResponse{}, nil).Times(1)
				td.mockFrontendClient.EXPECT().ListClosedWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.ListClosedWorkflowExecutionsResponse{}, nil).Times(1)

				return clitest.NewCLIContext(t,
					td.app,
					clitest.StringArgument(FlagWorkflowID, failovermanager.RebalanceWorkflowID),
					clitest.StringArgument(FlagDomain, common.SystemLocalDomainName),
				)
			},
			expectedOutput: "\n", // Example output for success case
			expectedError:  "",
		},
		{
			name: "SetWorkflowIDError",

			prepareEnv: func(td *cliTestData) *cli.Context {
				// Create CLI app and set up flag set without FlagWorkflowID
				return clitest.NewCLIContext(t,
					td.app,
					clitest.StringArgument(FlagDomain, common.SystemLocalDomainName),
				)
			},
			expectedOutput: "",
			expectedError:  "no such flag -workflow_id",
		},
		{
			name: "SetDomainError",
			prepareEnv: func(td *cliTestData) *cli.Context {
				// Create CLI app and set up flag set without FlagDomain
				return clitest.NewCLIContext(t,
					td.app,
					clitest.StringArgument(FlagWorkflowID, failovermanager.RebalanceWorkflowID),
				)
			},
			expectedOutput: "",
			expectedError:  "no such flag -domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			// Prepare the test environment for the specific test case
			c := tt.prepareEnv(td)

			err := AdminRebalanceList(c)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestAdminAddSearchAttribute_errors(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func(app *cli.App) *cli.Context
		expectedOutput string
		expectedError  string
	}{
		{
			name: "MissingSearchAttributesKey",
			setupContext: func(app *cli.App) *cli.Context {
				return clitest.NewCLIContext(t, app /* missing FlagSearchAttributesKey argument */)
			},
			expectedOutput: "", // In this case, likely no output
			expectedError:  "Required flag not present:",
		},
		{
			name: "InvalidSearchAttributeKey",
			setupContext: func(app *cli.App) *cli.Context {
				const invalidSearchAttr = "123_invalid_key" // Invalid key, starts with number
				return clitest.NewCLIContext(t, app, clitest.StringArgument(FlagSearchAttributesKey, invalidSearchAttr))
			},
			expectedOutput: "", // In this case, likely no output
			expectedError:  "Invalid search-attribute key.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)

			// Set up the CLI context for the specific test case
			cliCtx := tt.setupContext(td.app)

			err := AdminAddSearchAttribute(cliCtx)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}
