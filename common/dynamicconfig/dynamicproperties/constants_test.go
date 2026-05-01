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

package dynamicproperties

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/constants"
)

type constantSuite struct {
	suite.Suite
}

func TestConstantSuite(t *testing.T) {
	suite.Run(t, new(constantSuite))
}

func (s *constantSuite) TestListAllProductionKeys() {
	// check if we given enough capacity
	testResult := ListAllProductionKeys()
	s.GreaterOrEqual(len(IntKeys)+len(BoolKeys)+len(FloatKeys)+len(StringKeys)+len(DurationKeys)+len(MapKeys), len(testResult))
	s.Equal(TestGetIntPropertyFilteredByTaskListInfoKey+1, testResult[0])
}

func (s *constantSuite) TestGetKeyFromKeyName() {
	okKeyName := "system.transactionSizeLimit"
	okResult, err := GetKeyFromKeyName(okKeyName)
	s.NoError(err)
	s.Equal(TransactionSizeLimit, okResult)

	notOkKeyName := "system.transactionSizeLimit1"
	notOkResult, err := GetKeyFromKeyName(notOkKeyName)
	s.Error(err)
	s.Nil(notOkResult)
}

func (s *constantSuite) TestGetAllKeys() {
	testResult := GetAllKeys()
	s.Equal(len(IntKeys)+len(BoolKeys)+len(FloatKeys)+len(StringKeys)+len(DurationKeys)+len(MapKeys)+len(ListKeys), len(testResult))
	s.Equal(_keyNames["testGetIntPropertyKey"], testResult["testGetIntPropertyKey"])
	s.NotEqual(_keyNames["testGetIntPropertyKey"], testResult["testGetIntPropertyFilteredByTaskListInfoKey"])
}

type NewKey int

func (k NewKey) String() string {
	return "NewKey"
}

func (k NewKey) Description() string {
	return "NewKey is a new key"
}

func (k NewKey) DefaultValue() interface{} {
	return 0
}

func (k NewKey) Filters() []Filter {
	return nil
}

func (s *constantSuite) TestValidateKeyValuePair() {
	newKeyError := ValidateKeyValuePair(NewKey(0), 0)
	s.Error(newKeyError)
	intKeyError := ValidateKeyValuePair(TestGetIntPropertyKey, "0")
	s.Error(intKeyError)
	boolKeyError := ValidateKeyValuePair(TestGetBoolPropertyKey, 0)
	s.Error(boolKeyError)
	floatKeyError := ValidateKeyValuePair(TestGetFloat64PropertyKey, 0)
	s.Error(floatKeyError)
	stringKeyError := ValidateKeyValuePair(TestGetStringPropertyKey, 0)
	s.Error(stringKeyError)
	durationKeyError := ValidateKeyValuePair(TestGetDurationPropertyKey, 0)
	s.Error(durationKeyError)
	mapKeyError := ValidateKeyValuePair(TestGetMapPropertyKey, 0)
	s.Error(mapKeyError)
	listKeyError := ValidateKeyValuePair(TestGetListPropertyKey, 0)
	s.Error(listKeyError)
}

func (s *constantSuite) TestIntKey() {
	testIntKeys := map[string]struct {
		key                  IntKey
		expectedString       string
		expectedDefaultValue int
		expectedDescription  string
		expectedFilters      []Filter
	}{
		"TestGetIntPropertyKey": {
			key:                  TestGetIntPropertyKey,
			expectedString:       "testGetIntPropertyKey",
			expectedDescription:  "",
			expectedDefaultValue: 0,
			expectedFilters:      nil,
		},
		"TransactionSizeLimit": {
			key:                  TransactionSizeLimit,
			expectedString:       "system.transactionSizeLimit",
			expectedDescription:  "TransactionSizeLimit is the largest allowed transaction size to persistence",
			expectedDefaultValue: 14680064,
		},
		"BlobSizeLimitWarn": {
			key:                  BlobSizeLimitWarn,
			expectedString:       "limit.blobSize.warn",
			expectedDescription:  "BlobSizeLimitWarn is the per event blob size limit for warning",
			expectedDefaultValue: 256 * 1024,
			expectedFilters:      []Filter{DomainName},
		},
	}

	for _, value := range testIntKeys {
		s.Equal(value.expectedString, value.key.String())
		s.Equal(value.expectedDefaultValue, value.key.DefaultValue())
		s.Equal(value.expectedDescription, value.key.Description())
		s.Equal(value.expectedFilters, value.key.Filters())
		s.Equal(value.expectedDefaultValue, value.key.DefaultInt())
	}
}

func (s *constantSuite) TestBoolKey() {
	testBoolKeys := map[string]struct {
		key                  BoolKey
		expectedString       string
		expectedDefaultValue bool
		expectedDescription  string
		expectedFilters      []Filter
	}{
		"TestGetBoolPropertyKey": {
			key:                  TestGetBoolPropertyKey,
			expectedString:       "testGetBoolPropertyKey",
			expectedDescription:  "",
			expectedDefaultValue: false,
			expectedFilters:      nil,
		},
		"FrontendEmitSignalNameMetricsTag": {
			key:                  FrontendEmitSignalNameMetricsTag,
			expectedString:       "frontend.emitSignalNameMetricsTag",
			expectedDescription:  "FrontendEmitSignalNameMetricsTag enables emitting signal name tag in metrics in frontend client",
			expectedDefaultValue: false,
			expectedFilters:      []Filter{DomainName},
		},
	}

	for _, value := range testBoolKeys {
		s.Equal(value.expectedString, value.key.String())
		s.Equal(value.expectedDefaultValue, value.key.DefaultValue())
		s.Equal(value.expectedDescription, value.key.Description())
		s.Equal(value.expectedFilters, value.key.Filters())
		s.Equal(value.expectedDefaultValue, value.key.DefaultBool())
	}
}

func (s *constantSuite) TestFloatKey() {
	testFloatKeys := map[string]struct {
		key          FloatKey
		KeyName      string
		Filters      []Filter
		Description  string
		DefaultValue float64
	}{
		"TestGetFloat64PropertyKey": {
			key:          TestGetFloat64PropertyKey,
			KeyName:      "testGetFloat64PropertyKey",
			Description:  "",
			DefaultValue: 0,
		},
		"DomainFailoverRefreshTimerJitterCoefficient": {
			key:          DomainFailoverRefreshTimerJitterCoefficient,
			KeyName:      "frontend.domainFailoverRefreshTimerJitterCoefficient",
			Description:  "DomainFailoverRefreshTimerJitterCoefficient is the jitter for domain failover refresh timer jitter",
			DefaultValue: 0.1,
		},
		"ReplicationTaskProcessorStartWaitJitterCoefficient": {
			key:          ReplicationTaskProcessorStartWaitJitterCoefficient,
			KeyName:      "history.ReplicationTaskProcessorStartWaitJitterCoefficient",
			Filters:      []Filter{ShardID},
			Description:  "ReplicationTaskProcessorStartWaitJitterCoefficient is the jitter for batch start wait timer",
			DefaultValue: 0.9,
		},
	}

	for _, value := range testFloatKeys {
		s.Equal(value.KeyName, value.key.String())
		s.Equal(value.DefaultValue, value.key.DefaultValue())
		s.Equal(value.Description, value.key.Description())
		s.Equal(value.Filters, value.key.Filters())
		s.Equal(value.DefaultValue, value.key.DefaultFloat())
	}
}

func (s *constantSuite) TestStringKey() {
	testStringKeys := map[string]struct {
		Key          StringKey
		KeyName      string
		Filters      []Filter
		Description  string
		DefaultValue string
	}{
		"TestGetStringPropertyKey": {
			Key:          TestGetStringPropertyKey,
			KeyName:      "testGetStringPropertyKey",
			Description:  "",
			DefaultValue: "",
		},
		"HistoryArchivalStatus": {
			Key:          HistoryArchivalStatus,
			KeyName:      "system.historyArchivalStatus",
			Description:  "HistoryArchivalStatus is key for the status of history archival to override the value from static config.",
			DefaultValue: "enabled",
		},
		"DefaultEventEncoding": {
			Key:          DefaultEventEncoding,
			KeyName:      "history.defaultEventEncoding",
			Filters:      []Filter{DomainName},
			Description:  "DefaultEventEncoding is the encoding type for history events",
			DefaultValue: string(constants.EncodingTypeThriftRW),
		},
		"ReadVisibilityStoreName": {
			Key:          ReadVisibilityStoreName,
			KeyName:      "system.readVisibilityStoreName",
			Filters:      []Filter{DomainName},
			Description:  "ReadVisibilityStoreName is key to identify which store to read visibility data from",
			DefaultValue: "es",
		},
		"ShardDistributorMigrationMode": {
			Key:          ShardDistributorMigrationMode,
			KeyName:      "shardDistributor.migrationMode",
			Filters:      []Filter{Namespace},
			Description:  "ShardDistributorMigrationMode is the mode the at represent the state of the migration to rely on shard distributor for the sharding mechanism",
			DefaultValue: "onboarded",
		},
	}

	for _, value := range testStringKeys {
		s.Equal(value.KeyName, value.Key.String())
		s.Equal(value.DefaultValue, value.Key.DefaultValue())
		s.Equal(value.Description, value.Key.Description())
		s.Equal(value.Filters, value.Key.Filters())
		s.Equal(value.DefaultValue, value.Key.DefaultString())
	}
}

func (s *constantSuite) TestDurationKey() {
	testDurationKeys := map[string]struct {
		Key          DurationKey
		KeyName      string
		Filters      []Filter
		Description  string
		DefaultValue time.Duration
	}{
		"TestGetDurationPropertyKey": {
			Key:          TestGetDurationPropertyKey,
			KeyName:      "testGetDurationPropertyKey",
			Description:  "",
			DefaultValue: 0,
		},
		"FrontendFailoverCoolDown": {
			Key:          FrontendFailoverCoolDown,
			KeyName:      "frontend.failoverCoolDown",
			Filters:      []Filter{DomainName},
			Description:  "FrontendFailoverCoolDown is duration between two domain failvoers",
			DefaultValue: time.Minute,
		},
		"MatchingIdleTasklistCheckInterval": {
			Key:          MatchingIdleTasklistCheckInterval,
			KeyName:      "matching.idleTasklistCheckInterval",
			Filters:      []Filter{DomainName, TaskListName, TaskType},
			Description:  "MatchingIdleTasklistCheckInterval is the IdleTasklistCheckInterval",
			DefaultValue: time.Minute * 5,
		},
	}

	for _, value := range testDurationKeys {
		s.Equal(value.KeyName, value.Key.String())
		s.Equal(value.DefaultValue, value.Key.DefaultValue())
		s.Equal(value.Description, value.Key.Description())
		s.Equal(value.Filters, value.Key.Filters())
		s.Equal(value.DefaultValue, value.Key.DefaultDuration())
	}
}

func (s *constantSuite) TestMapKey() {
	testMapKeys := map[string]struct {
		Key          MapKey
		KeyName      string
		Filters      []Filter
		Description  string
		DefaultValue map[string]interface{}
	}{
		"TestGetMapPropertyKey": {
			Key:          TestGetMapPropertyKey,
			KeyName:      "testGetMapPropertyKey",
			Description:  "",
			DefaultValue: nil,
		},
		"TaskSchedulerRoundRobinWeights": {
			Key:         TaskSchedulerRoundRobinWeights,
			KeyName:     "history.taskSchedulerRoundRobinWeight",
			Description: "TaskSchedulerRoundRobinWeights is the priority weight for weighted round robin task scheduler",
			DefaultValue: ConvertIntMapToDynamicConfigMapProperty(map[int]int{
				constants.GetTaskPriority(constants.HighPriorityClass, constants.DefaultPrioritySubclass):    500,
				constants.GetTaskPriority(constants.DefaultPriorityClass, constants.DefaultPrioritySubclass): 20,
				constants.GetTaskPriority(constants.LowPriorityClass, constants.DefaultPrioritySubclass):     5,
			}),
		},
		"QueueProcessorStuckTaskSplitThreshold": {
			Key:          QueueProcessorStuckTaskSplitThreshold,
			KeyName:      "history.queueProcessorStuckTaskSplitThreshold",
			Description:  "QueueProcessorStuckTaskSplitThreshold is the threshold for the number of attempts of a task",
			DefaultValue: ConvertIntMapToDynamicConfigMapProperty(map[int]int{0: 100, 1: 10000}),
		},
	}

	for _, value := range testMapKeys {
		s.Equal(value.KeyName, value.Key.String())
		s.Equal(value.DefaultValue, value.Key.DefaultValue())
		s.Equal(value.Description, value.Key.Description())
		s.Equal(value.Filters, value.Key.Filters())
		s.Equal(value.DefaultValue, value.Key.DefaultMap())
	}
}

func (s *constantSuite) TestListKey() {
	testListKeys := map[string]struct {
		Key          ListKey
		KeyName      string
		Filters      []Filter
		Description  string
		DefaultValue []interface{}
	}{
		"DefaultIsolationGroupConfigStoreManagerGlobalMapping": {
			Key:     DefaultIsolationGroupConfigStoreManagerGlobalMapping,
			KeyName: "system.defaultIsolationGroupConfigStoreManagerGlobalMapping",
			Description: "A configuration store for global isolation groups - used in isolation-group config only, not normal dynamic config." +
				"Not intended for use in normal dynamic config",
		},
		"HeaderForwardingRules": {
			Key:     HeaderForwardingRules,
			KeyName: "admin.HeaderForwardingRules",
			Description: "Only loaded at startup.  " +
				"A list of rpc.HeaderRule values that define which headers to include or exclude for all requests, applied in order.  " +
				"Regexes and header names are used as-is, you are strongly encouraged to use `(?i)` to make your regex case-insensitive.",
			DefaultValue: []interface{}{
				map[string]interface{}{
					"Add":   true,
					"Match": "",
				},
			},
		},
	}

	for _, value := range testListKeys {
		s.Equal(value.KeyName, value.Key.String())
		s.Equal(value.DefaultValue, value.Key.DefaultValue())
		s.Equal(value.Description, value.Key.Description())
		s.Equal(value.Filters, value.Key.Filters())
		s.Equal(value.DefaultValue, value.Key.DefaultList())
	}
}

func TestDynamicConfigFilterTypeIsMapped(t *testing.T) {
	require.Equal(t, int(LastFilterTypeForTest), len(filters))
	for i := UnknownFilter; i < LastFilterTypeForTest; i++ {
		require.NotEmpty(t, filters[i])
	}
}

func TestDynamicConfigFilterTypeIsParseable(t *testing.T) {
	allFilters := map[Filter]int{}
	for idx, filterString := range filters { // TestDynamicConfigFilterTypeIsMapped ensures this is a complete list
		// all filter-strings must parse to unique filters
		parsed := ParseFilter(filterString)
		prev, ok := allFilters[parsed]
		assert.False(t, ok, "%q is already mapped to the same filter type as %q", filterString, filters[prev])
		allFilters[parsed] = idx

		// otherwise, only "unknown" should map to "unknown".
		// ParseFilter should probably be re-implemented to simply use a map that is shared with the definitions
		// so values cannot get out of sync, but for now this is just asserting what is currently built.
		if idx == 0 {
			assert.Equalf(t, UnknownFilter, ParseFilter(filterString), "first filter string should have parsed as unknown: %v", filterString)
			// unknown filter string is likely safe to change and then should be updated here, but otherwise this ensures the logic isn't entirely position-dependent.
			require.Equalf(t, "unknownFilter", filterString, "expected first filter to be 'unknownFilter', but it was %v", filterString)
		} else {
			assert.NotEqualf(t, UnknownFilter, ParseFilter(filterString), "failed to parse filter: %s, make sure it is in ParseFilter's switch statement", filterString)
		}
	}
}

func TestDynamicConfigFilterStringsCorrectly(t *testing.T) {
	for _, filterString := range filters {
		// filter-string-parsing and the resulting filter's String() must match
		parsed := ParseFilter(filterString)
		assert.Equal(t, filterString, parsed.String(), "filters need to String() correctly as some impls rely on it")
	}
	// should not be possible normally, but improper casting could trigger it
	badFilter := Filter(len(filters))
	assert.Equal(t, UnknownFilter.String(), badFilter.String(), "filters with indexes outside the list of known strings should String() to the unknown filter type")
}
