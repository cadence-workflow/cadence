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

func (s *fileBasedClientSuite) TestMatch() {
	testCases := []struct {
		v       *constrainedValue
		filters map[dynamicproperties.Filter]interface{}
		matched bool
	}{
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
	}

	for index, tc := range testCases {
		matched := match(tc.v, tc.filters)
		s.Equal(tc.matched, matched, fmt.Sprintf("Test case %v failved", index))
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
