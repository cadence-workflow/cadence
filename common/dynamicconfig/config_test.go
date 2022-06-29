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

func (s *configSuite) SetupSuite() {
	s.client = NewInMemoryClient().(*inMemoryClient)
	logger := log.NewNoop()
	s.cln = NewCollection(s.client, logger)
}

func (s *configSuite) TestGetProperty() {
	key := TestGetStringPropertyKey
	value := s.cln.GetProperty(key)
	s.Equal(key.DefaultValue(), value())
	s.client.SetValue(key, "b")
	s.Equal("b", value())
}

func (s *configSuite) TestGetIntProperty() {
	key := TestGetIntPropertyKey
	value := s.cln.GetIntProperty(key)
	s.Equal(key.DefaultInt(), value())
	s.client.SetValue(key, 50)
	s.Equal(50, value())
}

func (s *configSuite) TestGetIntPropertyFilteredByDomain() {
	key := TestGetIntPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetIntPropertyFilteredByDomain(key)
	s.Equal(key.DefaultInt(), value(domain))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain))
}

func (s *configSuite) TestGetStringPropertyFnWithDomainFilter() {
	key := DefaultEventEncoding
	domain := "testDomain"
	value := s.cln.GetStringPropertyFilteredByDomain(key)
	s.Equal(key.DefaultString(), value(domain))
	s.client.SetValue(key, "efg")
	s.Equal("efg", value(domain))
}

func (s *configSuite) TestGetIntPropertyFilteredByTaskListInfo() {
	key := TestGetIntPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetIntPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultInt(), value(domain, taskList, taskType))
	s.client.SetValue(key, 50)
	s.Equal(50, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetFloat64Property() {
	key := TestGetFloat64PropertyKey
	value := s.cln.GetFloat64Property(key)
	s.Equal(key.DefaultFloat(), value())
	s.client.SetValue(key, 0.01)
	s.Equal(0.01, value())
}

func (s *configSuite) TestGetBoolProperty() {
	key := TestGetBoolPropertyKey
	value := s.cln.GetBoolProperty(key)
	s.Equal(key.DefaultBool(), value())
	s.client.SetValue(key, false)
	s.Equal(false, value())
}

func (s *configSuite) TestGetBoolPropertyFilteredByDomainID() {
	key := TestGetBoolPropertyFilteredByDomainIDKey
	domainID := "testDomainID"
	value := s.cln.GetBoolPropertyFilteredByDomainID(key)
	s.Equal(key.DefaultBool(), value(domainID))
	s.client.SetValue(key, false)
	s.Equal(false, value(domainID))
}

func (s *configSuite) TestGetBoolPropertyFilteredByTaskListInfo() {
	key := TestGetBoolPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetBoolPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultBool(), value(domain, taskList, taskType))
	s.client.SetValue(key, true)
	s.Equal(true, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetDurationProperty() {
	key := TestGetDurationPropertyKey
	value := s.cln.GetDurationProperty(key)
	s.Equal(key.DefaultDuration(), value())
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value())
}

func (s *configSuite) TestGetDurationPropertyFilteredByDomain() {
	key := TestGetDurationPropertyFilteredByDomainKey
	domain := "testDomain"
	value := s.cln.GetDurationPropertyFilteredByDomain(key)
	s.Equal(key.DefaultDuration(), value(domain))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain))
}

func (s *configSuite) TestGetDurationPropertyFilteredByTaskListInfo() {
	key := TestGetDurationPropertyFilteredByTaskListInfoKey
	domain := "testDomain"
	taskList := "testTaskList"
	taskType := 0
	value := s.cln.GetDurationPropertyFilteredByTaskListInfo(key)
	s.Equal(key.DefaultDuration(), value(domain, taskList, taskType))
	s.client.SetValue(key, time.Minute)
	s.Equal(time.Minute, value(domain, taskList, taskType))
}

func (s *configSuite) TestGetMapProperty() {
	key := TestGetMapPropertyKey
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

func (s *configSuite) TestUpdateConfig() {
	key := TestGetBoolPropertyKey
	value := s.cln.GetBoolProperty(key)
	err := s.client.UpdateValue(key, false)
	s.NoError(err)
	s.Equal(false, value())
	err = s.client.UpdateValue(key, true)
	s.NoError(err)
	s.Equal(true, value())
}

func TestDynamicConfigKeyIsMapped(t *testing.T) {
	for i := UnknownIntKey + 1; i < LastIntKey; i++ {
		key, ok := IntKeys[i]
		require.True(t, ok, "missing IntKey: %d", i)
		require.NotEmpty(t, key, "empty IntKey: %d", i)
	}
	for i := UnknownBoolKey + 1; i < LastBoolKey; i++ {
		key, ok := BoolKeys[i]
		require.True(t, ok, "missing BoolKey: %d", i)
		require.NotEmpty(t, key, "empty BoolKey: %d", i)
	}
	for i := UnknownFloatKey + 1; i < LastFloatKey; i++ {
		key, ok := FloatKeys[i]
		require.True(t, ok, "missing FloatKey: %d", i)
		require.NotEmpty(t, key, "empty FloatKey: %d", i)
	}
	for i := UnknownStringKey + 1; i < LastStringKey; i++ {
		key, ok := StringKeys[i]
		require.True(t, ok, "missing StringKey: %d", i)
		require.NotEmpty(t, key, "empty StringKey: %d", i)
	}
	for i := UnknownDurationKey + 1; i < LastDurationKey; i++ {
		key, ok := DurationKeys[i]
		require.True(t, ok, "missing DurationKey: %d", i)
		require.NotEmpty(t, key, "empty DurationKey: %d", i)
	}
	for i := UnknownMapKey + 1; i < LastMapKey; i++ {
		key, ok := MapKeys[i]
		require.True(t, ok, "missing MapKey: %d", i)
		require.NotEmpty(t, key, "empty MapKey: %d", i)
	}
}

func TestDynamicConfigFilterTypeIsMapped(t *testing.T) {
	require.Equal(t, int(LastFilterTypeForTest), len(filters))
	for i := UnknownFilter; i < LastFilterTypeForTest; i++ {
		require.NotEmpty(t, filters[i])
	}
}

func BenchmarkLogValue(b *testing.B) {
	keys := []Key{
		HistorySizeLimitError,
		MatchingThrottledLogRPS,
		MatchingIdleTasklistCheckInterval,
	}
	values := []interface{}{
		1024 * 1024,
		0.1,
		30 * time.Second,
	}
	filters := map[Filter]interface{}{
		ClusterName: "development",
		DomainName:  "domainName",
	}
	cmpFuncs := []func(interface{}, interface{}) bool{
		intCompareEquals,
		float64CompareEquals,
		durationCompareEquals,
	}

	collection := NewCollection(NewInMemoryClient(), log.NewNoop())
	// pre-warm the collection logValue map
	for i := range keys {
		collection.logValue(keys[i], filters, values[i], values[i], cmpFuncs[i])
	}

	for i := 0; i < b.N; i++ {
		for i := range keys {
			collection.logValue(keys[i], filters, values[i], values[i], cmpFuncs[i])
		}
	}
}
