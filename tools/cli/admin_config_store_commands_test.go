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
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"go.uber.org/yarpc"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

func TestAdminGetDynamicConfig(t *testing.T) {
	tests := []struct {
		name        string
		cmdline     string
		setupMock   func(td *cliTestData)
		errContains string // empty if no error is expected
	}{
		{
			cmdline: `cadence admin config get --name ""`,
			name:    "no arguments provided",
			setupMock: func(td *cliTestData) {
				// empty since arguments are missing
			},
			errContains: "Required flag not found",
		},
		{
			name:    "failed to get dynamic config values",
			cmdline: `cadence admin config get --name test-dynamic-config-name`,
			setupMock: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().GetDynamicConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *types.GetDynamicConfigRequest, _ ...yarpc.CallOption) (*types.GetDynamicConfigResponse, error) {
						assert.Equal(t, "test-dynamic-config-name", request.ConfigName)
						return nil, assert.AnError
					})
			},
			errContains: "Failed to get dynamic config value",
		},
		{
			name:    "received a dynamic config value successfully",
			cmdline: `cadence admin config get --name test-dynamic-config-name`,
			setupMock: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().GetDynamicConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *types.GetDynamicConfigRequest, _ ...yarpc.CallOption) (*types.GetDynamicConfigResponse, error) {
						assert.Equal(t, "test-dynamic-config-name", request.ConfigName)
						return &types.GetDynamicConfigResponse{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeThriftRW.Ptr(),
								Data:         []byte(`"config-value"`),
							},
						}, nil
					})
			},
			errContains: "",
		},
		{
			name:    "received a dynamic config value with filters successfully",
			cmdline: `cadence admin config get --name test-dynamic-config-name --filter '{"domainName":"test-domain", "shardID": 1, "isEnabled": true}'`,
			setupMock: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().GetDynamicConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *types.GetDynamicConfigRequest, _ ...yarpc.CallOption) (*types.GetDynamicConfigResponse, error) {
						assert.Equal(t, "test-dynamic-config-name", request.ConfigName)
						assert.ElementsMatch(t, request.Filters, []*types.DynamicConfigFilter{
							{
								Name: "domainName",
								Value: &types.DataBlob{
									EncodingType: types.EncodingTypeJSON.Ptr(),
									Data:         []byte(`"test-domain"`),
								},
							},
							{
								Name: "shardID",
								Value: &types.DataBlob{
									EncodingType: types.EncodingTypeJSON.Ptr(),
									Data:         []byte(`1`),
								},
							},
							{
								Name: "isEnabled",
								Value: &types.DataBlob{
									EncodingType: types.EncodingTypeJSON.Ptr(),
									Data:         []byte(`true`),
								},
							},
						})
						return &types.GetDynamicConfigResponse{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeThriftRW.Ptr(),
								Data:         []byte(`"config-value"`),
							},
						}, nil
					})
			},
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			tt.setupMock(td)

			err := clitest.RunCommandLine(t, td.app, tt.cmdline)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminUpdateDynamicConfig(t *testing.T) {
	tests := []struct {
		name        string
		cmdline     string
		setupMocks  func(td *cliTestData)
		errContains string // empty if no error is expected
	}{
		{
			name:    "no arguments provided",
			cmdline: `cadence admin config update --name "" --value ""`,
			setupMocks: func(td *cliTestData) {
				// empty since arguments are missing
			},
			errContains: "Required flag not found",
		},
		{
			name:    "calling with required arguments",
			cmdline: `cadence admin config update --name test-dynamic-config-name --value "{}"`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).Return(nil)
			},
			errContains: "",
		},
		{
			name:    "failed to update dynamic config values",
			cmdline: `cadence admin config update --name test-dynamic-config-name --value "{}"`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			errContains: "Failed to update dynamic config value",
		},
		{
			name:    "update with simple boolean value",
			cmdline: `cadence admin config update --name test.config --value ` + `'{"Value":true,"Filters":[]}'`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *types.UpdateDynamicConfigRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, "test.config", request.ConfigName)
						assert.Len(t, request.ConfigValues, 1)

						// Verify the value is correctly parsed
						var actualValue interface{}
						err := json.Unmarshal(request.ConfigValues[0].Value.Data, &actualValue)
						assert.NoError(t, err)
						assert.Equal(t, true, actualValue)

						// Verify no filters
						assert.Empty(t, request.ConfigValues[0].Filters)

						return nil
					})
			},
			errContains: "",
		},
		{
			name:    "update with map value and no filters",
			cmdline: `cadence admin config update --name frontend.validSearchAttributes --value ` + `'{"Value":{"DomainID":1,"WorkflowID":1},"Filters":[]}'`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *types.UpdateDynamicConfigRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, "frontend.validSearchAttributes", request.ConfigName)
						assert.Len(t, request.ConfigValues, 1)

						// Verify the value is correctly parsed
						var actualValue interface{}
						err := json.Unmarshal(request.ConfigValues[0].Value.Data, &actualValue)
						assert.NoError(t, err)

						expectedValue := map[string]interface{}{
							"DomainID":   float64(1), // JSON numbers unmarshal to float64
							"WorkflowID": float64(1),
						}
						assert.Equal(t, expectedValue, actualValue)

						// Verify no filters
						assert.Empty(t, request.ConfigValues[0].Filters)

						return nil
					})
			},
			errContains: "",
		},
		{
			name:    "update with map value and filters",
			cmdline: `cadence admin config update --name frontend.validSearchAttributes --value ` + `'{"Value":{"DomainID":1,"WorkflowID":1},"Filters":[{"Name":"domainName","Value":"test-domain"}]}'`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, request *types.UpdateDynamicConfigRequest, _ ...yarpc.CallOption) error {
						assert.Equal(t, "frontend.validSearchAttributes", request.ConfigName)
						assert.Len(t, request.ConfigValues, 1)

						// Verify the value is correctly parsed
						var actualValue interface{}
						err := json.Unmarshal(request.ConfigValues[0].Value.Data, &actualValue)
						assert.NoError(t, err)

						expectedValue := map[string]interface{}{
							"DomainID":   float64(1),
							"WorkflowID": float64(1),
						}
						assert.Equal(t, expectedValue, actualValue)

						// Verify filters
						assert.Len(t, request.ConfigValues[0].Filters, 1)
						assert.Equal(t, "domainName", request.ConfigValues[0].Filters[0].Name)

						var filterValue interface{}
						err = json.Unmarshal(request.ConfigValues[0].Filters[0].Value.Data, &filterValue)
						assert.NoError(t, err)
						assert.Equal(t, "test-domain", filterValue)

						return nil
					})
			},
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			tt.setupMocks(td)

			err := clitest.RunCommandLine(t, td.app, tt.cmdline)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminRestoreDynamicConfig(t *testing.T) {
	tests := []struct {
		name        string
		cmdline     string
		setupMocks  func(td *cliTestData)
		errContains string // empty if no error is expected
	}{
		{
			name:    "no arguments provided",
			cmdline: `cadence admin config restore --name ''`,
			setupMocks: func(td *cliTestData) {
				// empty since args are missing
			},
			errContains: "Required flag not found",
		},
		{
			name:    "calling with required arguments",
			cmdline: `cadence admin config restore --name test-dynamic-config-name --filter '{"domainName":"test-domain"}'`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().RestoreDynamicConfig(gomock.Any(), gomock.Any()).Return(nil)
			},
			errContains: "",
		},
		{
			name:    "failed to update dynamic config values",
			cmdline: `cadence admin config restore --name test-dynamic-config-name --filter '{"Value":"some-value","Filters":[]}'`,
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().RestoreDynamicConfig(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			errContains: "Failed to restore dynamic config value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			tt.setupMocks(td)

			err := clitest.RunCommandLine(t, td.app, tt.cmdline)
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminListDynamicConfig(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(td *cliTestData)
		errContains string // empty if no error is expected
	}{
		{
			name: "failed with no dynamic config values stored to list",
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			errContains: "",
		},
		{
			name: "failed to list dynamic config values",
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
			},
			errContains: "Failed to list dynamic config value(s)",
		},
		{
			name: "succeeded to list dynamic config values",
			setupMocks: func(td *cliTestData) {
				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(), gomock.Any()).Return(&types.ListDynamicConfigResponse{
					Entries: []*types.DynamicConfigEntry{
						{
							Name: "test-dynamic-config-name",
							Values: []*types.DynamicConfigValue{
								{
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeThriftRW.Ptr(),
										Data:         []byte("config-value"),
									},
									Filters: []*types.DynamicConfigFilter{
										{
											Name: "Filter1",
											Value: &types.DataBlob{
												EncodingType: types.EncodingTypeThriftRW.Ptr(),
												Data:         []byte("filter-value"),
											},
										},
									},
								},
							},
						},
					}}, nil)
			},
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			tt.setupMocks(td)

			err := clitest.RunCommandLine(t, td.app, "cadence admin config list")
			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}

func TestAdminListConfigKeys(t *testing.T) {
	t.Run("list config keys", func(t *testing.T) {
		td := newCLITestData(t)
		assert.NoError(t, clitest.RunCommandLine(t, td.app, "cadence admin config listall"))
	})
}
