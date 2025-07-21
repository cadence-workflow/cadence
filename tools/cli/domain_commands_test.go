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

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func (s *cliAppSuite) TestDomainRegister() {
	testCases := []testcase{
		{
			"local",
			"cadence --do test-domain domain register --global_domain false",
			"",
			func() {
				s.serverFrontendClient.EXPECT().RegisterDomain(gomock.Any(), &types.RegisterDomainRequest{
					Name:                                   "test-domain",
					WorkflowExecutionRetentionPeriodInDays: 3,
					IsGlobalDomain:                         false,
				}).Return(nil)
			},
		},
		{
			"global",
			"cadence --do test-domain domain register --global_domain true",
			"",
			func() {
				s.serverFrontendClient.EXPECT().RegisterDomain(gomock.Any(), &types.RegisterDomainRequest{
					Name:                                   "test-domain",
					WorkflowExecutionRetentionPeriodInDays: 3,
					IsGlobalDomain:                         true,
				}).Return(nil)
			},
		},
		{
			"domain with other options",
			"cadence --do test-domain domain register --global_domain true --retention 5 --desc description --active_cluster c1 --clusters c1,c2 --domain_data key1=value1,key2=value2",
			"",
			func() {
				s.serverFrontendClient.EXPECT().RegisterDomain(gomock.Any(), &types.RegisterDomainRequest{
					Name:                                   "test-domain",
					WorkflowExecutionRetentionPeriodInDays: 5,
					IsGlobalDomain:                         true,
					Description:                            "description",
					Data: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
					ActiveClusterName: "c1",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "c1"}, {ClusterName: "c2"},
					},
				}).Return(nil)
			},
		},
		{
			"domain with other options and clusters mentioned as multiple options",
			"cadence --do test-domain domain register --global_domain true --retention 5 --desc description --active_cluster c1 --cl c1 --cl c2 --domain_data key1=value1,key2=value2 --cl c3",
			"",
			func() {
				s.serverFrontendClient.EXPECT().RegisterDomain(gomock.Any(), &types.RegisterDomainRequest{
					Name:                                   "test-domain",
					WorkflowExecutionRetentionPeriodInDays: 5,
					IsGlobalDomain:                         true,
					Description:                            "description",
					Data: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
					ActiveClusterName: "c1",
					Clusters: []*types.ClusterReplicationConfiguration{
						{ClusterName: "c1"}, {ClusterName: "c2"}, {ClusterName: "c3"},
					},
				}).Return(nil)
			},
		},
		{
			"domain exists",
			"cadence --do test-domain domain register --global_domain true",
			"Domain test-domain already registered",
			func() {
				s.serverFrontendClient.EXPECT().RegisterDomain(gomock.Any(), &types.RegisterDomainRequest{
					Name:                                   "test-domain",
					WorkflowExecutionRetentionPeriodInDays: 3,
					IsGlobalDomain:                         true,
				}).Return(&types.DomainAlreadyExistsError{})
			},
		},
		{
			"failed",
			"cadence --do test-domain domain register --global_domain true",
			"Register Domain operation failed",
			func() {
				s.serverFrontendClient.EXPECT().RegisterDomain(gomock.Any(), &types.RegisterDomainRequest{
					Name:                                   "test-domain",
					WorkflowExecutionRetentionPeriodInDays: 3,
					IsGlobalDomain:                         true,
				}).Return(&types.BadRequestError{"fake error"})
			},
		},
		{
			"missing flag",
			"cadence domain register",
			"option domain is required",
			nil,
		},
		{
			"fail on extra arguments at end",
			"cadence --do test-domain domain register --global_domain true --retention 5 --desc description --active_cluster c1 --clusters c1,c2 unused_arg",
			"Domain commands cannot have arguments: <unused_arg>\nClusters are now specified as --clusters c1,c2 see help for more info",
			nil,
		},
		{
			"fail on extra arguments at in command",
			"cadence --do test-domain domain register --global_domain true --retention 5 --desc description --active_cluster c1 unused_arg --clusters c1,c2",
			"Domain commands cannot have arguments: <unused_arg --clusters c1,c2>\nClusters are now specified as --clusters c1,c2 see help for more info",
			nil,
		},
		{
			"invalid global domain flag",
			"cadence --do test-domain domain register --global_domain invalid",
			"format is invalid",
			nil,
		},
		{
			"invalid history archival status",
			"cadence --do test-domain domain register --global_domain false --history_archival_status invalid",
			"failed to parse",
			nil,
		},
		{
			"invalid visibility archival status",
			"cadence --do test-domain domain register --global_domain false --visibility_archival_status invalid",
			"failed to parse",
			nil,
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestDomainUpdate() {
	describeResponse := &types.DescribeDomainResponse{
		DomainInfo: &types.DomainInfo{
			Name:        "test-domain",
			Description: "a test domain",
			OwnerEmail:  "test@cadence.io",
			Data: map[string]string{
				"key1": "value1",
			},
		},
		Configuration: &types.DomainConfiguration{
			WorkflowExecutionRetentionPeriodInDays: 3,
		},
		ReplicationConfiguration: &types.DomainReplicationConfiguration{
			ActiveClusterName: "c1",
			Clusters: []*types.ClusterReplicationConfiguration{
				{
					ClusterName: "c1",
				},
				{
					ClusterName: "c2",
				},
			},
		},
	}

	testCases := []testcase{
		{
			"update nothing",
			"cadence --do test-domain domain update",
			"",
			func() {
				s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: common.StringPtr("test-domain"),
				}).Return(describeResponse, nil)

				s.serverFrontendClient.EXPECT().UpdateDomain(gomock.Any(), &types.UpdateDomainRequest{
					Name:                                   "test-domain",
					Description:                            common.StringPtr("a test domain"),
					OwnerEmail:                             common.StringPtr("test@cadence.io"),
					Data:                                   nil,
					WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(3),
					EmitMetric:                             common.BoolPtr(false),
					HistoryArchivalURI:                     common.StringPtr(""),
					VisibilityArchivalURI:                  common.StringPtr(""),
					ActiveClusterName:                      nil,
					Clusters:                               nil,
				}).Return(&types.UpdateDomainResponse{}, nil)
			},
		},
		{
			"update description",
			"cadence --do test-domain domain update --desc new-description",
			"",
			func() {
				s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: common.StringPtr("test-domain"),
				}).Return(describeResponse, nil)
				s.serverFrontendClient.EXPECT().UpdateDomain(gomock.Any(), &types.UpdateDomainRequest{
					Name:                                   "test-domain",
					Description:                            common.StringPtr("new-description"),
					OwnerEmail:                             common.StringPtr("test@cadence.io"),
					Data:                                   nil,
					WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(3),
					EmitMetric:                             common.BoolPtr(false),
					HistoryArchivalURI:                     common.StringPtr(""),
					VisibilityArchivalURI:                  common.StringPtr(""),
					ActiveClusterName:                      nil,
					Clusters:                               nil,
				}).Return(&types.UpdateDomainResponse{}, nil)
			},
		},
		{
			"failover",
			"cadence --do test-domain domain update --ac c2",
			"",
			func() {
				s.serverFrontendClient.EXPECT().UpdateDomain(gomock.Any(), &types.UpdateDomainRequest{
					Name:              "test-domain",
					ActiveClusterName: common.StringPtr("c2"),
				}).Return(&types.UpdateDomainResponse{}, nil)
			},
		},
		{
			"graceful failover",
			"cadence --do test-domain domain update --ac c2 --failover_type grace --failover_timeout_seconds 10",
			"",
			func() {
				s.serverFrontendClient.EXPECT().UpdateDomain(gomock.Any(), &types.UpdateDomainRequest{
					Name:                     "test-domain",
					ActiveClusterName:        common.StringPtr("c2"),
					FailoverTimeoutInSeconds: common.Int32Ptr(10),
				}).Return(&types.UpdateDomainResponse{}, nil)
			},
		},
		{
			"domain not exist",
			"cadence --do test-domain domain update --desc new-description",
			"does not exist",
			func() {
				s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: common.StringPtr("test-domain"),
				}).Return(nil, &types.EntityNotExistsError{})
			},
		},
		{
			"describe failure",
			"cadence --do test-domain domain update --desc new-description",
			"describe error",
			func() {
				s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: common.StringPtr("test-domain"),
				}).Return(nil, fmt.Errorf("describe error"))
			},
		},
		{
			"update failure",
			"cadence --do test-domain domain update --desc new-description",
			"update error",
			func() {
				s.serverFrontendClient.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: common.StringPtr("test-domain"),
				}).Return(describeResponse, nil)
				s.serverFrontendClient.EXPECT().UpdateDomain(gomock.Any(), &types.UpdateDomainRequest{
					Name:                                   "test-domain",
					Description:                            common.StringPtr("new-description"),
					OwnerEmail:                             common.StringPtr("test@cadence.io"),
					Data:                                   nil,
					WorkflowExecutionRetentionPeriodInDays: common.Int32Ptr(3),
					EmitMetric:                             common.BoolPtr(false),
					HistoryArchivalURI:                     common.StringPtr(""),
					VisibilityArchivalURI:                  common.StringPtr(""),
					ActiveClusterName:                      nil,
					Clusters:                               nil,
				}).Return(nil, fmt.Errorf("update error"))
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}

func (s *cliAppSuite) TestListDomains() {
	testCases := []testcase{
		{
			"list domains by default",
			"cadence admin domain list",
			"",
			func() {
				s.serverFrontendClient.EXPECT().ListDomains(gomock.Any(), gomock.Any()).Return(&types.ListDomainsResponse{
					Domains: []*types.DescribeDomainResponse{
						{
							DomainInfo: &types.DomainInfo{
								Name:   "test-domain",
								Status: types.DomainStatusRegistered.Ptr(),
							},
							ReplicationConfiguration: &types.DomainReplicationConfiguration{},
							Configuration:            &types.DomainConfiguration{},
							FailoverInfo:             &types.FailoverInfo{},
						},
					},
				}, nil)
			},
		},
	}

	for _, tt := range testCases {
		s.Run(tt.name, func() {
			s.runTestCase(tt)
		})
	}
}
