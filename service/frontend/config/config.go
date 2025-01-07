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
)

// Config represents configuration for cadence-frontend service
type Config struct {
	NumHistoryShards                int
	IsAdvancedVisConfigExist        bool
	DomainConfig                    domain.Config
	PersistenceMaxQPS               dynamicconfig.IntPropertyFn
	PersistenceGlobalMaxQPS         dynamicconfig.IntPropertyFn
	VisibilityMaxPageSize           dynamicconfig.IntPropertyFnWithDomainFilter
	EnableVisibilitySampling        dynamicconfig.BoolPropertyFn
	EnableReadFromClosedExecutionV2 dynamicconfig.BoolPropertyFn
	// deprecated: never used for ratelimiting, only sampling-based failure injection, and only on database-based visibility
	VisibilityListMaxQPS            dynamicconfig.IntPropertyFnWithDomainFilter
	EnableReadVisibilityFromES      dynamicconfig.BoolPropertyFnWithDomainFilter
	EnableReadVisibilityFromPinot   dynamicconfig.BoolPropertyFnWithDomainFilter
	EnableVisibilityDoubleRead      dynamicconfig.BoolPropertyFnWithDomainFilter
	EnableLogCustomerQueryParameter dynamicconfig.BoolPropertyFnWithDomainFilter
	// deprecated: never read from
	ESVisibilityListMaxQPS            dynamicconfig.IntPropertyFnWithDomainFilter
	ESIndexMaxResultWindow            dynamicconfig.IntPropertyFn
	HistoryMaxPageSize                dynamicconfig.IntPropertyFnWithDomainFilter
	UserRPS                           dynamicconfig.IntPropertyFn
	WorkerRPS                         dynamicconfig.IntPropertyFn
	VisibilityRPS                     dynamicconfig.IntPropertyFn
	AsyncRPS                          dynamicconfig.IntPropertyFn
	MaxDomainUserRPSPerInstance       dynamicconfig.IntPropertyFnWithDomainFilter
	MaxDomainWorkerRPSPerInstance     dynamicconfig.IntPropertyFnWithDomainFilter
	MaxDomainVisibilityRPSPerInstance dynamicconfig.IntPropertyFnWithDomainFilter
	MaxDomainAsyncRPSPerInstance      dynamicconfig.IntPropertyFnWithDomainFilter
	GlobalDomainUserRPS               dynamicconfig.IntPropertyFnWithDomainFilter
	GlobalDomainWorkerRPS             dynamicconfig.IntPropertyFnWithDomainFilter
	GlobalDomainVisibilityRPS         dynamicconfig.IntPropertyFnWithDomainFilter
	GlobalDomainAsyncRPS              dynamicconfig.IntPropertyFnWithDomainFilter
	EnableClientVersionCheck          dynamicconfig.BoolPropertyFn
	EnableQueryAttributeValidation    dynamicconfig.BoolPropertyFn
	DisallowQuery                     dynamicconfig.BoolPropertyFnWithDomainFilter
	ShutdownDrainDuration             dynamicconfig.DurationPropertyFn
	Lockdown                          dynamicconfig.BoolPropertyFnWithDomainFilter

	// global ratelimiter config, uses GlobalDomain*RPS for RPS configuration
	GlobalRatelimiterKeyMode        dynamicconfig.StringPropertyWithRatelimitKeyFilter
	GlobalRatelimiterUpdateInterval dynamicconfig.DurationPropertyFn

	// isolation configuration
	EnableTasklistIsolation dynamicconfig.BoolPropertyFnWithDomainFilter

	// id length limits
	MaxIDLengthWarnLimit  dynamicconfig.IntPropertyFn
	DomainNameMaxLength   dynamicconfig.IntPropertyFnWithDomainFilter
	IdentityMaxLength     dynamicconfig.IntPropertyFnWithDomainFilter
	WorkflowIDMaxLength   dynamicconfig.IntPropertyFnWithDomainFilter
	SignalNameMaxLength   dynamicconfig.IntPropertyFnWithDomainFilter
	WorkflowTypeMaxLength dynamicconfig.IntPropertyFnWithDomainFilter
	RequestIDMaxLength    dynamicconfig.IntPropertyFnWithDomainFilter
	TaskListNameMaxLength dynamicconfig.IntPropertyFnWithDomainFilter

	// security protection settings
	EnableAdminProtection         dynamicconfig.BoolPropertyFn
	AdminOperationToken           dynamicconfig.StringPropertyFn
	DisableListVisibilityByFilter dynamicconfig.BoolPropertyFnWithDomainFilter

	// size limit system protection
	BlobSizeLimitError dynamicconfig.IntPropertyFnWithDomainFilter
	BlobSizeLimitWarn  dynamicconfig.IntPropertyFnWithDomainFilter

	ThrottledLogRPS dynamicconfig.IntPropertyFn

	// Domain specific config
	EnableDomainNotActiveAutoForwarding         dynamicconfig.BoolPropertyFnWithDomainFilter
	EnableGracefulFailover                      dynamicconfig.BoolPropertyFn
	DomainFailoverRefreshInterval               dynamicconfig.DurationPropertyFn
	DomainFailoverRefreshTimerJitterCoefficient dynamicconfig.FloatPropertyFn

	// ValidSearchAttributes is legal indexed keys that can be used in list APIs
	ValidSearchAttributes             dynamicconfig.MapPropertyFn
	SearchAttributesNumberOfKeysLimit dynamicconfig.IntPropertyFnWithDomainFilter
	SearchAttributesSizeOfValueLimit  dynamicconfig.IntPropertyFnWithDomainFilter
	SearchAttributesTotalSizeLimit    dynamicconfig.IntPropertyFnWithDomainFilter

	// VisibilityArchival system protection
	VisibilityArchivalQueryMaxPageSize dynamicconfig.IntPropertyFn

	SendRawWorkflowHistory dynamicconfig.BoolPropertyFnWithDomainFilter

	// max number of decisions per RespondDecisionTaskCompleted request (unlimited by default)
	DecisionResultCountLimit dynamicconfig.IntPropertyFnWithDomainFilter

	// Debugging

	// Emit signal related metrics with signal name tag. Be aware of cardinality.
	EmitSignalNameMetricsTag dynamicconfig.BoolPropertyFnWithDomainFilter

	// HostName for machine running the service
	HostName string
}

// NewConfig returns new service config with default values
func NewConfig(dc *dynamicconfig.Collection, numHistoryShards int, isAdvancedVisConfigExist bool, hostName string) *Config {
	return &Config{
		NumHistoryShards:                            numHistoryShards,
		IsAdvancedVisConfigExist:                    isAdvancedVisConfigExist,
		PersistenceMaxQPS:                           dc.GetIntProperty(dynamicconfig.FrontendPersistenceMaxQPS),
		PersistenceGlobalMaxQPS:                     dc.GetIntProperty(dynamicconfig.FrontendPersistenceGlobalMaxQPS),
		VisibilityMaxPageSize:                       dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendVisibilityMaxPageSize),
		EnableVisibilitySampling:                    dc.GetBoolProperty(dynamicconfig.EnableVisibilitySampling),
		EnableReadFromClosedExecutionV2:             dc.GetBoolProperty(dynamicconfig.EnableReadFromClosedExecutionV2),
		VisibilityListMaxQPS:                        dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendVisibilityListMaxQPS),
		ESVisibilityListMaxQPS:                      dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendESVisibilityListMaxQPS),
		EnableReadVisibilityFromES:                  dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableReadVisibilityFromES),
		EnableReadVisibilityFromPinot:               dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableReadVisibilityFromPinot),
		EnableLogCustomerQueryParameter:             dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableLogCustomerQueryParameter),
		EnableVisibilityDoubleRead:                  dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableVisibilityDoubleRead),
		ESIndexMaxResultWindow:                      dc.GetIntProperty(dynamicconfig.FrontendESIndexMaxResultWindow),
		HistoryMaxPageSize:                          dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendHistoryMaxPageSize),
		UserRPS:                                     dc.GetIntProperty(dynamicconfig.FrontendUserRPS),
		WorkerRPS:                                   dc.GetIntProperty(dynamicconfig.FrontendWorkerRPS),
		VisibilityRPS:                               dc.GetIntProperty(dynamicconfig.FrontendVisibilityRPS),
		AsyncRPS:                                    dc.GetIntProperty(dynamicconfig.FrontendAsyncRPS),
		MaxDomainUserRPSPerInstance:                 dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendMaxDomainUserRPSPerInstance),
		MaxDomainWorkerRPSPerInstance:               dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendMaxDomainWorkerRPSPerInstance),
		MaxDomainVisibilityRPSPerInstance:           dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendMaxDomainVisibilityRPSPerInstance),
		MaxDomainAsyncRPSPerInstance:                dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendMaxDomainAsyncRPSPerInstance),
		GlobalDomainUserRPS:                         dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendGlobalDomainUserRPS),
		GlobalDomainWorkerRPS:                       dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendGlobalDomainWorkerRPS),
		GlobalDomainVisibilityRPS:                   dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendGlobalDomainVisibilityRPS),
		GlobalDomainAsyncRPS:                        dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendGlobalDomainAsyncRPS),
		GlobalRatelimiterKeyMode:                    dc.GetStringPropertyFilteredByRatelimitKey(dynamicconfig.FrontendGlobalRatelimiterMode),
		GlobalRatelimiterUpdateInterval:             dc.GetDurationProperty(dynamicconfig.GlobalRatelimiterUpdateInterval),
		MaxIDLengthWarnLimit:                        dc.GetIntProperty(dynamicconfig.MaxIDLengthWarnLimit),
		DomainNameMaxLength:                         dc.GetIntPropertyFilteredByDomain(dynamicconfig.DomainNameMaxLength),
		IdentityMaxLength:                           dc.GetIntPropertyFilteredByDomain(dynamicconfig.IdentityMaxLength),
		WorkflowIDMaxLength:                         dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowIDMaxLength),
		SignalNameMaxLength:                         dc.GetIntPropertyFilteredByDomain(dynamicconfig.SignalNameMaxLength),
		WorkflowTypeMaxLength:                       dc.GetIntPropertyFilteredByDomain(dynamicconfig.WorkflowTypeMaxLength),
		RequestIDMaxLength:                          dc.GetIntPropertyFilteredByDomain(dynamicconfig.RequestIDMaxLength),
		TaskListNameMaxLength:                       dc.GetIntPropertyFilteredByDomain(dynamicconfig.TaskListNameMaxLength),
		EnableAdminProtection:                       dc.GetBoolProperty(dynamicconfig.EnableAdminProtection),
		AdminOperationToken:                         dc.GetStringProperty(dynamicconfig.AdminOperationToken),
		DisableListVisibilityByFilter:               dc.GetBoolPropertyFilteredByDomain(dynamicconfig.DisableListVisibilityByFilter),
		BlobSizeLimitError:                          dc.GetIntPropertyFilteredByDomain(dynamicconfig.BlobSizeLimitError),
		BlobSizeLimitWarn:                           dc.GetIntPropertyFilteredByDomain(dynamicconfig.BlobSizeLimitWarn),
		ThrottledLogRPS:                             dc.GetIntProperty(dynamicconfig.FrontendThrottledLogRPS),
		ShutdownDrainDuration:                       dc.GetDurationProperty(dynamicconfig.FrontendShutdownDrainDuration),
		EnableDomainNotActiveAutoForwarding:         dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableDomainNotActiveAutoForwarding),
		EnableGracefulFailover:                      dc.GetBoolProperty(dynamicconfig.EnableGracefulFailover),
		DomainFailoverRefreshInterval:               dc.GetDurationProperty(dynamicconfig.DomainFailoverRefreshInterval),
		DomainFailoverRefreshTimerJitterCoefficient: dc.GetFloat64Property(dynamicconfig.DomainFailoverRefreshTimerJitterCoefficient),
		EnableClientVersionCheck:                    dc.GetBoolProperty(dynamicconfig.EnableClientVersionCheck),
		EnableQueryAttributeValidation:              dc.GetBoolProperty(dynamicconfig.EnableQueryAttributeValidation),
		ValidSearchAttributes:                       dc.GetMapProperty(dynamicconfig.ValidSearchAttributes),
		SearchAttributesNumberOfKeysLimit:           dc.GetIntPropertyFilteredByDomain(dynamicconfig.SearchAttributesNumberOfKeysLimit),
		SearchAttributesSizeOfValueLimit:            dc.GetIntPropertyFilteredByDomain(dynamicconfig.SearchAttributesSizeOfValueLimit),
		SearchAttributesTotalSizeLimit:              dc.GetIntPropertyFilteredByDomain(dynamicconfig.SearchAttributesTotalSizeLimit),
		VisibilityArchivalQueryMaxPageSize:          dc.GetIntProperty(dynamicconfig.VisibilityArchivalQueryMaxPageSize),
		DisallowQuery:                               dc.GetBoolPropertyFilteredByDomain(dynamicconfig.DisallowQuery),
		SendRawWorkflowHistory:                      dc.GetBoolPropertyFilteredByDomain(dynamicconfig.SendRawWorkflowHistory),
		DecisionResultCountLimit:                    dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendDecisionResultCountLimit),
		EmitSignalNameMetricsTag:                    dc.GetBoolPropertyFilteredByDomain(dynamicconfig.FrontendEmitSignalNameMetricsTag),
		Lockdown:                                    dc.GetBoolPropertyFilteredByDomain(dynamicconfig.Lockdown),
		EnableTasklistIsolation:                     dc.GetBoolPropertyFilteredByDomain(dynamicconfig.EnableTasklistIsolation),
		DomainConfig: domain.Config{
			MaxBadBinaryCount:      dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendMaxBadBinaries),
			MinRetentionDays:       dc.GetIntProperty(dynamicconfig.MinRetentionDays),
			MaxRetentionDays:       dc.GetIntProperty(dynamicconfig.MaxRetentionDays),
			FailoverCoolDown:       dc.GetDurationPropertyFilteredByDomain(dynamicconfig.FrontendFailoverCoolDown),
			RequiredDomainDataKeys: dc.GetMapProperty(dynamicconfig.RequiredDomainDataKeys),
			FailoverHistoryMaxSize: dc.GetIntPropertyFilteredByDomain(dynamicconfig.FrontendFailoverHistoryMaxSize),
		},
		HostName: hostName,
	}
}
