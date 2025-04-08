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
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestUpdateGlobalState(t *testing.T) {

	tests := map[string]struct {
		in           types.UpdateGlobalIsolationGroupsRequest
		dcAffordance func(client *dynamicconfig.MockClient)
		expectedErr  error
	}{
		"updating normal value": {
			in: types.UpdateGlobalIsolationGroupsRequest{IsolationGroups: types.IsolationGroupConfiguration{
				"zone-1": {Name: "zone-1", State: types.IsolationGroupStateHealthy},
				"zone-2": {Name: "zone-2", State: types.IsolationGroupStateDrained},
			}},
			dcAffordance: func(client *dynamicconfig.MockClient) {
				client.EXPECT().UpdateValue(
					dynamicproperties.DefaultIsolationGroupConfigStoreManagerGlobalMapping,
					gomock.Any(), // covering the mapping in the mapper unit-test instead
				)
			},
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			dcMock := dynamicconfig.NewMockClient(gomock.NewController(t))
			td.dcAffordance(dcMock)
			handler := handlerImpl{
				log:                        testlogger.New(t),
				globalIsolationGroupDrains: dcMock,
			}
			err := handler.UpdateGlobalState(context.Background(), td.in)
			assert.Equal(t, td.expectedErr, err)
		})
	}
}

func TestGetGlobalState(t *testing.T) {

	validInput := types.IsolationGroupConfiguration{
		"zone-1": types.IsolationGroupPartition{
			Name:  "zone-1",
			State: types.IsolationGroupStateDrained,
		},
		"zone-2": types.IsolationGroupPartition{
			Name:  "zone-2",
			State: types.IsolationGroupStateHealthy,
		},
	}

	validCfg, _ := MapUpdateGlobalIsolationGroupsRequest(validInput)

	validCfgData := validCfg[0].Value.GetData()
	dynamicConfigResponse := []interface{}{}
	json.Unmarshal(validCfgData, &dynamicConfigResponse)

	tests := map[string]struct {
		in           types.GetGlobalIsolationGroupsRequest
		dcAffordance func(client *dynamicconfig.MockClient)
		expected     *types.GetGlobalIsolationGroupsResponse
		expectedErr  error
	}{
		"updating normal value": {
			in: types.GetGlobalIsolationGroupsRequest{},
			dcAffordance: func(client *dynamicconfig.MockClient) {
				client.EXPECT().GetListValue(
					dynamicproperties.DefaultIsolationGroupConfigStoreManagerGlobalMapping,
					gomock.Any(),
				).Return(dynamicConfigResponse, nil)
			},
			expected: &types.GetGlobalIsolationGroupsResponse{IsolationGroups: validInput},
		},
		"an error was returned": {
			in: types.GetGlobalIsolationGroupsRequest{},
			dcAffordance: func(client *dynamicconfig.MockClient) {
				client.EXPECT().GetListValue(
					dynamicproperties.DefaultIsolationGroupConfigStoreManagerGlobalMapping,
					gomock.Any(),
				).Return(nil, errors.New("an error"))
			},
			expectedErr: errors.New("failed to get global isolation groups from datastore: an error"),
		},
	}

	for name, td := range tests {
		t.Run(name, func(t *testing.T) {
			dcMock := dynamicconfig.NewMockClient(gomock.NewController(t))
			td.dcAffordance(dcMock)
			handler := handlerImpl{
				log:                        testlogger.New(t),
				globalIsolationGroupDrains: dcMock,
			}
			res, err := handler.GetGlobalState(context.Background())
			assert.Equal(t, td.expected, res)
			if td.expectedErr != nil {
				// only compare strings because full wrapping makes it fiddly otherwise
				assert.Equal(t, td.expectedErr.Error(), err.Error())
			}
		})
	}
}
