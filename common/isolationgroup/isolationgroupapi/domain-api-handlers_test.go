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

package isolationgroupapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

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

func TestUpdateDomainState(t *testing.T) {

	tests := map[string]struct {
		in                      types.UpdateDomainIsolationGroupsRequest
		domainHandlerAffordance func(h *domain.MockHandler)
		expectedErr             error
	}{
		"updating normal value": {
			in: types.UpdateDomainIsolationGroupsRequest{
				Domain: "domain",
				IsolationGroups: types.IsolationGroupConfiguration{
					"zone-1": {Name: "zone-1", State: types.IsolationGroupStateHealthy},
					"zone-2": {Name: "zone-2", State: types.IsolationGroupStateDrained},
				}},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				h.EXPECT().UpdateIsolationGroups(gomock.Any(), types.UpdateDomainIsolationGroupsRequest{
					Domain: "domain",
					IsolationGroups: types.IsolationGroupConfiguration{
						"zone-1": {Name: "zone-1", State: types.IsolationGroupStateHealthy},
						"zone-2": {Name: "zone-2", State: types.IsolationGroupStateDrained},
					},
				}).Return(nil)
			},
		},
		"empty value - ie removing isolation groups": {
			in: types.UpdateDomainIsolationGroupsRequest{
				Domain:          "domain",
				IsolationGroups: types.IsolationGroupConfiguration{},
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				h.EXPECT().UpdateIsolationGroups(gomock.Any(), types.UpdateDomainIsolationGroupsRequest{
					Domain:          "domain",
					IsolationGroups: types.IsolationGroupConfiguration{},
				}).Return(nil)
			},
		},
		"error state - nil value - should not happen": {
			in: types.UpdateDomainIsolationGroupsRequest{
				Domain:          "domain",
				IsolationGroups: nil,
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				var ig types.IsolationGroupConfiguration
				h.EXPECT().UpdateIsolationGroups(gomock.Any(), types.UpdateDomainIsolationGroupsRequest{
					Domain:          "domain",
					IsolationGroups: ig, // nil
				}).Return(nil)
			},
		},
		"error returned": {
			in: types.UpdateDomainIsolationGroupsRequest{
				Domain:          "domain",
				IsolationGroups: nil,
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				var ig types.IsolationGroupConfiguration
				h.EXPECT().UpdateIsolationGroups(gomock.Any(), types.UpdateDomainIsolationGroupsRequest{
					Domain:          "domain",
					IsolationGroups: ig,
				}).Return(assert.AnError)
			},
			expectedErr: assert.AnError,
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			dcMock := dynamicconfig.NewMockClient(gomock.NewController(t))
			domainHandlerMock := domain.NewMockHandler(ctrl)
			td.domainHandlerAffordance(domainHandlerMock)
			handler := handlerImpl{
				log:                        testlogger.New(t),
				domainHandler:              domainHandlerMock,
				globalIsolationGroupDrains: dcMock,
			}
			err := handler.UpdateDomainState(context.Background(), td.in)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestGetDomainState(t *testing.T) {

	validCfg := types.IsolationGroupConfiguration{
		"zone-1": types.IsolationGroupPartition{
			Name:  "zone-1",
			State: types.IsolationGroupStateDrained,
		},
		"zone-2": types.IsolationGroupPartition{
			Name:  "zone-2",
			State: types.IsolationGroupStateHealthy,
		},
	}

	domainName := "domain"

	tests := map[string]struct {
		in                      types.GetDomainIsolationGroupsRequest
		expected                *types.GetDomainIsolationGroupsResponse
		domainHandlerAffordance func(h *domain.MockHandler)
		expectedErr             error
	}{
		"normal value being returned - valid config present": {
			in: types.GetDomainIsolationGroupsRequest{
				Domain: domainName,
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				h.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					Configuration: &types.DomainConfiguration{
						IsolationGroups: &validCfg,
					},
				}, nil)
			},
			expected: &types.GetDomainIsolationGroupsResponse{IsolationGroups: validCfg},
		},
		"normal value being returned - no config present - should just return empty": {
			in: types.GetDomainIsolationGroupsRequest{
				Domain: domainName,
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				h.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					Configuration: &types.DomainConfiguration{},
				}, nil)
			},
			expected: &types.GetDomainIsolationGroupsResponse{},
		},
		"normal value being returned - no config present - should just return nil - 2": {
			in: types.GetDomainIsolationGroupsRequest{
				Domain: domainName,
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				h.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(&types.DescribeDomainResponse{
					Configuration: &types.DomainConfiguration{
						IsolationGroups: &types.IsolationGroupConfiguration{},
					},
				}, nil)
			},
			expected: &types.GetDomainIsolationGroupsResponse{
				IsolationGroups: types.IsolationGroupConfiguration{},
			},
		},
		"an error was returned": {
			in: types.GetDomainIsolationGroupsRequest{
				Domain: domainName,
			},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				h.EXPECT().DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
					Name: &domainName,
				}).Return(nil, assert.AnError)
			},
			expectedErr: assert.AnError,
		},
		"invalid input - nil input": {
			in: types.GetDomainIsolationGroupsRequest{},
			domainHandlerAffordance: func(h *domain.MockHandler) {
				n := ""
				h.EXPECT().
					DescribeDomain(gomock.Any(), &types.DescribeDomainRequest{
						Name: &n,
					}).
					Return(nil, nil)
			},
			expected: &types.GetDomainIsolationGroupsResponse{},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			domainHandlerMock := domain.NewMockHandler(ctrl)
			td.domainHandlerAffordance(domainHandlerMock)
			handler := handlerImpl{
				log:           testlogger.New(t),
				domainHandler: domainHandlerMock,
			}
			res, err := handler.GetDomainState(context.Background(), td.in)
			assert.Equal(t, td.expected, res)
			if td.expectedErr != nil {
				assert.Equal(t, td.expectedErr.Error(), err.Error())
			}
		})
	}
}
