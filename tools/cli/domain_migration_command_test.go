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
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/cli/clitest"
)

const (
	testDomainName    = "test-domain"
	testNewDomainName = "test-new-domain"
	testDomainOwner   = "test-owner"
)

type domainMigrationCliTestData struct {
	mockFrontendClient     *frontend.MockClient
	mockAdminClient        *admin.MockClient
	ioHandler              *testIOHandler
	app                    *cli.App
	mockManagerFactory     *MockManagerFactory
	domainMigrationCLIImpl DomainMigrationCommand
}

func newDomainMigrationCliTestData(t *testing.T) *domainMigrationCliTestData {
	var td domainMigrationCliTestData

	ctrl := gomock.NewController(t)

	td.mockFrontendClient = frontend.NewMockClient(ctrl)
	td.mockAdminClient = admin.NewMockClient(ctrl)
	td.mockManagerFactory = NewMockManagerFactory(ctrl)
	td.ioHandler = &testIOHandler{}

	// Create a new CLI app with client factory and persistence manager factory
	td.app = NewCliApp(
		&clientFactoryMock{
			serverFrontendClient: td.mockFrontendClient,
			serverAdminClient:    td.mockAdminClient,
		},
		WithIOHandler(td.ioHandler),
		WithManagerFactory(td.mockManagerFactory), // Inject the mocked persistence manager factory
	)

	td.domainMigrationCLIImpl = &domainMigrationCLIImpl{
		frontendClient:         td.mockFrontendClient,
		destinationClient:      td.mockFrontendClient,
		frontendAdminClient:    td.mockAdminClient,
		destinationAdminClient: td.mockAdminClient,
	}
	return &td
}

func (td *domainMigrationCliTestData) consoleOutput() string {
	return td.ioHandler.outputBytes.String()
}

func TestDomainMetaDataCheck(t *testing.T) {
	testDomain := testDomainName
	testNewDomain := testNewDomainName
	tests := []struct {
		name           string
		testSetup      func(td *domainMigrationCliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "all arguments provided",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagDestinationDomain, testNewDomain),
				)

				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{OwnerEmail: testDomainOwner},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 30,
					},
				}, nil).Times(1)
				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &testNewDomain,
				}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{OwnerEmail: testDomainOwner},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 30,
					},
				}, nil).Times(1)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "could not describe old domain",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagDestinationDomain, testNewDomain),
				)

				td.mockFrontendClient.EXPECT().DescribeDomain(
					gomock.Any(), gomock.Any(),
				).Return(nil, assert.AnError).Times(1)
				return cliCtx
			},
			errContains: "Could not describe old domain, Please check to see if old domain exists before migrating.",
		},
		{
			name: "could not describe new domain",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagDestinationDomain, testNewDomain),
				)

				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any()).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{OwnerEmail: testDomainOwner},
					Configuration: &types.DomainConfiguration{
						WorkflowExecutionRetentionPeriodInDays: 30,
					},
				}, nil).Times(1)

				td.mockFrontendClient.EXPECT().DescribeDomain(
					gomock.Any(), gomock.Any(),
				).Return(nil, assert.AnError).Times(1)
				return cliCtx
			},
			errContains: "Could not describe new domain, Please check to see if new domain exists before migrating.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newDomainMigrationCliTestData(t)
			cliCtx := tt.testSetup(td)

			_, err := td.domainMigrationCLIImpl.DomainMetaDataCheck(cliCtx)

			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestDomainWorkflowCheck(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *domainMigrationCliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "all arguments provided",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomainName),
				)

				td.mockFrontendClient.EXPECT().CountWorkflowExecutions(gomock.Any(), gomock.Any()).Return(&types.CountWorkflowExecutionsResponse{
					Count: 25,
				}, nil).Times(1)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "could not count workflows",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomainName),
				)

				td.mockFrontendClient.EXPECT().CountWorkflowExecutions(
					gomock.Any(), gomock.Any(),
				).Return(nil, assert.AnError).Times(1)
				return cliCtx
			},
			errContains: "Failed to count workflow.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newDomainMigrationCliTestData(t)
			cliCtx := tt.testSetup(td)

			_, err := td.domainMigrationCLIImpl.DomainWorkFlowCheck(cliCtx)

			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestSearchAttributesChecker(t *testing.T) {
	tests := []struct {
		name           string
		testSetup      func(td *domainMigrationCliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "all arguments provided",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringSliceArgument(FlagSearchAttribute, "Domain:STRING", "WorkflowID:INT"),
				)

				td.mockFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(&types.GetSearchAttributesResponse{
					Keys: map[string]types.IndexedValueType{
						"Domain":     types.IndexedValueTypeString,
						"WorkflowID": types.IndexedValueTypeInt,
					},
				}, nil).Times(2)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "without the search attributes argument",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
				)

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "invalid search attribute format",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringSliceArgument(FlagSearchAttribute, "Domain"),
				)

				return cliCtx
			},
			errContains: "Invalid search attribute format:",
		},
		{
			name: "invalid search attribute type",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringSliceArgument(FlagSearchAttribute, "DomainData:Blob"),
				)

				return cliCtx
			},
			errContains: "Invalid search attribute type for DomainData: Blob",
		},
		{
			name: "failed to get search attributes for old domain",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringSliceArgument(FlagSearchAttribute, "Domain:STRING", "WorkflowID:INT"),
				)

				td.mockFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(nil, assert.AnError).Times(1)

				return cliCtx
			},
			errContains: "Unable to get search attributes for old domain.",
		},
		{
			name: "failed to get search attributes for new domain",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringSliceArgument(FlagSearchAttribute, "Domain:STRING", "WorkflowID:INT"),
				)
				td.mockFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(&types.GetSearchAttributesResponse{
					Keys: map[string]types.IndexedValueType{
						"Domain":     types.IndexedValueTypeString,
						"WorkflowID": types.IndexedValueTypeInt,
					},
				}, nil).Times(1)
				td.mockFrontendClient.EXPECT().GetSearchAttributes(gomock.Any()).Return(nil, assert.AnError).Times(1)

				return cliCtx
			},
			errContains: "Unable to get search attributes for new domain.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newDomainMigrationCliTestData(t)
			cliCtx := tt.testSetup(td)

			_, err := td.domainMigrationCLIImpl.SearchAttributesChecker(cliCtx)

			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}

func TestDomainConfigCheck(t *testing.T) {
	testDomain := testDomainName
	testNewDomain := testNewDomainName
	testDomainUUID := uuid.New()
	testNewDomainUUID := uuid.New()

	tests := []struct {
		name           string
		testSetup      func(td *domainMigrationCliTestData) *cli.Context
		errContains    string // empty if no error is expected
		expectedOutput string
	}{
		{
			name: "dynamic configs are equal",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagDestinationDomain, testNewDomainName),
				)

				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{Name: &testDomain}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{UUID: testDomainUUID},
				}, nil).Times(1)
				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{Name: &testNewDomain}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{UUID: testNewDomainUUID},
				}, nil).Times(1)

				td.mockAdminClient.EXPECT().GetDynamicConfig(gomock.Any(), gomock.Any()).Return(&types.GetDynamicConfigResponse{
					Value: &types.DataBlob{
						EncodingType: types.EncodingTypeThriftRW.Ptr(),
						Data:         []byte("config-value"),
					},
				}, nil).AnyTimes()

				return cliCtx
			},
			errContains: "",
		},
		{
			name: "failed to get domainID fo the current domain",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagDestinationDomain, testNewDomainName),
				)

				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{Name: &testDomain}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{UUID: testDomainUUID},
				}, assert.AnError).Times(1)
				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{Name: &testNewDomain}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{UUID: testNewDomainUUID},
				}, assert.AnError).Times(1)

				return cliCtx
			},
			errContains: "Failed to get domainID for the current domain.",
		},
		{
			name: "failed to get domainID fo the destination domain",
			testSetup: func(td *domainMigrationCliTestData) *cli.Context {
				cliCtx := clitest.NewCLIContext(
					t,
					td.app,
					clitest.StringArgument(FlagDomain, testDomain),
					clitest.StringArgument(FlagDestinationDomain, testNewDomainName),
				)

				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{Name: &testDomain}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{UUID: testDomainUUID},
				}, nil).Times(1)
				td.mockFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{Name: &testNewDomain}).Return(&types.DescribeDomainResponse{
					DomainInfo: &types.DomainInfo{UUID: testNewDomainUUID},
				}, assert.AnError).Times(1)

				return cliCtx
			},
			errContains: "Failed to get domainID for the destination domain.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newDomainMigrationCliTestData(t)
			cliCtx := tt.testSetup(td)

			_, err := td.domainMigrationCLIImpl.DynamicConfigCheck(cliCtx)

			if tt.errContains == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errContains)
			}
			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}
