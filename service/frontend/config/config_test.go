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
	"github.com/uber/cadence/common/log/testlogger"
)

type configTestCase struct {
	key   dynamicconfig.Key
	value interface{}
}

var ignoreField = configTestCase{key: dynamicconfig.UnknownIntKey}

func TestNewConfig(t *testing.T) {
	fields := map[string]configTestCase{
		"NumHistoryShards":                            {nil, 1001},
		"IsAdvancedVisConfigExist":                    {nil, true},
		"HostName":                                    {nil, "hostname"},
		"DomainConfig":                                ignoreField, // Handle this separately since it's also a config object
		"PersistenceMaxQPS":                           {dynamicconfig.FrontendPersistenceMaxQPS, 1},
		"PersistenceGlobalMaxQPS":                     {dynamicconfig.FrontendPersistenceGlobalMaxQPS, 2},
		"VisibilityMaxPageSize":                       {dynamicconfig.FrontendVisibilityMaxPageSize, 3},
		"EnableVisibilitySampling":                    {dynamicconfig.EnableVisibilitySampling, true},
		"EnableReadFromClosedExecutionV2":             {dynamicconfig.EnableReadFromClosedExecutionV2, false},
		"VisibilityListMaxQPS":                        {dynamicconfig.FrontendVisibilityListMaxQPS, 4},
		"ESVisibilityListMaxQPS":                      {dynamicconfig.FrontendESVisibilityListMaxQPS, 5},
		"ReadVisibilityStoreName":                     {dynamicconfig.ReadVisibilityStoreName, "es"},
		"EnableLogCustomerQueryParameter":             {dynamicconfig.EnableLogCustomerQueryParameter, true},
		"ESIndexMaxResultWindow":                      {dynamicconfig.FrontendESIndexMaxResultWindow, 6},
		"HistoryMaxPageSize":                          {dynamicconfig.FrontendHistoryMaxPageSize, 7},
		"UserRPS":                                     {dynamicconfig.FrontendUserRPS, 8},
		"WorkerRPS":                                   {dynamicconfig.FrontendWorkerRPS, 9},
		"VisibilityRPS":                               {dynamicconfig.FrontendVisibilityRPS, 10},
		"AsyncRPS":                                    {dynamicconfig.FrontendAsyncRPS, 11},
		"MaxDomainUserRPSPerInstance":                 {dynamicconfig.FrontendMaxDomainUserRPSPerInstance, 12},
		"MaxDomainWorkerRPSPerInstance":               {dynamicconfig.FrontendMaxDomainWorkerRPSPerInstance, 13},
		"MaxDomainVisibilityRPSPerInstance":           {dynamicconfig.FrontendMaxDomainVisibilityRPSPerInstance, 14},
		"MaxDomainAsyncRPSPerInstance":                {dynamicconfig.FrontendMaxDomainAsyncRPSPerInstance, 15},
		"GlobalDomainUserRPS":                         {dynamicconfig.FrontendGlobalDomainUserRPS, 16},
		"GlobalDomainWorkerRPS":                       {dynamicconfig.FrontendGlobalDomainWorkerRPS, 17},
		"GlobalDomainVisibilityRPS":                   {dynamicconfig.FrontendGlobalDomainVisibilityRPS, 18},
		"GlobalDomainAsyncRPS":                        {dynamicconfig.FrontendGlobalDomainAsyncRPS, 19},
		"MaxIDLengthWarnLimit":                        {dynamicconfig.MaxIDLengthWarnLimit, 20},
		"DomainNameMaxLength":                         {dynamicconfig.DomainNameMaxLength, 21},
		"IdentityMaxLength":                           {dynamicconfig.IdentityMaxLength, 22},
		"WorkflowIDMaxLength":                         {dynamicconfig.WorkflowIDMaxLength, 23},
		"SignalNameMaxLength":                         {dynamicconfig.SignalNameMaxLength, 24},
		"WorkflowTypeMaxLength":                       {dynamicconfig.WorkflowTypeMaxLength, 25},
		"RequestIDMaxLength":                          {dynamicconfig.RequestIDMaxLength, 26},
		"TaskListNameMaxLength":                       {dynamicconfig.TaskListNameMaxLength, 27},
		"HistoryMgrNumConns":                          {dynamicconfig.FrontendHistoryMgrNumConns, 28},
		"EnableAdminProtection":                       {dynamicconfig.EnableAdminProtection, true},
		"AdminOperationToken":                         {dynamicconfig.AdminOperationToken, "token"},
		"DisableListVisibilityByFilter":               {dynamicconfig.DisableListVisibilityByFilter, false},
		"BlobSizeLimitError":                          {dynamicconfig.BlobSizeLimitError, 29},
		"BlobSizeLimitWarn":                           {dynamicconfig.BlobSizeLimitWarn, 30},
		"ThrottledLogRPS":                             {dynamicconfig.FrontendThrottledLogRPS, 31},
		"ShutdownDrainDuration":                       {dynamicconfig.FrontendShutdownDrainDuration, time.Duration(32)},
		"EnableDomainNotActiveAutoForwarding":         {dynamicconfig.EnableDomainNotActiveAutoForwarding, true},
		"EnableGracefulFailover":                      {dynamicconfig.EnableGracefulFailover, false},
		"DomainFailoverRefreshInterval":               {dynamicconfig.DomainFailoverRefreshInterval, time.Duration(33)},
		"DomainFailoverRefreshTimerJitterCoefficient": {dynamicconfig.DomainFailoverRefreshTimerJitterCoefficient, 34.0},
		"EnableClientVersionCheck":                    {dynamicconfig.EnableClientVersionCheck, true},
		"EnableQueryAttributeValidation":              {dynamicconfig.EnableQueryAttributeValidation, false},
		"ValidSearchAttributes":                       {dynamicconfig.ValidSearchAttributes, map[string]interface{}{"foo": "bar"}},
		"SearchAttributesNumberOfKeysLimit":           {dynamicconfig.SearchAttributesNumberOfKeysLimit, 35},
		"SearchAttributesSizeOfValueLimit":            {dynamicconfig.SearchAttributesSizeOfValueLimit, 36},
		"SearchAttributesTotalSizeLimit":              {dynamicconfig.SearchAttributesTotalSizeLimit, 37},
		"VisibilityArchivalQueryMaxPageSize":          {dynamicconfig.VisibilityArchivalQueryMaxPageSize, 38},
		"DisallowQuery":                               {dynamicconfig.DisallowQuery, true},
		"SendRawWorkflowHistory":                      {dynamicconfig.SendRawWorkflowHistory, false},
		"DecisionResultCountLimit":                    {dynamicconfig.FrontendDecisionResultCountLimit, 39},
		"EmitSignalNameMetricsTag":                    {dynamicconfig.FrontendEmitSignalNameMetricsTag, true},
		"Lockdown":                                    {dynamicconfig.Lockdown, false},
		"EnableTasklistIsolation":                     {dynamicconfig.EnableTasklistIsolation, true},
		"GlobalRatelimiterKeyMode":                    {dynamicconfig.FrontendGlobalRatelimiterMode, "disabled"},
		"GlobalRatelimiterUpdateInterval":             {dynamicconfig.GlobalRatelimiterUpdateInterval, 3 * time.Second},
	}
	domainFields := map[string]configTestCase{
		"MaxBadBinaryCount":      {dynamicconfig.FrontendMaxBadBinaries, 40},
		"MinRetentionDays":       {dynamicconfig.MinRetentionDays, 41},
		"MaxRetentionDays":       {dynamicconfig.MaxRetentionDays, 42},
		"FailoverCoolDown":       {dynamicconfig.FrontendFailoverCoolDown, time.Duration(43)},
		"RequiredDomainDataKeys": {dynamicconfig.RequiredDomainDataKeys, map[string]interface{}{"bar": "baz"}},
		"FailoverHistoryMaxSize": {dynamicconfig.FrontendFailoverHistoryMaxSize, 44},
	}
	client := dynamicconfig.NewInMemoryClient()
	dc := dynamicconfig.NewCollection(client, testlogger.New(t))

	config := NewConfig(dc, 1001, true, "hostname")

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
		case dynamicconfig.IntPropertyFn:
			return fn()
		case dynamicconfig.IntPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.BoolPropertyFn:
			return fn()
		case dynamicconfig.BoolPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.DurationPropertyFn:
			return fn()
		case dynamicconfig.DurationPropertyFnWithDomainFilter:
			return fn("domain")
		case dynamicconfig.FloatPropertyFn:
			return fn()
		case dynamicconfig.MapPropertyFn:
			return fn()
		case dynamicconfig.StringPropertyFn:
			return fn()
		case dynamicconfig.StringPropertyWithRatelimitKeyFilter:
			return fn("user:domain")
		case dynamicconfig.StringPropertyFnWithDomainFilter:
			return fn("domain")
		default:
			panic("Unable to handle type: " + f.Type().Name())
		}
	default:
		return f.Interface()
	}
}
