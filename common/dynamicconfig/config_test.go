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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

type configSuite struct {
	suite.Suite
	client *inMemoryClient
	cln    *Collection
}

func TestConfigSuite(t *testing.T) {
	s := new(configSuite)
	suite.Run(t, s)
}

func (s *configSuite) SetupTest() {
	s.client = NewInMemoryClient().(*inMemoryClient)
	logger := log.NewNoop()
	s.cln = NewCollection(s.client, logger)
}

func (s *configSuite) TestGetProperty() {
	key := dynamicproperties.TestGetStringPropertyKey
	value := s.cln.GetProperty(key)
	s.Equal(key.DefaultValue(), value())
	s.client.SetValue(key, "b")
	s.Equal("b", value())
}

func (s *configSuite) TestGetStringProperty() {
	key := dynamicproperties.TestGetStringPropertyKey
	value := s.cln.GetStringProperty(key)
	s.Equal(key.DefaultValue(), value())
	s.client.SetValue(key, "b")
	s.Equal("b", value())
}

func (s *configSuite) TestGetIntProperty() {
	key := dynamicproperties.TestGetIntPropertyKey
	value := s.cln.GetIntProperty(key)
	s.Equal(key.DefaultInt(), value())
	s.client.SetValue(key, 50)
	s.Equal(50, value())
}

func (s *configSuite) TestGetIntPropertyFilteredByDomain() {
	key := dynamicproperties.TestGetIntPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetIntPropertyFilteredByDomain(key)
	s.Equal(key.DefaultInt(), value(domain))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain))
}

func (s *configSuite) TestGetIntPropertyFilteredByWorkflowType() {
	key := dynamicproperties.TestGetIntPropertyFilteredByWorkflowTypeKey
	domain := "testDomain"
	workflowType := "testWorkflowType"
	value := s.cln.GetIntPropertyFilteredByWorkflowType(key)
	s.Equal(key.DefaultInt(), value(domain, workflowType))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain, workflowType))
}

func (s *configSuite) TestGetIntPropertyFilteredByShardID() {
	key := dynamicproperties.TestGetIntPropertyFilteredByShardIDKey
	shardID := 1
	value := s.cln.GetIntPropertyFilteredByShardID(key)
	s.Equal(key.DefaultInt(), value(shardID))
	s.client.SetValue(key, 10)
	s.Equal(10, value(shardID))
}

func (s *configSuite) TestGetStringPropertyFnWithDomainFilter() {
	key := dynamicproperties.DefaultEventEncoding
	domain := "testDomain"
	value := s.cln.GetStringPropertyFilteredByDomain(key)
	s.Equal(key.DefaultString(), value(domain))
	s.client.SetValue(key, "efg")
	s.Equal("efg", value(domain))
}

func (s *configSuite) TestGetStringPropertyFnByTaskListInfo() {
	key := dynamicproperties.TasklistLoadBalancerStrategy
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetStringPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultString(), value(domain, taskList, taskType))
	s.client.SetValue(key, "round-robin")
	s.Equal("round-robin", value(domain, taskList, taskType))
}

func (s *configSuite) TestGetStringPropertyFnWithNamespaceFilter() {
	key := dynamicproperties.ShardDistributorMigrationMode
	namespace := "testService"
	value := s.cln.GetStringPropertyFilteredByNamespace(key)
	s.Equal(key.DefaultString(), value(namespace))
	s.client.SetValue(key, "efg")
	s.Equal("efg", value(namespace))
}

func (s *configSuite) TestGetStringPropertyFilteredByRatelimitKey() {
	key := dynamicproperties.FrontendGlobalRatelimiterMode
	ratelimitKey := "user:testDomain"
	value := s.cln.GetStringPropertyFilteredByRatelimitKey(key)
	s.Equal(key.DefaultString(), value(ratelimitKey))
	s.client.SetValue(key, "fake-mode")
	s.Equal("fake-mode", value(ratelimitKey))
}

func (s *configSuite) TestGetIntPropertyFilteredByTaskListInfo() {
	key := dynamicproperties.TestGetIntPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetIntPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultInt(), value(domain, taskList, taskType))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetFloat64Property() {
	key := dynamicproperties.TestGetFloat64PropertyKey
	value := s.cln.GetFloat64Property(key)
	s.Equal(key.DefaultFloat(), value())
	s.client.SetValue(key, 0.01)
	s.Equal(0.01, value())
}

func (s *configSuite) TestGetFloat64PropertyFilteredByShardID() {
	key := dynamicproperties.TestGetFloat64PropertyFilteredByShardIDKey
	shardID := 1
	value := s.cln.GetFloat64PropertyFilteredByShardID(key)
	s.Equal(key.DefaultFloat(), value(shardID))
	s.client.SetValue(key, 0.01)
	s.Equal(0.01, value(shardID))
}

func (s *configSuite) TestGetBoolProperty() {
	key := dynamicproperties.TestGetBoolPropertyKey
	value := s.cln.GetBoolProperty(key)
	s.Equal(key.DefaultBool(), value())
	s.client.SetValue(key, false)
	s.Equal(false, value())
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomainID() {
	key := dynamicproperties.TestGetBoolPropertyFilteredByDomainIDKey
	domainID := "testDomainID"
	value := s.cln.GetBoolPropertyFilteredByDomainID(key)
	s.Equal(key.DefaultBool(), value(domainID))
	s.client.SetValue(key, false)
	s.Equal(false, value(domainID))
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomain() {
	key := dynamicproperties.TestGetBoolPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetBoolPropertyFilteredByDomain(key)
	s.Equal(key.DefaultBool(), value(domain))
	s.client.SetValue(key, true)
	s.Equal(true, value(domain))
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomainIDAndWorkflowID() {
	key := dynamicproperties.TestGetBoolPropertyFilteredByDomainIDAndWorkflowIDKey
	domainID := "testDomainID"
	workflowID := "testWorkflowID"
	value := s.cln.GetBoolPropertyFilteredByDomainIDAndWorkflowID(key)
	s.Equal(key.DefaultBool(), value(domainID, workflowID))
	s.client.SetValue(key, true)
	s.Equal(true, value(domainID, workflowID))
}

func (s *configSuite) TestGetBoolPropertyFilteredByTaskListInfo() {
	key := dynamicproperties.TestGetBoolPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetBoolPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultBool(), value(domain, taskList, taskType))
	s.client.SetValue(key, true)
	s.Equal(true, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetBoolPropertyFilteredByShardID() {
	key := dynamicproperties.TestGetBoolPropertyFilteredByShardIDKey
	shardID := 1
	value := s.cln.GetBoolPropertyFilteredByShardID(key)
	s.Equal(key.DefaultBool(), value(shardID))
	s.client.SetValue(key, true)
	s.Equal(true, value(shardID))
}

func (s *configSuite) TestGetDurationProperty() {
	key := dynamicproperties.TestGetDurationPropertyKey
	value := s.cln.GetDurationProperty(key)
	s.Equal(key.DefaultDuration(), value())
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value())
}

func (s *configSuite) TestGetDurationPropertyFilteredByDomain() {
	key := dynamicproperties.TestGetDurationPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetDurationPropertyFilteredByDomain(key)
	s.Equal(key.DefaultDuration(), value(domain))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain))
}

func (s *configSuite) TestGetDurationPropertyFilteredByDomainID() {
	key := dynamicproperties.TestGetDurationPropertyFilteredByDomainIDKey
	domain := "testDomainID"
	value := s.cln.GetDurationPropertyFilteredByDomainID(key)
	s.Equal(key.DefaultDuration(), value(domain))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain))
}

func (s *configSuite) TestGetDurationPropertyFilteredByShardID() {
	key := dynamicproperties.TestGetDurationPropertyFilteredByShardID
	shardID := 1
	value := s.cln.GetDurationPropertyFilteredByShardID(key)
	s.Equal(key.DefaultDuration(), value(shardID))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(shardID))
}

func (s *configSuite) TestGetDurationPropertyFilteredByTaskListInfo() {
	key := dynamicproperties.TestGetDurationPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetDurationPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultDuration(), value(domain, taskList, taskType))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetDurationPropertyFilteredByWorkflowType() {
	key := dynamicproperties.TestGetDurationPropertyFilteredByWorkflowTypeKey
	domain := "testDomain"
	workflowType := "testWorkflowType"
	value := s.cln.GetDurationPropertyFilteredByWorkflowType(key)
	s.Equal(key.DefaultDuration(), value(domain, workflowType))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain, workflowType))
}

func (s *configSuite) TestGetMapProperty() {
	key := dynamicproperties.TestGetMapPropertyKey
	val := map[string]interface{}{
		"testKey": 123,
	}
	value := s.cln.GetMapProperty(key)
	s.Equal(key.DefaultMap(), value())
	val["testKey"] = "321"
	s.client.SetValue(key, val)
	s.Equal(val, value())
	s.Equal("321", value()["testKey"])
}

func (s *configSuite) TestGetMapPropertyFilteredByDomain() {
	key := dynamicproperties.TestGetMapPropertyKey
	domain := "testDomain"
	val := map[string]interface{}{
		"testKey": 123,
	}
	value := s.cln.GetMapPropertyFilteredByDomain(key)
	s.Equal(key.DefaultMap(), value(domain))
	val["testKey"] = "321"
	s.client.SetValue(key, val)
	s.Equal(val, value(domain))
	s.Equal("321", value(domain)["testKey"])
}

func (s *configSuite) TestGetListProperty() {
	key := dynamicproperties.TestGetListPropertyKey
	arr := []interface{}{}
	value := s.cln.GetListProperty(key)
	s.Equal(key.DefaultList(), value())
	arr = append(arr, 1)
	s.client.SetValue(key, arr)
	s.Equal(1, len(value()))
	s.Equal(1, value()[0])
}

func (s *configSuite) TestUpdateConfig() {
	key := dynamicproperties.TestGetBoolPropertyKey
	value := s.cln.GetBoolProperty(key)
	err := s.client.UpdateValue(key, false)
	s.NoError(err)
	s.Equal(false, value())
	err = s.client.UpdateValue(key, true)
	s.NoError(err)
	s.Equal(true, value())
}

func TestDynamicConfigKeyIsMapped(t *testing.T) {
	for i := dynamicproperties.UnknownIntKey + 1; i < dynamicproperties.LastIntKey; i++ {
		key, ok := dynamicproperties.IntKeys[i]
		require.True(t, ok, "missing IntKey: %d", i)
		require.NotEmpty(t, key, "empty IntKey: %d", i)
	}
	for i := dynamicproperties.UnknownBoolKey + 1; i < dynamicproperties.LastBoolKey; i++ {
		key, ok := dynamicproperties.BoolKeys[i]
		require.True(t, ok, "missing BoolKey: %d", i)
		require.NotEmpty(t, key, "empty BoolKey: %d", i)
	}
	for i := dynamicproperties.UnknownFloatKey + 1; i < dynamicproperties.LastFloatKey; i++ {
		key, ok := dynamicproperties.FloatKeys[i]
		require.True(t, ok, "missing FloatKey: %d", i)
		require.NotEmpty(t, key, "empty FloatKey: %d", i)
	}
	for i := dynamicproperties.UnknownStringKey + 1; i < dynamicproperties.LastStringKey; i++ {
		key, ok := dynamicproperties.StringKeys[i]
		require.True(t, ok, "missing StringKey: %d", i)
		require.NotEmpty(t, key, "empty StringKey: %d", i)
	}
	for i := dynamicproperties.UnknownDurationKey + 1; i < dynamicproperties.LastDurationKey; i++ {
		key, ok := dynamicproperties.DurationKeys[i]
		require.True(t, ok, "missing DurationKey: %d", i)
		require.NotEmpty(t, key, "empty DurationKey: %d", i)
	}
	for i := dynamicproperties.UnknownMapKey + 1; i < dynamicproperties.LastMapKey; i++ {
		key, ok := dynamicproperties.MapKeys[i]
		require.True(t, ok, "missing MapKey: %d", i)
		require.NotEmpty(t, key, "empty MapKey: %d", i)
	}
}
