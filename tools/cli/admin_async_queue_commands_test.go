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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminGetAsyncWFConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	// Define table-driven tests
	tests := []struct {
		name             string
		setupMocks       func(*admin.MockClient)
		expectedError    string
		expectedStr      string
		cmdline          string
		mockDepsError    error
		mockContextError error
	}{
		{
			name: "Success",
			setupMocks: func(client *admin.MockClient) {
				expectedResponse := &types.GetDomainAsyncWorkflowConfiguratonResponse{
					Configuration: &types.AsyncWorkflowConfiguration{
						Enabled:   true,
						QueueType: "queueType",
					},
				}
				client.EXPECT().
					GetDomainAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).
					Return(expectedResponse, nil).
					Times(1)
			},
			expectedError: "",
			expectedStr:   "PredefinedQueueName",
			cmdline:       "cadence --domain test-domain admin async-wf-queue get",
		},
		{
			name: "Required flag not present",
			setupMocks: func(client *admin.MockClient) {
				// No call to the mock admin client is expected
			},
			expectedError: "Required flag not present:",
			cmdline:       "cadence admin async-wf-queue get", // --domain is missing
		},
		{
			name: "Config not found (resp.Configuration == nil)",
			setupMocks: func(client *admin.MockClient) {
				client.EXPECT().
					GetDomainAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).
					Return(&types.GetDomainAsyncWorkflowConfiguratonResponse{
						Configuration: nil,
					}, nil).
					Times(1)
			},
			expectedError: "",
			cmdline:       "cadence --domain test-domain admin async-wf-queue get",
		},
		{
			name: "Failed to get async wf config",
			setupMocks: func(client *admin.MockClient) {
				client.EXPECT().
					GetDomainAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("failed to get async config")).
					Times(1)
			},
			expectedError: "Failed to get async wf queue config",
			cmdline:       "cadence --domain test-domain admin async-wf-queue get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the admin client
			adminClient := admin.NewMockClient(mockCtrl)

			// Set up mocks for the current test case
			tt.setupMocks(adminClient)
			ioHandler := &testIOHandler{}

			// Create mock app with clientFactoryMock
			app := NewCliApp(&clientFactoryMock{
				serverAdminClient: adminClient,
			}, WithIOHandler(ioHandler))

			err := clitest.RunCommandLine(t, app, tt.cmdline)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, ioHandler.outputBytes.String(), tt.expectedStr)
			}
		})
	}
}

func TestAdminUpdateAsyncWFConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Define table-driven tests
	tests := []struct {
		name             string
		setupMocks       func(*admin.MockClient)
		expectedError    string
		cmdline          string
		mockContextError error
		unmarshalError   error
	}{
		{
			name: "Success",
			setupMocks: func(client *admin.MockClient) {
				client.EXPECT().
					UpdateDomainAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).
					Return(&types.UpdateDomainAsyncWorkflowConfiguratonResponse{}, nil).
					Times(1)
			},
			expectedError: "",
			cmdline:       `cadence --domain test-domain admin async-wf-queue update --json '{"Enabled": true}'`,
		},
		{
			name: "Required flag not present for domain",
			setupMocks: func(client *admin.MockClient) {
				// No call to the mock admin client is expected
			},
			expectedError: "Required flag not present:",
			cmdline:       `cadence admin async-wf-queue update --json '{"Enabled": true}'`, // --domain is missing
		},
		{
			name: "Required flag not present for JSON",
			setupMocks: func(client *admin.MockClient) {
				// No call to the mock admin client is expected
			},
			expectedError: "Required flag not present:",
			cmdline:       `cadence --domain test-domain admin async-wf-queue update --json ""`, // empty --json flag
		},
		{
			name: "Failed to parse async workflow config",
			setupMocks: func(client *admin.MockClient) {
				// No call setup for this test case as JSON parsing fails
			},
			expectedError:  "Failed to parse async workflow config",
			cmdline:        `cadence --domain test-domain admin async-wf-queue update --json invalid-json`,
			unmarshalError: fmt.Errorf("unmarshal error"),
		},
		{
			name: "Failed to update async workflow config",
			setupMocks: func(client *admin.MockClient) {
				client.EXPECT().
					UpdateDomainAsyncWorkflowConfiguraton(gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("update failed")).
					Times(1)
			},
			expectedError: "Failed to update async workflow queue config",
			cmdline:       `cadence --domain test-domain admin async-wf-queue update --json '{"Enabled": true}'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the admin client
			adminClient := admin.NewMockClient(mockCtrl)

			// Set up mocks for the current test case
			tt.setupMocks(adminClient)

			// Create mock app with clientFactoryMock
			app := NewCliApp(&clientFactoryMock{
				serverAdminClient: adminClient,
			})

			err := clitest.RunCommandLine(t, app, tt.cmdline)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
