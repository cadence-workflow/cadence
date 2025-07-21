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

package batcher

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally"
	"go.uber.org/cadence/.gen/go/shared"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/resource"
)

func Test__Start(t *testing.T) {
	batcher, mockResource := setuptest(t)
	err := batcher.Start()
	require.NoError(t, err)
	mockResource.Finish(t)
}

func setuptest(t *testing.T) (*Batcher, *resource.Test) {
	ctrl := gomock.NewController(t)
	mockResource := resource.NewTest(t, ctrl, metrics.Worker)

	mockClientBean := client.NewMockBean(ctrl)
	mockResource.SDKClient.EXPECT().DescribeDomain(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&shared.DescribeDomainResponse{}, nil).AnyTimes()
	mockResource.SDKClient.EXPECT().PollForDecisionTask(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&shared.PollForDecisionTaskResponse{}, nil).AnyTimes()
	mockResource.SDKClient.EXPECT().PollForActivityTask(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&shared.PollForActivityTaskResponse{}, nil).AnyTimes()
	sdkClient := mockResource.GetSDKClient()
	mockClientBean.EXPECT().GetFrontendClient().Return(mockResource.FrontendClient).AnyTimes()
	mockClientBean.EXPECT().GetRemoteAdminClient(gomock.Any()).Return(mockResource.RemoteAdminClient, nil).AnyTimes()

	return New(&BootstrapParams{
		Logger:        testlogger.New(t),
		ServiceClient: sdkClient,
		ClientBean:    mockClientBean,
		TallyScope:    tally.TestScope(nil),
		Config: Config{
			ClusterMetadata: cluster.NewMetadata(
				config.ClusterGroupMetadata{
					FailoverVersionIncrement: 12,
					PrimaryClusterName:       "test-primary-cluster",
					CurrentClusterName:       "test-primary-cluster",
					ClusterGroup: map[string]config.ClusterInformation{
						"test-primary-cluster":   {},
						"test-secondary-cluster": {},
					},
				},
				nil,
				metrics.NewClient(tally.NoopScope, metrics.Worker),
				testlogger.New(t),
			),
		},
	}), mockResource
}
