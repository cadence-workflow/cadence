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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

const testDynamicConfigName = "test-dynamic-config-name"

func TestAdminGetDynamicConfig(t *testing.T) {
	tests := []struct {
		name        string
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "calling with required arguments",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
				)

				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(),
					&types.ListDynamicConfigRequest{
						ConfigName: testDynamicConfigName,
					}).Return(nil, nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "failed to get dynamic config values",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
				)

				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(),
					&types.ListDynamicConfigRequest{
						ConfigName: testDynamicConfigName,
					}).Return(nil, assert.AnError)

				return cliCtx
			},
			errContains: "Failed to get dynamic config value(s)",
		},
		{
			name: "received a list of dynamic config values",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
				)

				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(),
					&types.ListDynamicConfigRequest{
						ConfigName: testDynamicConfigName,
					}).Return(&types.ListDynamicConfigResponse{
					Entries: []*types.DynamicConfigEntry{
						{
							Name: testDynamicConfigName,
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
				return cliCtx
			},
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminGetDynamicConfig(cliCtx)
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
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "calling with required arguments",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
					clitest.StringArgument(FlagDynamicConfigValue, "'{'Value':some-value,'Filters':[]}'"),
				)

				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).Return(nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "failed to update dynamic config values",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
					clitest.StringArgument(FlagDynamicConfigValue, "'{'Value':some-value,'Filters':[]}'"),
				)

				td.mockAdminClient.EXPECT().UpdateDynamicConfig(gomock.Any(), gomock.Any()).Return(assert.AnError)

				return cliCtx
			},
			errContains: "Failed to update dynamic config value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminUpdateDynamicConfig(cliCtx)
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
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "no arguments provided",
			testSetup: func(td *cliTestData) *cli.Context {
				return clitest.NewCLIContext(t, td.app /* arguments are missing */)
			},
			errContains: "Required flag not found",
		},
		{
			name: "calling with required arguments",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
					clitest.StringArgument(FlagDynamicConfigFilter, "{'Name':'domainName', 'Value':'test-domain'}"),
				)

				td.mockAdminClient.EXPECT().RestoreDynamicConfig(gomock.Any(), gomock.Any()).Return(nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "failed to update dynamic config values",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDynamicConfigName, testDynamicConfigName),
					clitest.StringArgument(FlagDynamicConfigValue, "'{'Value':some-value,'Filters':[]}'"),
				)

				td.mockAdminClient.EXPECT().RestoreDynamicConfig(gomock.Any(), gomock.Any()).Return(assert.AnError)

				return cliCtx
			},
			errContains: "Failed to restore dynamic config value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminRestoreDynamicConfig(cliCtx)
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
		testSetup   func(td *cliTestData) *cli.Context
		errContains string // empty if no error is expected
	}{
		{
			name: "failed with no dynamic config values stored to list",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
				)

				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(), gomock.Any()).Return(nil, nil)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "failed to list dynamic config values",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
				)

				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

				return cliCtx
			},
			errContains: "Failed to list dynamic config value(s)",
		},
		{
			name: "succeeded to list dynamic config values",
			testSetup: func(td *cliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
				)

				td.mockAdminClient.EXPECT().ListDynamicConfig(gomock.Any(), gomock.Any()).Return(&types.ListDynamicConfigResponse{
					Entries: []*types.DynamicConfigEntry{
						{
							Name: testDynamicConfigName,
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

				return cliCtx
			},
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			cliCtx := tt.testSetup(td)

			err := AdminListDynamicConfig(cliCtx)
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
		cliCtx := clitest.NewCLIContext(t, td.app)

		err := AdminListConfigKeys(cliCtx)
		assert.NoError(t, err)
	})
}
