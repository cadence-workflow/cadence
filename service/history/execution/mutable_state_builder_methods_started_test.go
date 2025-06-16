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

package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/config"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/shard"
)

func TestGetActiveClusterSelectionPolicy(t *testing.T) {
	tests := []struct {
		name        string
		domainEntry *cache.DomainCacheEntry
		attr        *types.WorkflowExecutionStartedEventAttributes
		mockFn      func(acm *activecluster.MockManager)
		want        *types.ActiveClusterSelectionPolicy
		wantErr     bool
	}{
		{
			name:        "local domain",
			domainEntry: constants.TestLocalDomainEntry,
			want:        nil,
		},
		{
			name:        "active-passive domain",
			domainEntry: constants.TestGlobalDomainEntry,
			want:        nil,
		},
		{
			name:        "active-active domain - attributes missing policy",
			domainEntry: constants.TestActiveActiveDomainEntry,
			attr:        &types.WorkflowExecutionStartedEventAttributes{},
			want:        nil,
		},
		{
			name:        "active-active domain - attributes with region sticky policy",
			domainEntry: constants.TestActiveActiveDomainEntry,
			attr: &types.WorkflowExecutionStartedEventAttributes{
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
					ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
					StickyRegion:                   "user-provided-region-to-be-replaced",
				},
			},
			mockFn: func(acm *activecluster.MockManager) {
				acm.EXPECT().CurrentRegion().Return("region1")
			},
			want: &types.ActiveClusterSelectionPolicy{
				ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyRegionSticky.Ptr(),
				StickyRegion:                   "region1",
			},
		},
		{
			name:        "active-active domain - attributes with external entity policy but entity type not supported",
			domainEntry: constants.TestActiveActiveDomainEntry,
			attr: &types.WorkflowExecutionStartedEventAttributes{
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
					ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyExternalEntity.Ptr(),
					ExternalEntityType:             "city",
				},
			},
			mockFn: func(acm *activecluster.MockManager) {
				acm.EXPECT().SupportedExternalEntityType(gomock.Any()).Return(false)
			},
			wantErr: true,
		},
		{
			name:        "active-active domain - attributes with external entity policy",
			domainEntry: constants.TestActiveActiveDomainEntry,
			attr: &types.WorkflowExecutionStartedEventAttributes{
				ActiveClusterSelectionPolicy: &types.ActiveClusterSelectionPolicy{
					ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyExternalEntity.Ptr(),
					ExternalEntityType:             "city",
					ExternalEntityKey:              "city-1",
				},
			},
			mockFn: func(acm *activecluster.MockManager) {
				acm.EXPECT().SupportedExternalEntityType(gomock.Any()).Return(true)
			},
			want: &types.ActiveClusterSelectionPolicy{
				ActiveClusterSelectionStrategy: types.ActiveClusterSelectionStrategyExternalEntity.Ptr(),
				ExternalEntityType:             "city",
				ExternalEntityKey:              "city-1",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShard := shard.NewTestContext(
				t,
				ctrl,
				&persistence.ShardInfo{
					ShardID:          0,
					RangeID:          1,
					TransferAckLevel: 0,
				},
				config.NewForTest(),
			)
			// set the checksum probabilities to 100% for exercising during test
			mockShard.GetConfig().MutableStateChecksumGenProbability = func(domain string) int { return 100 }
			mockShard.GetConfig().MutableStateChecksumVerifyProbability = func(domain string) int { return 100 }
			mockShard.GetConfig().EnableRetryForChecksumFailure = func(domain string) bool { return true }
			logger := log.NewNoop()

			mockShard.Resource.MatchingClient.EXPECT().AddActivityTask(gomock.Any(), gomock.Any()).Return(&types.AddActivityTaskResponse{}, nil).AnyTimes()
			mockShard.Resource.DomainCache.EXPECT().GetDomainID(constants.TestDomainName).Return(constants.TestDomainID, nil).AnyTimes()
			mockShard.Resource.DomainCache.EXPECT().GetDomainByID(constants.TestDomainID).Return(test.domainEntry, nil).AnyTimes()
			if test.mockFn != nil {
				test.mockFn(mockShard.Resource.ActiveClusterMgr)
			}
			msb := newMutableStateBuilder(mockShard, logger, test.domainEntry)
			policy, err := msb.getActiveClusterSelectionPolicy(test.attr)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.want, policy)
		})
	}
}
