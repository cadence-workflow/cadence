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

package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
)

type configTestCase struct {
	key   dynamicproperties.Key
	value interface{}
}

var ignoreField = configTestCase{key: dynamicproperties.UnknownIntKey}

func TestNewConfig(t *testing.T) {
	fields := map[string]configTestCase{
		"NumHistoryShards":                            {nil, 1001},
		"IsAdvancedVisConfigExist":                    {nil, true},
		"HostName":                                    {nil, "hostname"},
		"DomainConfig":                                ignoreField, // Handle this separately since it's also a config object
		"PersistenceMaxQPS":                           {dynamicproperties.FrontendPersistenceMaxQPS, 1},
		"PersistenceGlobalMaxQPS":                     {dynamicproperties.FrontendPersistenceGlobalMaxQPS, 2},
		"VisibilityMaxPageSize":                       {dynamicproperties.FrontendVisibilityMaxPageSize, 3},
		"EnableVisibilitySampling":                    {dynamicproperties.EnableVisibilitySampling, true},
		"EnableReadFromClosedExecutionV2":             {dynamicproperties.EnableReadFromClosedExecutionV2, false},
		"VisibilityListMaxQPS":                        {dynamicproperties.FrontendVisibilityListMaxQPS, 4},
		"ESVisibilityListMaxQPS":                      {dynamicproperties.FrontendESVisibilityListMaxQPS, 5},
		"ReadVisibilityStoreName":                     {dynamicproperties.ReadVisibilityStoreName, "es"},
		"EnableLogCustomerQueryParameter":             {dynamicproperties.EnableLogCustomerQueryParameter, true},
		"ESIndexMaxResultWindow":                      {dynamicproperties.FrontendESIndexMaxResultWindow, 6},
		"HistoryMaxPageSize":                          {dynamicproperties.FrontendHistoryMaxPageSize, 7},
		"UserRPS":                                     {dynamicproperties.FrontendUserRPS, 8},
		"WorkerRPS":                                   {dynamicproperties.FrontendWorkerRPS, 9},
		"VisibilityRPS":                               {dynamicproperties.FrontendVisibilityRPS, 10},
		"AsyncRPS":                                    {dynamicproperties.FrontendAsyncRPS, 11},
		"MaxDomainUserRPSPerInstance":                 {dynamicproperties.FrontendMaxDomainUserRPSPerInstance, 12},
		"MaxDomainWorkerRPSPerInstance":               {dynamicproperties.FrontendMaxDomainWorkerRPSPerInstance, 13},
		"MaxDomainVisibilityRPSPerInstance":           {dynamicproperties.FrontendMaxDomainVisibilityRPSPerInstance, 14},
		"MaxDomainAsyncRPSPerInstance":                {dynamicproperties.FrontendMaxDomainAsyncRPSPerInstance, 15},
		"GlobalDomainUserRPS":                         {dynamicproperties.FrontendGlobalDomainUserRPS, 16},
		"GlobalDomainWorkerRPS":                       {dynamicproperties.FrontendGlobalDomainWorkerRPS, 17},
		"GlobalDomainVisibilityRPS":                   {dynamicproperties.FrontendGlobalDomainVisibilityRPS, 18},
		"GlobalDomainAsyncRPS":                        {dynamicproperties.FrontendGlobalDomainAsyncRPS, 19},
		"MaxIDLengthWarnLimit":                        {dynamicproperties.MaxIDLengthWarnLimit, 20},
		"DomainNameMaxLength":                         {dynamicproperties.DomainNameMaxLength, 21},
		"IdentityMaxLength":                           {dynamicproperties.IdentityMaxLength, 22},
		"WorkflowIDMaxLength":                         {dynamicproperties.WorkflowIDMaxLength, 23},
		"SignalNameMaxLength":                         {dynamicproperties.SignalNameMaxLength, 24},
		"WorkflowTypeMaxLength":                       {dynamicproperties.WorkflowTypeMaxLength, 25},
		"RequestIDMaxLength":                          {dynamicproperties.RequestIDMaxLength, 26},
		"TaskListNameMaxLength":                       {dynamicproperties.TaskListNameMaxLength, 27},
		"HistoryMgrNumConns":                          {dynamicproperties.FrontendHistoryMgrNumConns, 28},
		"EnableAdminProtection":                       {dynamicproperties.EnableAdminProtection, true},
		"AdminOperationToken":                         {dynamicproperties.AdminOperationToken, "token"},
		"DisableListVisibilityByFilter":               {dynamicproperties.DisableListVisibilityByFilter, false},
		"BlobSizeLimitError":                          {dynamicproperties.BlobSizeLimitError, 29},
		"BlobSizeLimitWarn":                           {dynamicproperties.BlobSizeLimitWarn, 30},
		"ThrottledLogRPS":                             {dynamicproperties.FrontendThrottledLogRPS, 31},
		"ShutdownDrainDuration":                       {dynamicproperties.FrontendShutdownDrainDuration, time.Duration(32)},
		"WarmupDuration":                              {dynamicproperties.FrontendWarmupDuration, time.Duration(40)},
		"EnableDomainNotActiveAutoForwarding":         {dynamicproperties.EnableDomainNotActiveAutoForwarding, true},
		"EnableGracefulFailover":                      {dynamicproperties.EnableGracefulFailover, false},
		"DomainFailoverRefreshInterval":               {dynamicproperties.DomainFailoverRefreshInterval, time.Duration(33)},
		"DomainFailoverRefreshTimerJitterCoefficient": {dynamicproperties.DomainFailoverRefreshTimerJitterCoefficient, 34.0},
		"EnableClientVersionCheck":                    {dynamicproperties.EnableClientVersionCheck, true},
		"EnableQueryAttributeValidation":              {dynamicproperties.EnableQueryAttributeValidation, false},
		"ValidSearchAttributes":                       {dynamicproperties.ValidSearchAttributes, map[string]interface{}{"foo": "bar"}},
		"SearchAttributesNumberOfKeysLimit":           {dynamicproperties.SearchAttributesNumberOfKeysLimit, 35},
		"SearchAttributesSizeOfValueLimit":            {dynamicproperties.SearchAttributesSizeOfValueLimit, 36},
		"SearchAttributesTotalSizeLimit":              {dynamicproperties.SearchAttributesTotalSizeLimit, 37},
		"VisibilityArchivalQueryMaxPageSize":          {dynamicproperties.VisibilityArchivalQueryMaxPageSize, 38},
		"DisallowQuery":                               {dynamicproperties.DisallowQuery, true},
		"SendRawWorkflowHistory":                      {dynamicproperties.SendRawWorkflowHistory, false},
		"DecisionResultCountLimit":                    {dynamicproperties.FrontendDecisionResultCountLimit, 39},
		"EmitSignalNameMetricsTag":                    {dynamicproperties.FrontendEmitSignalNameMetricsTag, true},
		"Lockdown":                                    {dynamicproperties.Lockdown, false},
		"EnableTasklistIsolation":                     {dynamicproperties.EnableTasklistIsolation, true},
		"GlobalRatelimiterKeyMode":                    {dynamicproperties.FrontendGlobalRatelimiterMode, "disabled"},
		"GlobalRatelimiterUpdateInterval":             {dynamicproperties.GlobalRatelimiterUpdateInterval, 3 * time.Second},
		"PinotOptimizedQueryColumns":                  {dynamicproperties.PinotOptimizedQueryColumns, map[string]interface{}{"foo": "bar"}},
	}
	domainFields := map[string]configTestCase{
		"MaxBadBinaryCount":      {dynamicproperties.FrontendMaxBadBinaries, 40},
		"MinRetentionDays":       {dynamicproperties.MinRetentionDays, 41},
		"MaxRetentionDays":       {dynamicproperties.MaxRetentionDays, 42},
		"FailoverCoolDown":       {dynamicproperties.FrontendFailoverCoolDown, time.Duration(43)},
		"RequiredDomainDataKeys": {dynamicproperties.RequiredDomainDataKeys, map[string]interface{}{"bar": "baz"}},
		"FailoverHistoryMaxSize": {dynamicproperties.FrontendFailoverHistoryMaxSize, 44},
	}
	client := dynamicconfig.NewInMemoryClient()
	logger := testlogger.New(t)
	dc := dynamicconfig.NewCollection(client, logger)

	config := NewConfig(dc, 1001, true, "hostname", logger)

	assertFieldsMatch(t, *config, client, fields)
	assertFieldsMatch(t, config.DomainConfig, client, domainFields)
}

func assertFieldsMatch(t *testing.T, config interface{}, client dynamicconfig.Client, fields map[string]configTestCase) {
	configType := reflect.ValueOf(config)

	for i := 0; i < configType.NumField(); i++ {
		f := configType.Field(i)
		fieldName := configType.Type().Field(i).Name

		if expected, ok := fields[fieldName]; ok {
			if expected.key == ignoreField.key {
				continue
			}
			if expected.key != nil {
				err := client.UpdateValue(expected.key, expected.value)
				if err != nil {
					t.Errorf("Failed to update config for %s: %s", fieldName, err)
					return
				}
			}
			actual := getValue(&f)
			assert.Equal(t, expected.value, actual)

		} else {
			t.Errorf("Unknown property on Config: %s", fieldName)
		}
	}
}

func getValue(f *reflect.Value) interface{} {
	switch f.Kind() {
	case reflect.Func:
		switch fn := f.Interface().(type) {
		case dynamicproperties.IntPropertyFn:
			return fn()
		case dynamicproperties.IntPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.BoolPropertyFn:
			return fn()
		case dynamicproperties.BoolPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.DurationPropertyFn:
			return fn()
		case dynamicproperties.DurationPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicproperties.FloatPropertyFn:
			return fn()
		case dynamicproperties.MapPropertyFn:
			return fn()
		case dynamicproperties.StringPropertyFn:
			return fn()
		case dynamicproperties.StringPropertyWithRatelimitKeyFilter:
			return fn("user:domain")
		case dynamicproperties.StringPropertyFnWithDomainFilter:
			return fn("domain")
		default:
			panic("Unable to handle type: " + f.Type().Name())
		}
	default:
		return f.Interface()
	}
}
