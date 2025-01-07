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
	"flag"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/types"
)

func TestAdminGetAsyncWFConfig(t *testing.T) {
	mockCtrl := gomock.NewController(t)

	// Define table-driven tests
	tests := []struct {
		name             string
		setupMocks       func(*admin.MockClient)
		expectedError    string
		expectedStr      string
		flagDomain       string
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
			flagDomain:    "test-domain",
		},
		{
			name: "Required flag not present",
			setupMocks: func(client *admin.MockClient) {
				// No call to the mock admin client is expected
			},
			expectedError: "Required flag not present:",
			flagDomain:    "",
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
			flagDomain:    "test-domain",
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
			flagDomain:    "test-domain",
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

			// Set up CLI context with flags
			set := flag.NewFlagSet("test", 0)
			set.String(FlagDomain, tt.flagDomain, "Domain flag")
			c := cli.NewContext(app, set, nil)

			// Call the function under test
			err := AdminGetAsyncWFConfig(c)

			// Check the expected outcome
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
		flagDomain       string
		flagJSON         string
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
			flagDomain:    "test-domain",
			flagJSON:      `{"Enabled": true}`,
		},
		{
			name: "Required flag not present for domain",
			setupMocks: func(client *admin.MockClient) {
				// No call to the mock admin client is expected
			},
			expectedError: "Required flag not present:",
			flagDomain:    "",
			flagJSON:      `{"Enabled": true}`,
		},
		{
			name: "Required flag not present for JSON",
			setupMocks: func(client *admin.MockClient) {
				// No call to the mock admin client is expected
			},
			expectedError: "Required flag not present:",
			flagDomain:    "test-domain",
			flagJSON:      "",
		},
		{
			name: "Failed to parse async workflow config",
			setupMocks: func(client *admin.MockClient) {
				// No call setup for this test case as JSON parsing fails
			},
			expectedError:  "Failed to parse async workflow config",
			flagDomain:     "test-domain",
			flagJSON:       `invalid-json`,
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
			flagDomain:    "test-domain",
			flagJSON:      `{"Enabled": true}`,
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

			// Set up CLI context with flags
			set := flag.NewFlagSet("test", 0)
			set.String(FlagDomain, tt.flagDomain, "Domain flag")
			set.String(FlagJSON, tt.flagJSON, "JSON flag")
			c := cli.NewContext(app, set, nil)

			// Call the function under test
			err := AdminUpdateAsyncWFConfig(c)

			// Check the expected outcome
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

		})
	}
}
