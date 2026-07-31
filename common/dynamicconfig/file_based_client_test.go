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

package dynamicconfig

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

type fileBasedClientSuite struct {
	suite.Suite
	*require.Assertions
	client Client
	doneCh chan struct{}
}

func TestFileBasedClientSuite(t *testing.T) {
	s := new(fileBasedClientSuite)
	suite.Run(t, s)
}

func (s *fileBasedClientSuite) SetupSuite() {
	var err error
	s.doneCh = make(chan struct{})
	s.client, err = NewFileBasedClient(&FileBasedClientConfig{
		Filepath:     "config/testConfig.yaml",
		PollInterval: time.Second * 5,
	}, log.NewNoop(), s.doneCh)
	s.Require().NoError(err)
}

func (s *fileBasedClientSuite) TearDownSuite() {
	close(s.doneCh)
}

func (s *fileBasedClientSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *fileBasedClientSuite) TestGetValue() {
	v, err := s.client.GetValue(dynamicproperties.TestGetBoolPropertyKey)
	s.NoError(err)
	s.Equal(false, v)
}

func (s *fileBasedClientSuite) TestGetValue_NonExistKey() {
	v, err := s.client.GetValue(dynamicproperties.EnableVisibilitySampling)
	s.Error(err)
	s.Equal(dynamicproperties.EnableVisibilitySampling.DefaultBool(), v)
}

func (s *fileBasedClientSuite) TestGetValueWithFilters() {
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

func (s *fileBasedClientSuite) TestGetValueWithFilters_UnknownFilter() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName:    "global-samples-domain1",
		dynamicproperties.UnknownFilter: "unknown-filter1",
	}
	v, err := s.client.GetValueWithFilters(dynamicproperties.TestGetBoolPropertyKey, filters)
	s.NoError(err)
	s.Equal(false, v)
}

func (s *fileBasedClientSuite) TestGetIntValue() {
	v, err := s.client.GetIntValue(dynamicproperties.TestGetIntPropertyKey, nil)
	s.NoError(err)
	s.Equal(1000, v)
}

func (s *fileBasedClientSuite) TestGetIntValue_FilterNotMatch() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}
	v, err := s.client.GetIntValue(dynamicproperties.TestGetIntPropertyKey, filters)
	s.NoError(err)
	s.Equal(1000, v)
}

func (s *fileBasedClientSuite) TestGetIntValue_WrongType() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "global-samples-domain",
	}
	v, err := s.client.GetIntValue(dynamicproperties.TestGetIntPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetIntPropertyKey.DefaultInt(), v)
}

func (s *fileBasedClientSuite) TestGetFloatValue() {
	v, err := s.client.GetFloatValue(dynamicproperties.TestGetFloat64PropertyKey, nil)
	s.NoError(err)
	s.Equal(12.0, v)
}

func (s *fileBasedClientSuite) TestGetFloatValue_WrongType() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}
	v, err := s.client.GetFloatValue(dynamicproperties.TestGetFloat64PropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetFloat64PropertyKey.DefaultFloat(), v)
}

func (s *fileBasedClientSuite) TestGetBoolValue() {
	v, err := s.client.GetBoolValue(dynamicproperties.TestGetBoolPropertyKey, nil)
	s.NoError(err)
	s.Equal(false, v)
}

func (s *fileBasedClientSuite) TestGetStringValue() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.TaskListName: "random tasklist",
	}
	v, err := s.client.GetStringValue(dynamicproperties.TestGetStringPropertyKey, filters)
	s.NoError(err)
	s.Equal("constrained-string", v)
}

func (s *fileBasedClientSuite) TestGetMapValue() {
	v, err := s.client.GetMapValue(dynamicproperties.TestGetMapPropertyKey, nil)
	s.NoError(err)
	expectedVal := map[string]interface{}{
		"key1": "1",
		"key2": 1,
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

func (s *fileBasedClientSuite) TestGetMapValue_WrongType() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.TaskListName: "random tasklist",
	}
	v, err := s.client.GetMapValue(dynamicproperties.TestGetMapPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetMapPropertyKey.DefaultMap(), v)
}

func (s *fileBasedClientSuite) TestGetDurationValue() {
	v, err := s.client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, nil)
	s.NoError(err)
	s.Equal(time.Minute, v)
}

func (s *fileBasedClientSuite) TestGetDurationValue_NotStringRepresentation() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName: "samples-domain",
	}
	v, err := s.client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetDurationPropertyKey.DefaultDuration(), v)
}

func (s *fileBasedClientSuite) TestGetDurationValue_ParseFailed() {
	filters := map[dynamicproperties.Filter]interface{}{
		dynamicproperties.DomainName:   "samples-domain",
		dynamicproperties.TaskListName: "longIdleTimeTasklist",
	}
	v, err := s.client.GetDurationValue(dynamicproperties.TestGetDurationPropertyKey, filters)
	s.Error(err)
	s.Equal(dynamicproperties.TestGetDurationPropertyKey.DefaultDuration(), v)
}

func (s *fileBasedClientSuite) TestValidateConfig_ConfigNotExist() {
	_, err := NewFileBasedClient(nil, nil, nil)
	s.Error(err)
}

func (s *fileBasedClientSuite) TestValidateConfig_FileNotExist() {
	_, err := NewFileBasedClient(&FileBasedClientConfig{
		Filepath:     "file/not/exist.yaml",
		PollInterval: time.Second * 10,
	}, nil, nil)
	s.Error(err)
}

func (s *fileBasedClientSuite) TestValidateConfig_ShortPollInterval() {
	cfg := &FileBasedClientConfig{
		Filepath:     "config/testConfig.yaml",
		PollInterval: time.Second,
	}
	_, err := NewFileBasedClient(cfg, log.NewNoop(), nil)
	s.NoError(err)
	s.Equal(minPollInterval, cfg.PollInterval, "fallback to default poll interval")

}

type testMatcherValue struct {
	matches bool
}

func (v testMatcherValue) Matches(constraint interface{}) bool {
	return v.matches
}

func (s *fileBasedClientSuite) TestMatch() {
	testCases := []struct {
		v       *constrainedValue
		filters map[dynamicproperties.Filter]interface{}
		matched bool
	}{
		{
			// filter value implementing Matcher dispatches to Matches() instead of equality
			v: &constrainedValue{
				Constraints: map[string]interface{}{"domainName": "irrelevant"},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName: testMatcherValue{matches: true},
			},
			matched: true,
		},
		{
			// same Matcher dispatch, but Matches() returns false
			v: &constrainedValue{
				Constraints: map[string]interface{}{"domainName": "irrelevant"},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName: testMatcherValue{matches: false},
			},
			matched: false,
		},
		{
			v: &constrainedValue{
				Constraints: map[string]interface{}{},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName: "some random domain",
			},
			matched: true,
		},
		{
			v: &constrainedValue{
				Constraints: map[string]interface{}{"some key": "some value"},
			},
			filters: map[dynamicproperties.Filter]interface{}{},
			matched: false,
		},
		{
			v: &constrainedValue{
				Constraints: map[string]interface{}{"domainName": "samples-domain"},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName: "some random domain",
			},
			matched: false,
		},
		{
			v: &constrainedValue{
				Constraints: map[string]interface{}{
					"domainName":   "samples-domain",
					"taskListName": "sample-task-list",
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName:   "samples-domain",
				dynamicproperties.TaskListName: "sample-task-list",
			},
			matched: true,
		},
		{
			v: &constrainedValue{
				Constraints: map[string]interface{}{
					"domainName":        "samples-domain",
					"some-other-filter": "sample-task-list",
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.DomainName:   "samples-domain",
				dynamicproperties.TaskListName: "sample-task-list",
			},
			matched: false,
		},
		{
			v: &constrainedValue{
				Constraints: map[string]interface{}{
					"domainName": "samples-domain",
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.TaskListName: "sample-task-list",
			},
			matched: false,
		},
		{
			// shardID 500 is within the first 60% bucket of a 1000-shard cluster
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 60.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 500, NumberOfShards: 1000},
			},
			matched: true,
		},
		{
			// shardID 500 is outside the first 40% bucket of a 1000-shard cluster
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 40.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 500, NumberOfShards: 1000},
			},
			matched: false,
		},
		{
			// 0% matches nothing, not even shard 0
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 0.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 0, NumberOfShards: 1000},
			},
			matched: false,
		},
		{
			// 100% matches every shard
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 100.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 999, NumberOfShards: 1000},
			},
			matched: true,
		},
		{
			// out-of-range percentage (>100) fails closed, never matches
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 150.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 5, NumberOfShards: 1000},
			},
			matched: false,
		},
		{
			// out-of-range percentage (<0) fails closed, never matches
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": -5.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 5, NumberOfShards: 1000},
			},
			matched: false,
		},
		{
			// shardIDPercentage composes with other filters via AND
			v: &constrainedValue{
				Constraints: map[string]interface{}{
					"shardIDPercentage": 60.0,
					"domainName":        "samples-domain",
				},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 500, NumberOfShards: 1000},
				dynamicproperties.DomainName:        "some other domain",
			},
			matched: false,
		},
		{
			// raising the percentage from 10 to 11 only ever adds shard 105, matching test above the monotonicity boundary
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 11.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 105, NumberOfShards: 1000},
			},
			matched: true,
		},
		{
			// bucketing is relative to the real shard count, not a fixed 1000: with 8 shards,
			// shard 1 is 12.5%, which falls inside a 50% threshold
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 50.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 1, NumberOfShards: 8},
			},
			matched: true,
		},
		{
			// with only 8 shards, shard 5 is 62.5%, which falls outside a 50% threshold
			// (the old fixed-1000 bucketing would have wrongly matched, since 5 % 1000 = 5 < 500)
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 50.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 5, NumberOfShards: 8},
			},
			matched: false,
		},
		{
			// numberOfShards unset (zero value) fails closed
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 100.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 5, NumberOfShards: 0},
			},
			matched: false,
		},
		{
			// negative numberOfShards fails closed
			v: &constrainedValue{
				Constraints: map[string]interface{}{"shardIDPercentage": 100.0},
			},
			filters: map[dynamicproperties.Filter]interface{}{
				dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: 5, NumberOfShards: -1},
			},
			matched: false,
		},
	}

	for index, tc := range testCases {
		matched := match(tc.v, tc.filters)
		s.Equal(tc.matched, matched, fmt.Sprintf("Test case %v failved", index))
	}
}

// TestShardIDPercentageTiers verifies that multiple shardIDPercentage entries under the
// same key, listed in ascending threshold order, behave as non-overlapping tiers: the
// first entry whose threshold the shard is below wins, since getValueWithFilters returns
// on the first match in list order.
func (s *fileBasedClientSuite) TestShardIDPercentageTiers() {
	client := &fileBasedClient{logger: log.NewNoop()}
	key := dynamicproperties.TestGetIntPropertyFilteredByShardIDKey
	err := client.storeValues(map[string][]*constrainedValue{
		key.String(): {
			{Value: 1, Constraints: map[string]interface{}{"shardIDPercentage": 10.0}}, // [0%, 10%)
			{Value: 2, Constraints: map[string]interface{}{"shardIDPercentage": 20.0}}, // [10%, 20%)
			{Value: 3, Constraints: map[string]interface{}{}},                          // default: [20%, 100%)
		},
	})
	s.Require().NoError(err)

	tests := []struct {
		shardID  int
		expected int
	}{
		{shardID: 50, expected: 1},  // 5% -> first tier
		{shardID: 99, expected: 1},  // 9.9% -> first tier
		{shardID: 100, expected: 2}, // 10% -> second tier (first tier's "< 10%" excludes it)
		{shardID: 199, expected: 2}, // 19.9% -> second tier
		{shardID: 200, expected: 3}, // 20% -> default, since neither tier's threshold covers it
		{shardID: 999, expected: 3}, // 99.9% -> default
	}
	for _, tc := range tests {
		v, err := client.GetIntValue(key, map[dynamicproperties.Filter]interface{}{
			dynamicproperties.ShardIDPercentage: dynamicproperties.ShardIDPercentageValue{ShardID: tc.shardID, NumberOfShards: 1000},
		})
		s.NoError(err)
		s.Equal(tc.expected, v, "shardID %d", tc.shardID)
	}
}

func (s *fileBasedClientSuite) TestUpdateConfig() {
	client := s.client.(*fileBasedClient)
	key := dynamicproperties.ValidSearchAttributes

	// pre-check existing config
	current, err := client.GetMapValue(key, nil)
	s.NoError(err)
	currentDomainVal, ok := current["DomainID"]
	s.True(ok)
	s.Equal(1, currentDomainVal)
	_, ok = current["WorkflowID"]
	s.False(ok)

	// update config
	v := map[string]interface{}{
		"WorkflowID": 1,
		"DomainID":   2,
	}
	err = client.UpdateValue(key, v)
	s.NoError(err)

	// verify update result
	current, err = client.GetMapValue(key, nil)
	s.NoError(err)
	currentDomainVal, ok = current["DomainID"]
	s.True(ok)
	s.Equal(2, currentDomainVal)
	currentWorkflowIDVal, ok := current["WorkflowID"]
	s.True(ok)
	s.Equal(1, currentWorkflowIDVal)

	// revert test file back
	v = map[string]interface{}{
		"DomainID": 1,
	}
	err = client.UpdateValue(key, v)
	s.NoError(err)
}
