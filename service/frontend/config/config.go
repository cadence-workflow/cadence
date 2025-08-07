// Copyright (c) 2019 Uber Technologies, Inc.
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

package config

import (
	"github.com/uber/cadence/common/domain"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
)

// Config represents configuration for cadence-frontend service
type Config struct {
	NumHistoryShards                int
	IsAdvancedVisConfigExist        bool
	DomainConfig                    domain.Config
	PersistenceMaxQPS               dynamicproperties.IntPropertyFn
	PersistenceGlobalMaxQPS         dynamicproperties.IntPropertyFn
	VisibilityMaxPageSize           dynamicproperties.IntPropertyFnWithDomainFilter
	EnableVisibilitySampling        dynamicproperties.BoolPropertyFn
	EnableReadFromClosedExecutionV2 dynamicproperties.BoolPropertyFn
	// deprecated: never used for ratelimiting, only sampling-based failure injection, and only on database-based visibility
	VisibilityListMaxQPS            dynamicproperties.IntPropertyFnWithDomainFilter
	EnableLogCustomerQueryParameter dynamicproperties.BoolPropertyFnWithDomainFilter
	ReadVisibilityStoreName         dynamicproperties.StringPropertyFnWithDomainFilter
	// deprecated: never read from
	ESVisibilityListMaxQPS            dynamicproperties.IntPropertyFnWithDomainFilter
	ESIndexMaxResultWindow            dynamicproperties.IntPropertyFn
	HistoryMaxPageSize                dynamicproperties.IntPropertyFnWithDomainFilter
	UserRPS                           dynamicproperties.IntPropertyFn
	WorkerRPS                         dynamicproperties.IntPropertyFn
	VisibilityRPS                     dynamicproperties.IntPropertyFn
	AsyncRPS                          dynamicproperties.IntPropertyFn
	MaxDomainUserRPSPerInstance       dynamicproperties.IntPropertyFnWithDomainFilter
	MaxDomainWorkerRPSPerInstance     dynamicproperties.IntPropertyFnWithDomainFilter
	MaxDomainVisibilityRPSPerInstance dynamicproperties.IntPropertyFnWithDomainFilter
	MaxDomainAsyncRPSPerInstance      dynamicproperties.IntPropertyFnWithDomainFilter
	GlobalDomainUserRPS               dynamicproperties.IntPropertyFnWithDomainFilter
	GlobalDomainWorkerRPS             dynamicproperties.IntPropertyFnWithDomainFilter
	GlobalDomainVisibilityRPS         dynamicproperties.IntPropertyFnWithDomainFilter
	GlobalDomainAsyncRPS              dynamicproperties.IntPropertyFnWithDomainFilter
	EnableClientVersionCheck          dynamicproperties.BoolPropertyFn
	EnableQueryAttributeValidation    dynamicproperties.BoolPropertyFn
	DisallowQuery                     dynamicproperties.BoolPropertyFnWithDomainFilter
	ShutdownDrainDuration             dynamicproperties.DurationPropertyFn
	WarmupDuration                    dynamicproperties.DurationPropertyFn
	Lockdown                          dynamicproperties.BoolPropertyFnWithDomainFilter

	// global ratelimiter config, uses GlobalDomain*RPS for RPS configuration
	GlobalRatelimiterKeyMode        dynamicproperties.StringPropertyWithRatelimitKeyFilter
	GlobalRatelimiterUpdateInterval dynamicproperties.DurationPropertyFn

	// isolation configuration
	EnableTasklistIsolation dynamicproperties.BoolPropertyFnWithDomainFilter

	// id length limits
	MaxIDLengthWarnLimit  dynamicproperties.IntPropertyFn
	DomainNameMaxLength   dynamicproperties.IntPropertyFnWithDomainFilter
	IdentityMaxLength     dynamicproperties.IntPropertyFnWithDomainFilter
	WorkflowIDMaxLength   dynamicproperties.IntPropertyFnWithDomainFilter
	SignalNameMaxLength   dynamicproperties.IntPropertyFnWithDomainFilter
	WorkflowTypeMaxLength dynamicproperties.IntPropertyFnWithDomainFilter
	RequestIDMaxLength    dynamicproperties.IntPropertyFnWithDomainFilter
	TaskListNameMaxLength dynamicproperties.IntPropertyFnWithDomainFilter

	// security protection settings
	EnableAdminProtection         dynamicproperties.BoolPropertyFn
	AdminOperationToken           dynamicproperties.StringPropertyFn
	DisableListVisibilityByFilter dynamicproperties.BoolPropertyFnWithDomainFilter

	// size limit system protection
	BlobSizeLimitError dynamicproperties.IntPropertyFnWithDomainFilter
	BlobSizeLimitWarn  dynamicproperties.IntPropertyFnWithDomainFilter

	ThrottledLogRPS dynamicproperties.IntPropertyFn

	// Domain specific config
	EnableDomainNotActiveAutoForwarding         dynamicproperties.BoolPropertyFnWithDomainFilter
	EnableGracefulFailover                      dynamicproperties.BoolPropertyFn
	DomainFailoverRefreshInterval               dynamicproperties.DurationPropertyFn
	DomainFailoverRefreshTimerJitterCoefficient dynamicproperties.FloatPropertyFn

	// ValidSearchAttributes is legal indexed keys that can be used in list APIs
	ValidSearchAttributes             dynamicproperties.MapPropertyFn
	SearchAttributesNumberOfKeysLimit dynamicproperties.IntPropertyFnWithDomainFilter
	SearchAttributesSizeOfValueLimit  dynamicproperties.IntPropertyFnWithDomainFilter
	SearchAttributesTotalSizeLimit    dynamicproperties.IntPropertyFnWithDomainFilter
	PinotOptimizedQueryColumns        dynamicproperties.MapPropertyFn
	// VisibilityArchival system protection
	VisibilityArchivalQueryMaxPageSize dynamicproperties.IntPropertyFn

	SendRawWorkflowHistory dynamicproperties.BoolPropertyFnWithDomainFilter

	// max number of decisions per RespondDecisionTaskCompleted request (unlimited by default)
	DecisionResultCountLimit dynamicproperties.IntPropertyFnWithDomainFilter

	// Debugging

	// Emit signal related metrics with signal name tag. Be aware of cardinality.
	EmitSignalNameMetricsTag dynamicproperties.BoolPropertyFnWithDomainFilter

	// HostName for machine running the service
	HostName string
}

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection, numHistoryShards int, isAdvancedVisConfigExist bool, hostName string, logger log.Logger) *Config {
	logger.Debugf("Creating new frontend config for host %s, numHistoryShards: %d, isAdvancedVisConfigExist: %t", hostName, numHistoryShards, isAdvancedVisConfigExist)
	return &Config{
		NumHistoryShards:                            numHistoryShards,
		IsAdvancedVisConfigExist:                    isAdvancedVisConfigExist,
		PersistenceMaxQPS:                           dc.GetIntProperty(dynamicproperties.FrontendPersistenceMaxQPS),
		PersistenceGlobalMaxQPS:                     dc.GetIntProperty(dynamicproperties.FrontendPersistenceGlobalMaxQPS),
		VisibilityMaxPageSize:                       dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendVisibilityMaxPageSize),
		EnableVisibilitySampling:                    dc.GetBoolProperty(dynamicproperties.EnableVisibilitySampling),
		EnableReadFromClosedExecutionV2:             dc.GetBoolProperty(dynamicproperties.EnableReadFromClosedExecutionV2),
		VisibilityListMaxQPS:                        dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendVisibilityListMaxQPS),
		ESVisibilityListMaxQPS:                      dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendESVisibilityListMaxQPS),
		ReadVisibilityStoreName:                     dc.GetStringPropertyFilteredByDomain(dynamicproperties.ReadVisibilityStoreName),
		EnableLogCustomerQueryParameter:             dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableLogCustomerQueryParameter),
		ESIndexMaxResultWindow:                      dc.GetIntProperty(dynamicproperties.FrontendESIndexMaxResultWindow),
		HistoryMaxPageSize:                          dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendHistoryMaxPageSize),
		UserRPS:                                     dc.GetIntProperty(dynamicproperties.FrontendUserRPS),
		WorkerRPS:                                   dc.GetIntProperty(dynamicproperties.FrontendWorkerRPS),
		VisibilityRPS:                               dc.GetIntProperty(dynamicproperties.FrontendVisibilityRPS),
		AsyncRPS:                                    dc.GetIntProperty(dynamicproperties.FrontendAsyncRPS),
		MaxDomainUserRPSPerInstance:                 dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendMaxDomainUserRPSPerInstance),
		MaxDomainWorkerRPSPerInstance:               dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendMaxDomainWorkerRPSPerInstance),
		MaxDomainVisibilityRPSPerInstance:           dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendMaxDomainVisibilityRPSPerInstance),
		MaxDomainAsyncRPSPerInstance:                dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendMaxDomainAsyncRPSPerInstance),
		GlobalDomainUserRPS:                         dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendGlobalDomainUserRPS),
		GlobalDomainWorkerRPS:                       dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendGlobalDomainWorkerRPS),
		GlobalDomainVisibilityRPS:                   dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendGlobalDomainVisibilityRPS),
		GlobalDomainAsyncRPS:                        dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendGlobalDomainAsyncRPS),
		GlobalRatelimiterKeyMode:                    dc.GetStringPropertyFilteredByRatelimitKey(dynamicproperties.FrontendGlobalRatelimiterMode),
		GlobalRatelimiterUpdateInterval:             dc.GetDurationProperty(dynamicproperties.GlobalRatelimiterUpdateInterval),
		MaxIDLengthWarnLimit:                        dc.GetIntProperty(dynamicproperties.MaxIDLengthWarnLimit),
		DomainNameMaxLength:                         dc.GetIntPropertyFilteredByDomain(dynamicproperties.DomainNameMaxLength),
		IdentityMaxLength:                           dc.GetIntPropertyFilteredByDomain(dynamicproperties.IdentityMaxLength),
		WorkflowIDMaxLength:                         dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowIDMaxLength),
		SignalNameMaxLength:                         dc.GetIntPropertyFilteredByDomain(dynamicproperties.SignalNameMaxLength),
		WorkflowTypeMaxLength:                       dc.GetIntPropertyFilteredByDomain(dynamicproperties.WorkflowTypeMaxLength),
		RequestIDMaxLength:                          dc.GetIntPropertyFilteredByDomain(dynamicproperties.RequestIDMaxLength),
		TaskListNameMaxLength:                       dc.GetIntPropertyFilteredByDomain(dynamicproperties.TaskListNameMaxLength),
		EnableAdminProtection:                       dc.GetBoolProperty(dynamicproperties.EnableAdminProtection),
		AdminOperationToken:                         dc.GetStringProperty(dynamicproperties.AdminOperationToken),
		DisableListVisibilityByFilter:               dc.GetBoolPropertyFilteredByDomain(dynamicproperties.DisableListVisibilityByFilter),
		BlobSizeLimitError:                          dc.GetIntPropertyFilteredByDomain(dynamicproperties.BlobSizeLimitError),
		BlobSizeLimitWarn:                           dc.GetIntPropertyFilteredByDomain(dynamicproperties.BlobSizeLimitWarn),
		ThrottledLogRPS:                             dc.GetIntProperty(dynamicproperties.FrontendThrottledLogRPS),
		ShutdownDrainDuration:                       dc.GetDurationProperty(dynamicproperties.FrontendShutdownDrainDuration),
		WarmupDuration:                              dc.GetDurationProperty(dynamicproperties.FrontendWarmupDuration),
		EnableDomainNotActiveAutoForwarding:         dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableDomainNotActiveAutoForwarding),
		EnableGracefulFailover:                      dc.GetBoolProperty(dynamicproperties.EnableGracefulFailover),
		DomainFailoverRefreshInterval:               dc.GetDurationProperty(dynamicproperties.DomainFailoverRefreshInterval),
		DomainFailoverRefreshTimerJitterCoefficient: dc.GetFloat64Property(dynamicproperties.DomainFailoverRefreshTimerJitterCoefficient),
		EnableClientVersionCheck:                    dc.GetBoolProperty(dynamicproperties.EnableClientVersionCheck),
		EnableQueryAttributeValidation:              dc.GetBoolProperty(dynamicproperties.EnableQueryAttributeValidation),
		ValidSearchAttributes:                       dc.GetMapProperty(dynamicproperties.ValidSearchAttributes),
		SearchAttributesNumberOfKeysLimit:           dc.GetIntPropertyFilteredByDomain(dynamicproperties.SearchAttributesNumberOfKeysLimit),
		SearchAttributesSizeOfValueLimit:            dc.GetIntPropertyFilteredByDomain(dynamicproperties.SearchAttributesSizeOfValueLimit),
		SearchAttributesTotalSizeLimit:              dc.GetIntPropertyFilteredByDomain(dynamicproperties.SearchAttributesTotalSizeLimit),
		PinotOptimizedQueryColumns:                  dc.GetMapProperty(dynamicproperties.PinotOptimizedQueryColumns),
		VisibilityArchivalQueryMaxPageSize:          dc.GetIntProperty(dynamicproperties.VisibilityArchivalQueryMaxPageSize),
		DisallowQuery:                               dc.GetBoolPropertyFilteredByDomain(dynamicproperties.DisallowQuery),
		SendRawWorkflowHistory:                      dc.GetBoolPropertyFilteredByDomain(dynamicproperties.SendRawWorkflowHistory),
		DecisionResultCountLimit:                    dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendDecisionResultCountLimit),
		EmitSignalNameMetricsTag:                    dc.GetBoolPropertyFilteredByDomain(dynamicproperties.FrontendEmitSignalNameMetricsTag),
		Lockdown:                                    dc.GetBoolPropertyFilteredByDomain(dynamicproperties.Lockdown),
		EnableTasklistIsolation:                     dc.GetBoolPropertyFilteredByDomain(dynamicproperties.EnableTasklistIsolation),
		DomainConfig: domain.Config{
			MaxBadBinaryCount:      dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendMaxBadBinaries),
			MinRetentionDays:       dc.GetIntProperty(dynamicproperties.MinRetentionDays),
			MaxRetentionDays:       dc.GetIntProperty(dynamicproperties.MaxRetentionDays),
			FailoverCoolDown:       dc.GetDurationPropertyFilteredByDomain(dynamicproperties.FrontendFailoverCoolDown),
			RequiredDomainDataKeys: dc.GetMapProperty(dynamicproperties.RequiredDomainDataKeys),
			FailoverHistoryMaxSize: dc.GetIntPropertyFilteredByDomain(dynamicproperties.FrontendFailoverHistoryMaxSize),
		},
		HostName: hostName,
	}
}
