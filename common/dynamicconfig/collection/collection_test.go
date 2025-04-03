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

package collection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
)

type configSuite struct {
	suite.Suite
	client *dynamicconfig.InMemory
	cln    *Collection
}

func TestConfigSuite(t *testing.T) {
	s := new(configSuite)
	suite.Run(t, s)
}

func (s *configSuite) SetupTest() {
	s.client = dynamicconfig.NewInMemoryClient().(*dynamicconfig.InMemory)
	logger := log.NewNoop()
	s.cln = NewCollection(s.client, logger)
}

func (s *configSuite) TestGetProperty() {
	key := dynamicconfig.TestGetStringPropertyKey
	value := s.cln.GetProperty(key)
	s.Equal(key.DefaultValue(), value())
	s.client.SetValue(key, "b")
	s.Equal("b", value())
}

func (s *configSuite) TestGetStringProperty() {
	key := dynamicconfig.TestGetStringPropertyKey
	value := s.cln.GetStringProperty(key)
	s.Equal(key.DefaultValue(), value())
	s.client.SetValue(key, "b")
	s.Equal("b", value())
}

func (s *configSuite) TestGetIntProperty() {
	key := dynamicconfig.TestGetIntPropertyKey
	value := s.cln.GetIntProperty(key)
	s.Equal(key.DefaultInt(), value())
	s.client.SetValue(key, 50)
	s.Equal(50, value())
}

func (s *configSuite) TestGetIntPropertyFilteredByDomain() {
	key := dynamicconfig.TestGetIntPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetIntPropertyFilteredByDomain(key)
	s.Equal(key.DefaultInt(), value(domain))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain))
}

func (s *configSuite) TestGetIntPropertyFilteredByWorkflowType() {
	key := dynamicconfig.TestGetIntPropertyFilteredByWorkflowTypeKey
	domain := "testDomain"
	workflowType := "testWorkflowType"
	value := s.cln.GetIntPropertyFilteredByWorkflowType(key)
	s.Equal(key.DefaultInt(), value(domain, workflowType))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain, workflowType))
}

func (s *configSuite) TestGetIntPropertyFilteredByShardID() {
	key := dynamicconfig.TestGetIntPropertyFilteredByShardIDKey
	shardID := 1
	value := s.cln.GetIntPropertyFilteredByShardID(key)
	s.Equal(key.DefaultInt(), value(shardID))
	s.client.SetValue(key, 10)
	s.Equal(10, value(shardID))
}

func (s *configSuite) TestGetStringPropertyFnWithDomainFilter() {
	key := dynamicconfig.DefaultEventEncoding
	domain := "testDomain"
	value := s.cln.GetStringPropertyFilteredByDomain(key)
	s.Equal(key.DefaultString(), value(domain))
	s.client.SetValue(key, "efg")
	s.Equal("efg", value(domain))
}

func (s *configSuite) TestGetStringPropertyFnByTaskListInfo() {
	key := dynamicconfig.TasklistLoadBalancerStrategy
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetStringPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultString(), value(domain, taskList, taskType))
	s.client.SetValue(key, "round-robin")
	s.Equal("round-robin", value(domain, taskList, taskType))
}

func (s *configSuite) TestGetStringPropertyFilteredByRatelimitKey() {
	key := dynamicconfig.FrontendGlobalRatelimiterMode
	ratelimitKey := "user:testDomain"
	value := s.cln.GetStringPropertyFilteredByRatelimitKey(key)
	s.Equal(key.DefaultString(), value(ratelimitKey))
	s.client.SetValue(key, "fake-mode")
	s.Equal("fake-mode", value(ratelimitKey))
}

func (s *configSuite) TestGetIntPropertyFilteredByTaskListInfo() {
	key := dynamicconfig.TestGetIntPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetIntPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultInt(), value(domain, taskList, taskType))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetFloat64Property() {
	key := dynamicconfig.TestGetFloat64PropertyKey
	value := s.cln.GetFloat64Property(key)
	s.Equal(key.DefaultFloat(), value())
	s.client.SetValue(key, 0.01)
	s.Equal(0.01, value())
}

func (s *configSuite) TestGetFloat64PropertyFilteredByShardID() {
	key := dynamicconfig.TestGetFloat64PropertyFilteredByShardIDKey
	shardID := 1
	value := s.cln.GetFloat64PropertyFilteredByShardID(key)
	s.Equal(key.DefaultFloat(), value(shardID))
	s.client.SetValue(key, 0.01)
	s.Equal(0.01, value(shardID))
}

func (s *configSuite) TestGetBoolProperty() {
	key := dynamicconfig.TestGetBoolPropertyKey
	value := s.cln.GetBoolProperty(key)
	s.Equal(key.DefaultBool(), value())
	s.client.SetValue(key, false)
	s.Equal(false, value())
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomainID() {
	key := dynamicconfig.TestGetBoolPropertyFilteredByDomainIDKey
	domainID := "testDomainID"
	value := s.cln.GetBoolPropertyFilteredByDomainID(key)
	s.Equal(key.DefaultBool(), value(domainID))
	s.client.SetValue(key, false)
	s.Equal(false, value(domainID))
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomain() {
	key := dynamicconfig.TestGetBoolPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetBoolPropertyFilteredByDomain(key)
	s.Equal(key.DefaultBool(), value(domain))
	s.client.SetValue(key, true)
	s.Equal(true, value(domain))
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomainIDAndWorkflowID() {
	key := dynamicconfig.TestGetBoolPropertyFilteredByDomainIDAndWorkflowIDKey
	domainID := "testDomainID"
	workflowID := "testWorkflowID"
	value := s.cln.GetBoolPropertyFilteredByDomainIDAndWorkflowID(key)
	s.Equal(key.DefaultBool(), value(domainID, workflowID))
	s.client.SetValue(key, true)
	s.Equal(true, value(domainID, workflowID))
}

func (s *configSuite) TestGetBoolPropertyFilteredByTaskListInfo() {
	key := dynamicconfig.TestGetBoolPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetBoolPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultBool(), value(domain, taskList, taskType))
	s.client.SetValue(key, true)
	s.Equal(true, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetDurationProperty() {
	key := dynamicconfig.TestGetDurationPropertyKey
	value := s.cln.GetDurationProperty(key)
	s.Equal(key.DefaultDuration(), value())
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value())
}

func (s *configSuite) TestGetDurationPropertyFilteredByDomain() {
	key := dynamicconfig.TestGetDurationPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetDurationPropertyFilteredByDomain(key)
	s.Equal(key.DefaultDuration(), value(domain))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain))
}

func (s *configSuite) TestGetDurationPropertyFilteredByDomainID() {
	key := dynamicconfig.TestGetDurationPropertyFilteredByDomainIDKey
	domain := "testDomainID"
	value := s.cln.GetDurationPropertyFilteredByDomainID(key)
	s.Equal(key.DefaultDuration(), value(domain))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain))
}

func (s *configSuite) TestGetDurationPropertyFilteredByShardID() {
	key := dynamicconfig.TestGetDurationPropertyFilteredByShardID
	shardID := 1
	value := s.cln.GetDurationPropertyFilteredByShardID(key)
	s.Equal(key.DefaultDuration(), value(shardID))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(shardID))
}

func (s *configSuite) TestGetDurationPropertyFilteredByTaskListInfo() {
	key := dynamicconfig.TestGetDurationPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetDurationPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultDuration(), value(domain, taskList, taskType))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetDurationPropertyFilteredByWorkflowType() {
	key := dynamicconfig.TestGetDurationPropertyFilteredByWorkflowTypeKey
	domain := "testDomain"
	workflowType := "testWorkflowType"
	value := s.cln.GetDurationPropertyFilteredByWorkflowType(key)
	s.Equal(key.DefaultDuration(), value(domain, workflowType))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain, workflowType))
}

func (s *configSuite) TestGetMapProperty() {
	key := dynamicconfig.TestGetMapPropertyKey
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
	key := dynamicconfig.TestGetMapPropertyKey
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
	key := dynamicconfig.TestGetListPropertyKey
	arr := []interface{}{}
	value := s.cln.GetListProperty(key)
	s.Equal(key.DefaultList(), value())
	arr = append(arr, 1)
	s.client.SetValue(key, arr)
	s.Equal(1, len(value()))
	s.Equal(1, value()[0])
}

func (s *configSuite) TestUpdateConfig() {
	key := dynamicconfig.TestGetBoolPropertyKey
	value := s.cln.GetBoolProperty(key)
	err := s.client.UpdateValue(key, false)
	s.NoError(err)
	s.Equal(false, value())
	err = s.client.UpdateValue(key, true)
	s.NoError(err)
	s.Equal(true, value())
}
