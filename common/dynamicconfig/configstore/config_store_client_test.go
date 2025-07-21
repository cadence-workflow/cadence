// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	c "github.com/uber/cadence/common/dynamicconfig/configstore/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	p "github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/types"
)

const (
	retryAttempts = 2
)

type configStoreClientSuite struct {
	suite.Suite
	*require.Assertions
	client         *configStoreClient
	mockManager    *p.MockConfigStoreManager
	mockController *gomock.Controller
	doneCh         chan struct{}
}

var snapshot1 *p.DynamicConfigSnapshot

func TestConfigStoreClientSuite(t *testing.T) {
	s := new(configStoreClientSuite)
	suite.Run(t, s)
}

func (s *configStoreClientSuite) SetupSuite() {
	s.doneCh = make(chan struct{})
	s.mockController = gomock.NewController(s.T())

	mockPlugin := nosqlplugin.NewMockPlugin(s.mockController)
	mockPlugin.EXPECT().
		CreateDB(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	nosql.RegisterPlugin("cassandra", mockPlugin)
}

func (s *configStoreClientSuite) TearDownSuite() {
	close(s.doneCh)
}

func (s *configStoreClientSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	snapshot1 = &p.DynamicConfigSnapshot{
		Version: 1,
		Values: &types.DynamicConfigBlob{
			SchemaVersion: 1,
			Entries: []*types.DynamicConfigEntry{
				{
					Name: dynamicproperties.TestGetBoolPropertyKey.String(),
					Values: []*types.DynamicConfigValue{
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(false),
							},
							Filters: nil,
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(true),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "domainName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("global-samples-domain"),
									},
								},
							},
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(true),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "domainName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("samples-domain"),
									},
								},
							},
						},
					},
				},
				{
					Name: dynamicproperties.TestGetIntPropertyKey.String(),
					Values: []*types.DynamicConfigValue{
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(1000),
							},
							Filters: nil,
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(1000.1),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "domainName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("global-samples-domain"),
									},
								},
							},
						},
					},
				},
				{
					Name: dynamicproperties.TestGetFloat64PropertyKey.String(),
					Values: []*types.DynamicConfigValue{
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(12),
							},
							Filters: nil,
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper("wrong type"),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "domainName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("samples-domain"),
									},
								},
							},
						},
					},
				},
				{
					Name: dynamicproperties.TestGetStringPropertyKey.String(),
					Values: []*types.DynamicConfigValue{
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper("some random string"),
							},
							Filters: nil,
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper("constrained-string"),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "taskListName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("random tasklist"),
									},
								},
							},
						},
					},
				},
				{
					Name: dynamicproperties.TestGetMapPropertyKey.String(),
					Values: []*types.DynamicConfigValue{
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data: jsonMarshalHelper(map[string]interface{}{
									"key1": "1",
									"key2": 1,
									"key3": []interface{}{
										false,
										map[string]interface{}{
											"key4": true,
											"key5": 2.1,
										},
									},
								}),
							},
							Filters: nil,
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper("1"),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "taskListName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("random tasklist"),
									},
								},
							},
						},
					},
				},
				{
					Name: dynamicproperties.TestGetDurationPropertyKey.String(),
					Values: []*types.DynamicConfigValue{
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper("1m"),
							},
							Filters: nil,
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper("wrong duration string"),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "domainName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("samples-domain"),
									},
								},
								{
									Name: "taskListName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("longIdleTimeTaskList"),
									},
								},
							},
						},
						{
							Value: &types.DataBlob{
								EncodingType: types.EncodingTypeJSON.Ptr(),
								Data:         jsonMarshalHelper(2),
							},
							Filters: []*types.DynamicConfigFilter{
								{
									Name: "domainName",
									Value: &types.DataBlob{
										EncodingType: types.EncodingTypeJSON.Ptr(),
										Data:         jsonMarshalHelper("samples-domain"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	connections := make(map[string]config.DBShardConnection)
	connections[config.NonShardedStoreName] = config.DBShardConnection{
		NoSQLPlugin: &config.NoSQL{
			PluginName: "cassandra",
		},
	}

	var err error
	s.client, err = newConfigStoreClient(
		&c.ClientConfig{
			PollInterval:        time.Second * 2,
			UpdateRetryAttempts: retryAttempts,
			FetchTimeout:        time.Second * 1,
			UpdateTimeout:       time.Second * 1,
		},
		&config.DataStore{
			ShardedNoSQL: &config.ShardedNoSQL{
				DefaultShard: config.NonShardedStoreName,
				Connections:  connections,
			},
		}, log.NewNoop(), metrics.NewNoopMetricsClient(), p.DynamicConfig)
	s.Require().NoError(err)

	s.mockManager = p.NewMockConfigStoreManager(s.mockController)
	s.client.configStoreManager = s.mockManager
}

func defaultTestSetup(s *configStoreClientSuite) {
	s.mockManager.EXPECT().
		FetchDynamicConfig(gomock.Any(), p.DynamicConfig).
		Return(&p.FetchDynamicConfigResponse{
			Snapshot: snapshot1,
		}, nil).
		AnyTimes()
	err := s.client.startUpdate()
	s.NoError(err)
}

func (s *configStoreClientSuite) TestGetValue() {
	defaultTestSetup(s)
	v, err := s.client.GetValue(dynamicproperties.TestGetBoolPropertyKey)
	s.NoError(err)
	s.Equal(false, v)
}

func (s *configStoreClientSuite) TestGetValue_NonExistKey() {
	defaultTestSetup(s)
	v, err := s.client.GetValue(dynamicproperties.MaxRetentionDays)
	s.Error(err)
	s.Equal(dynamicproperties.MaxRetentionDays.DefaultInt(), v)
	v, err = s.client.GetValue(dynamicproperties.EnableVisibilitySampling)
	s.Error(err)
	s.Equal(dynamicproperties.EnableVisibilitySampling.DefaultBool(), v)
	v, err = s.client.GetValue(dynamicproperties.FrontendErrorInjectionRate)
	s.Error(err)
	s.Equal(dynamicproperties.FrontendErrorInjectionRate.DefaultFloat(), v)
	v, err = s.client.GetValue(dynamicproperties.WriteVisibilityStoreName)
	s.Error(err)
	s.Equal(dynamicproperties.WriteVisibilityStoreName.DefaultString(), v)
	v, err = s.client.GetValue(dynamicproperties.FrontendShutdownDrainDuration)
	s.Error(err)
	s.Equal(dynamicproperties.FrontendShutdownDrainDuration.DefaultDuration(), v)
	v, err = s.client.GetValue(dynamicproperties.RequiredDomainDataKeys)
	s.Error(err)
	s.Equal(dynamicproperties.RequiredDomainDataKeys.DefaultMap(), v)
}

func (s *configStoreClientSuite) TestGetValueWithFilters() {
	defaultTestSetup(s)

	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "global-samples-domain",
	}

	v, err := s.client.GetValueWithFilters(dynamicproperties.TestGetBoolPropertyKey, filters)
	s.NoError(err)
	s.Equal(true, v)

	filters = map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "non-exist-domain",
	}
	v, err = s.client.GetValueWithFilters(dynamicproperties.TestGetBoolPropertyKey, filters)
	s.NoError(err)
	s.Equal(false, v)

	filters = map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName:   "samples-domain",
		dynamicproperties.TaskListName: "non-exist-tasklist",
	}
	v, err = s.client.GetValueWithFilters(dynamicproperties.TestGetBoolPropertyKey, filters)
	s.NoError(err)
	s.Equal(true, v)
}

func (s *configStoreClientSuite) TestGetValueWithFilters_UnknownFilter() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName:    "global-samples-domain1",
		dynamicproperties.UnknownFilter: "unknown-filter1",
	}
	v, err := s.client.GetValueWithFilters(dynamicproperties.TestGetBoolPropertyKey, filters)
	s.NoError(err)
	s.Equal(false, v)
}

func (s *configStoreClientSuite) TestGetIntValue() {
	defaultTestSetup(s)
	v, err := s.client.GetIntValue(dynamicproperties.TestGetIntPropertyKey, nil)
	s.NoError(err)
	s.Equal(1000, v)
}

func (s *configStoreClientSuite) TestGetIntValue_FilterNotMatch() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}
	v, err := s.client.GetIntValue(dynamicproperties.TestGetIntPropertyKey, filters)
	s.NoError(err)
	s.Equal(1000, v)
}

func (s *configStoreClientSuite) TestGetIntValue_WrongType() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "global-samples-domain",
	}
	v, err := s.client.GetIntValue(dynamicproperties.TestGetIntPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetIntPropertyKey.DefaultInt(), v)
}

func (s *configStoreClientSuite) TestGetFloatValue() {
	defaultTestSetup(s)
	v, err := s.client.GetFloatValue(dynamicproperties.TestGetFloat64PropertyKey, nil)
	s.NoError(err)
	s.Equal(12.0, v)
}

func (s *configStoreClientSuite) TestGetFloatValue_WrongType() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}
	v, err := s.client.GetFloatValue(dynamicproperties.TestGetFloat64PropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetFloat64PropertyKey.DefaultFloat(), v)
}

func (s *configStoreClientSuite) TestGetBoolValue() {
	defaultTestSetup(s)
	v, err := s.client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	s.NoError(err)
	s.Equal(false, v)
}

func (s *configStoreClientSuite) TestGetStringValue() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.TaskListName: "random tasklist",
	}
	v, err := s.client.GetStringValue(dynamicproperties.TestGetStringPropertyKey, filters)
	s.NoError(err)
	s.Equal("constrained-string", v)
}

func (s *configStoreClientSuite) TestGetMapValue() {
	defaultTestSetup(s)
	v, err := s.client.GetMapValue(dynamicproperties.TestGetMapPropertyKey, nil)
	s.NoError(err)
	expectedVal := map[string]interface{}{
		"key1": "1",
		"key2": float64(1),
		"key3": []interface{}{
			false,
			map[string]interface{}{
				"key4": true,
				"key5": 2.1,
			},
		},
	}
	s.Equal(expectedVal, v)
}

func (s *configStoreClientSuite) TestGetMapValue_WrongType() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.TaskListName: "random tasklist",
	}
	v, err := s.client.GetMapValue(dynamicproperties.TestGetMapPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetMapPropertyKey.DefaultMap(), v)
}

func (s *configStoreClientSuite) TestGetDurationValue() {
	defaultTestSetup(s)
	v, err := s.client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, nil)
	s.NoError(err)
	s.Equal(time.Minute, v)
}

func (s *configStoreClientSuite) TestGetDurationValue_NotStringRepresentation() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}
	v, err := s.client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetDurationPropertyKey.DefaultDuration(), v)
}

func (s *configStoreClientSuite) TestGetDurationValue_ParseFailed() {
	defaultTestSetup(s)
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName:   "samples-domain",
		dynamicproperties.TaskListName: "longIdleTimeTaskList",
	}
	v, err := s.client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetDurationPropertyKey.DefaultDuration(), v)
}

func (s *configStoreClientSuite) TestValidateConfig_InvalidConfig() {
	err := validateClientConfig(
		&c.ClientConfig{
			PollInterval:        time.Second * 1,
			UpdateRetryAttempts: 0,
			FetchTimeout:        time.Second * 3,
			UpdateTimeout:       time.Second * 4,
		},
	)
	s.Error(err)

	err = validateClientConfig(
		&c.ClientConfig{
			PollInterval:        time.Second * 2,
			UpdateRetryAttempts: -1,
			FetchTimeout:        time.Second * 2,
			UpdateTimeout:       time.Second * 2,
		},
	)
	s.Error(err)

	err = validateClientConfig(
		&c.ClientConfig{
			PollInterval:        time.Second * 2,
			UpdateRetryAttempts: 0,
			FetchTimeout:        time.Second * 0,
			UpdateTimeout:       time.Second * 0,
		},
	)
	s.Error(err)

	err = validateClientConfig(
		&c.ClientConfig{
			PollInterval:        time.Second * 2,
			UpdateRetryAttempts: 1,
			FetchTimeout:        time.Second * 1,
			UpdateTimeout:       time.Second * 0,
		},
	)
	s.Error(err)
}

func (s *configStoreClientSuite) TestMatchFilters() {
	testCases := []struct {
		v       *types.DynamicConfigValue
		filters map[dynamicproperties.Filter]interface{}
		matched bool
	}{
		{
			v: &types.DynamicConfigValue{
				Value:   nil,
				Filters: nil,
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName: "some random domain",
			},
			matched: true,
		},
		{
			v: &types.DynamicConfigValue{
				Value: nil,
				Filters: []*types.DynamicConfigFilter{
					{
						Name: "some key",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("some value"),
						},
					},
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{},
			matched: false,
		},
		{
			v: &types.DynamicConfigValue{
				Value: nil,
				Filters: []*types.DynamicConfigFilter{
					{
						Name: "domainName",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("samples-domain"),
						},
					},
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName: "some random domain",
			},
			matched: false,
		},
		{
			v: &types.DynamicConfigValue{
				Value: nil,
				Filters: []*types.DynamicConfigFilter{
					{
						Name: "domainName",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("samples-domain"),
						},
					},
					{
						Name: "taskListName",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("sample-task-list"),
						},
					},
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName:   "samples-domain",
				dynamicproperties.TaskListName: "sample-task-list",
			},
			matched: true,
		},
		{
			v: &types.DynamicConfigValue{
				Value: nil,
				Filters: []*types.DynamicConfigFilter{
					{
						Name: "domainName",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("samples-domain"),
						},
					},
					{
						Name: "some-other-filter",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("sample-task-list"),
						},
					},
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName:   "samples-domain",
				dynamicproperties.TaskListName: "sample-task-list",
			},
			matched: false,
		},
		{
			v: &types.DynamicConfigValue{
				Value: nil,
				Filters: []*types.DynamicConfigFilter{
					{
						Name: "domainName",
						Value: &types.DataBlob{
							EncodingType: types.EncodingTypeJSON.Ptr(),
							Data:         jsonMarshalHelper("samples-domain"),
						},
					},
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.TaskListName: "sample-task-list",
			},
			matched: false,
		},
	}

	for index, tc := range testCases {
		matched := matchFilters(tc.v, tc.filters)
		s.Equal(tc.matched, matched, fmt.Sprintf("Test case %v failed", index))
	}
}

func (s *configStoreClientSuite) TestUpdateValue_NilOverwrite() {
	defaultTestSetup(s)

	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		DoAndReturn(func(_ context.Context, request *p.UpdateDynamicConfigRequest, cfgType p.ConfigType) error {
			if request.Snapshot.Values.Entries[0].Name != dynamicproperties.TestGetBoolPropertyKey.String() {
				return nil
			}
			return errors.New("entry not removed")
		}).AnyTimes()

	err := s.client.UpdateValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	s.NoError(err)
}

func (s *configStoreClientSuite) TestUpdateValue_NoRetrySuccess() {
	defaultTestSetup(s)

	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), EqSnapshotVersion(2), p.DynamicConfig).
		Return(nil).MaxTimes(1)

	values := []*types.DynamicConfigValue{
		{
			Value: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         jsonMarshalHelper(true),
			},
			Filters: nil,
		},
	}

	err := s.client.UpdateValue(dynamicproperties.TestGetBoolPropertyKey, values)
	s.NoError(err)

	snapshot2 := snapshot1
	snapshot2.Values.Entries[0].Values = values
	s.mockManager.EXPECT().
		FetchDynamicConfig(gomock.Any(), p.DynamicConfig).
		Return(&p.FetchDynamicConfigResponse{
			Snapshot: snapshot2,
		}, nil).MaxTimes(1)

	err = s.client.update()
	s.NoError(err)

	v, err := s.client.GetValue(dynamicproperties.TestGetBoolPropertyKey)
	s.NoError(err)
	s.Equal(true, v)
}

func (s *configStoreClientSuite) TestUpdateValue_SuccessNewKey() {
	values := []*types.DynamicConfigValue{
		{
			Value: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         jsonMarshalHelper(true),
			},
			Filters: nil,
		},
	}

	s.mockManager.EXPECT().
		FetchDynamicConfig(gomock.Any(), p.DynamicConfig).
		Return(&p.FetchDynamicConfigResponse{
			Snapshot: &p.DynamicConfigSnapshot{
				Version: 1,
				Values: &types.DynamicConfigBlob{
					SchemaVersion: 1,
					Entries:       nil,
				},
			},
		}, nil).
		AnyTimes()

	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		DoAndReturn(func(_ context.Context, request *p.UpdateDynamicConfigRequest, cfgType p.ConfigType) error {
			s.Equal(1, len(request.Snapshot.Values.Entries))
			s.Equal(request.Snapshot.Values.Entries[0].Values, values)
			return nil
		}).AnyTimes()

	s.client.update()
	err := s.client.UpdateValue(dynamicproperties.TestGetBoolPropertyKey, values)
	s.NoError(err)
}

func (s *configStoreClientSuite) TestUpdateValue_RetrySuccess() {
	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), EqSnapshotVersion(2), p.DynamicConfig).
		Return(&p.ConditionFailedError{}).AnyTimes()

	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), EqSnapshotVersion(3), p.DynamicConfig).
		Return(nil).AnyTimes()

	snapshot1.Version = 2
	s.mockManager.EXPECT().
		FetchDynamicConfig(gomock.Any(), p.DynamicConfig).
		Return(&p.FetchDynamicConfigResponse{
			Snapshot: snapshot1,
		}, nil).AnyTimes()

	s.client.update()

	err := s.client.UpdateValue(dynamicproperties.TestGetBoolPropertyKey, []*types.DynamicConfigValue{})
	s.NoError(err)
}

func (s *configStoreClientSuite) TestUpdateValue_RetryFailure() {
	defaultTestSetup(s)

	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		Return(&p.ConditionFailedError{}).MaxTimes(retryAttempts + 1)

	err := s.client.UpdateValue(dynamicproperties.TestGetFloat64PropertyKey, []*types.DynamicConfigValue{})
	s.Error(err)
}

func (s *configStoreClientSuite) TestUpdateValue_Timeout() {
	defaultTestSetup(s)
	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		DoAndReturn(func(_ context.Context, _ *p.UpdateDynamicConfigRequest, cfgType p.ConfigType) error {
			time.Sleep(2 * time.Second)
			return nil
		}).AnyTimes()

	err := s.client.UpdateValue(dynamicproperties.TestGetDurationPropertyKey, []*types.DynamicConfigValue{})
	s.Error(err)
}

func (s *configStoreClientSuite) TestRestoreValue_NoFilter() {
	defaultTestSetup(s)
	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		DoAndReturn(func(_ context.Context, request *p.UpdateDynamicConfigRequest, cfgType p.ConfigType) error {
			for _, entry := range request.Snapshot.Values.Entries {
				if entry.Name == dynamicproperties.TestGetBoolPropertyKey.String() {
					for _, value := range entry.Values {
						s.Equal(value.Value.Data, jsonMarshalHelper(true))
						if value.Filters == nil {
							return errors.New("fallback value not restored")
						}
					}
				}
			}
			return nil
		}).AnyTimes()

	err := s.client.RestoreValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	s.NoError(err)
}

func (s *configStoreClientSuite) TestRestoreValue_FilterNoMatch() {
	defaultTestSetup(s)

	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		DoAndReturn(func(_ context.Context, request *p.UpdateDynamicConfigRequest, cfgType p.ConfigType) error {
			for _, resEntry := range request.Snapshot.Values.Entries {
				for _, oriEntry := range snapshot1.Values.Entries {
					if oriEntry.Name == resEntry.Name {
						s.Equal(resEntry.Values, oriEntry.Values)
					}
				}
			}
			return nil
		}).AnyTimes()

	noMatchFilter := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "unknown-domain",
	}

	err := s.client.RestoreValue(dynamicproperties.TestGetBoolPropertyKey, noMatchFilter)
	s.NoError(err)
}

func (s *configStoreClientSuite) TestRestoreValue_FilterMatch() {
	defaultTestSetup(s)
	s.mockManager.EXPECT().
		UpdateDynamicConfig(gomock.Any(), gomock.Any(), p.DynamicConfig).
		DoAndReturn(func(_ context.Context, request *p.UpdateDynamicConfigRequest, cfgType p.ConfigType) error {
			for _, resEntry := range request.Snapshot.Values.Entries {
				if resEntry.Name == dynamicproperties.TestGetBoolPropertyKey.String() {
					s.Equal(2, len(resEntry.Values))
				}
			}
			return nil
		}).AnyTimes()

	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}

	err := s.client.RestoreValue(dynamicproperties.TestGetBoolPropertyKey, filters)
	s.NoError(err)
}

func (s *configStoreClientSuite) TestListValues() {
	defaultTestSetup(s)
	val, err := s.client.ListValue(nil)
	s.NoError(err)
	for _, resEntry := range val {
		for _, oriEntry := range snapshot1.Values.Entries {
			if oriEntry.Name == resEntry.Name {
				s.Equal(resEntry.Values, oriEntry.Values)
			}
		}
	}
}

func (s *configStoreClientSuite) TestListValues_EmptyCache() {
	s.mockManager.EXPECT().
		FetchDynamicConfig(gomock.Any(), p.DynamicConfig).
		Return(&p.FetchDynamicConfigResponse{
			Snapshot: &p.DynamicConfigSnapshot{
				Version: 1,
				Values: &types.DynamicConfigBlob{
					SchemaVersion: 1,
					Entries:       nil,
				},
			},
		}, nil).
		MaxTimes(1)

	s.client.update()

	val, err := s.client.ListValue(nil)
	s.NoError(err)
	s.Nil(val)
}

func (s *configStoreClientSuite) TestValidateKeyDataBlobPair() {
	tests := []struct {
		name    string
		key     dynamicproperties.Key
		blob    *types.DataBlob
		wantErr bool
	}{
		{
			name: "valid int key",
			key:  dynamicproperties.TestGetIntPropertyKey,
			blob: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         jsonMarshalHelper(10),
			},
			wantErr: false,
		},
		{
			name: "invalid int key - wrong type",
			key:  dynamicproperties.TestGetIntPropertyKey,
			blob: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         jsonMarshalHelper(true),
			},
			wantErr: true,
		},
		{
			name: "valid bool key",
			key:  dynamicproperties.TestGetBoolPropertyKey,
			blob: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         jsonMarshalHelper(true),
			},
			wantErr: false,
		},
		{
			name: "invalid bool key - wrong type",
			key:  dynamicproperties.TestGetBoolPropertyKey,
			blob: &types.DataBlob{
				EncodingType: types.EncodingTypeJSON.Ptr(),
				Data:         jsonMarshalHelper("true"),
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			err := validateKeyDataBlobPair(tc.key, tc.blob)
			if tc.wantErr {
				s.Require().Error(err, "Expected an error for case: %s", tc.name)
			} else {
				s.Require().NoError(err, "Expected no error for case: %s", tc.name)
			}
		})
	}
}

func (s *configStoreClientSuite) TestNewConfigStoreClient_NilPersistenceConfig() {
	_, err := NewConfigStoreClient(&c.ClientConfig{}, nil, log.NewNoop(), metrics.NewNoopMetricsClient(), p.DynamicConfig)
	s.Require().Error(err, "should fail when persistence config is nil")
	s.Require().EqualError(err, "persistence cfg is nil")
}

func (s *configStoreClientSuite) TestNewConfigStoreClient_MissingDefaultPersistenceConfig() {
	persistenceCfg := &config.Persistence{
		DataStores: map[string]config.DataStore{},
	}
	_, err := NewConfigStoreClient(&c.ClientConfig{}, persistenceCfg, log.NewNoop(), metrics.NewNoopMetricsClient(), p.DynamicConfig)
	s.Require().Error(err, "should fail when default persistence config is missing")
	s.Require().EqualError(err, "default persistence config missing")
}

func (s *configStoreClientSuite) TestNewConfigStoreClient_InvalidClientConfig() {
	persistenceCfg := &config.Persistence{
		DataStores: map[string]config.DataStore{
			"default": {},
		},
		DefaultStore: "default",
	}
	clientCfg := &c.ClientConfig{
		PollInterval: time.Millisecond,
	}
	logger := log.NewNoop()
	_, err := NewConfigStoreClient(clientCfg, persistenceCfg, logger, metrics.NewNoopMetricsClient(), p.DynamicConfig)
	s.Require().Error(err, "should fail when client config is invalid")
}

func jsonMarshalHelper(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

type eqSnapshotVersionMatcher struct {
	version int64
}

func (e eqSnapshotVersionMatcher) Matches(x interface{}) bool {
	arg, ok := x.(*p.UpdateDynamicConfigRequest)
	if !ok {
		return false
	}
	return e.version == arg.Snapshot.Version
}

func (e eqSnapshotVersionMatcher) String() string {
	return fmt.Sprintf("Version match %d.\n", e.version)
}

func EqSnapshotVersion(version int64) gomock.Matcher {
	return eqSnapshotVersionMatcher{version}
}
