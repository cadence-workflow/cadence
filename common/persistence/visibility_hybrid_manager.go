// Copyright (c) 2021 Uber Technologies, Inc.
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

package persistence

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

type (
	visibilityHybridManager struct {
		logger                    log.Logger
		visibilityMgrs            map[string]VisibilityManager
		readVisibilityStoreName   dynamicproperties.StringPropertyFnWithDomainFilter
		writeVisibilityStoreName  dynamicproperties.StringPropertyFn
		logCustomerQueryParameter dynamicproperties.BoolPropertyFnWithDomainFilter
		name                      string
	}
)

const (
	ContextKey           = ResponseComparatorContextKey("visibility-override")
	dbVisStoreName       = "db"
	advancedWriteModeOff = "off"
)

// ResponseComparatorContextKey is for Pinot/ES response comparator. This struct will be passed into ctx as a key.
type ResponseComparatorContextKey string

type OperationType string

var Operation = struct {
	LIST  OperationType
	COUNT OperationType
}{
	LIST:  "list",
	COUNT: "count",
}

var _ VisibilityManager = (*visibilityHybridManager)(nil)

// NewVisibilityTripleManager create a visibility manager that operate on DB or advanced visibility based on dynamic config.
// For Pinot migration, Pinot is the destination visibility manager, ES is the source visibility manager, and DB is the fallback.
// For OpenSearch migration, OS is the destination visibility manager, ES is the source visibility manager, and DB is the fallback.
func NewVisibilityHybridManager(
	visibilityMgrs map[string]VisibilityManager,
	readVisibilityStoreName dynamicproperties.StringPropertyFnWithDomainFilter,
	writeVisibilityStoreName dynamicproperties.StringPropertyFn,
	logCustomerQueryParameter dynamicproperties.BoolPropertyFnWithDomainFilter,
	name string,
	logger log.Logger,
) VisibilityManager {
	if len(visibilityMgrs) == 0 {
		logger.Fatal("No visibility managers provided. At least one visibility manager is required.")
		return nil
	}

	if logCustomerQueryParameter == nil {
		logCustomerQueryParameter = dynamicproperties.GetBoolPropertyFnFilteredByDomain(false)
	}

	return &visibilityHybridManager{
		visibilityMgrs:            visibilityMgrs,
		readVisibilityStoreName:   readVisibilityStoreName,
		writeVisibilityStoreName:  writeVisibilityStoreName,
		logger:                    logger,
		logCustomerQueryParameter: logCustomerQueryParameter,
		name:                      name,
	}
}

func (v *visibilityHybridManager) Close() {
	for _, mgr := range v.visibilityMgrs {
		if mgr != nil {
			mgr.Close()
		}
	}
}

func (v *visibilityHybridManager) GetName() string {
	return v.name
}

func (v *visibilityHybridManager) RecordWorkflowExecutionStarted(
	ctx context.Context,
	request *RecordWorkflowExecutionStartedRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func(storeName string) error {
			mgr, ok := v.visibilityMgrs[storeName]
			if !ok || mgr == nil {
				return fmt.Errorf("Visibility store manager with name %s not found", storeName)
			}
			return mgr.RecordWorkflowExecutionStarted(ctx, request)
		},
	)
}

func (v *visibilityHybridManager) RecordWorkflowExecutionClosed(
	ctx context.Context,
	request *RecordWorkflowExecutionClosedRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func(storeName string) error {
			mgr, ok := v.visibilityMgrs[storeName]
			if !ok || mgr == nil {
				return fmt.Errorf("Visibility store manager with name %s not found", storeName)
			}
			return mgr.RecordWorkflowExecutionClosed(ctx, request)
		},
	)
}

func (v *visibilityHybridManager) RecordWorkflowExecutionUninitialized(
	ctx context.Context,
	request *RecordWorkflowExecutionUninitializedRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func(storeName string) error {
			mgr, ok := v.visibilityMgrs[storeName]
			if !ok || mgr == nil {
				return fmt.Errorf("Visibility store manager with name %s not found", storeName)
			}
			return mgr.RecordWorkflowExecutionUninitialized(ctx, request)
		},
	)
}

func (v *visibilityHybridManager) DeleteWorkflowExecution(
	ctx context.Context,
	request *VisibilityDeleteWorkflowExecutionRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func(storeName string) error {
			mgr, ok := v.visibilityMgrs[storeName]
			if !ok || mgr == nil {
				return fmt.Errorf("Visibility store manager with name %s not found", storeName)
			}
			return mgr.DeleteWorkflowExecution(ctx, request)
		},
	)
}

func (v *visibilityHybridManager) DeleteUninitializedWorkflowExecution(
	ctx context.Context,
	request *VisibilityDeleteWorkflowExecutionRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func(storeName string) error {
			mgr, ok := v.visibilityMgrs[storeName]
			if !ok || mgr == nil {
				return fmt.Errorf("Visibility store manager with name %s not found", storeName)
			}
			return mgr.DeleteUninitializedWorkflowExecution(ctx, request)
		},
	)
}

func (v *visibilityHybridManager) UpsertWorkflowExecution(
	ctx context.Context,
	request *UpsertWorkflowExecutionRequest,
) error {
	return v.chooseVisibilityManagerForWrite(
		ctx,
		func(storeName string) error {
			mgr, ok := v.visibilityMgrs[storeName]
			if !ok || mgr == nil {
				return fmt.Errorf("Visibility store manager with name %s not found", storeName)
			}
			return mgr.UpsertWorkflowExecution(ctx, request)
		},
	)
}

func (v *visibilityHybridManager) chooseVisibilityModeForAdmin() string {
	var modes []string

	for storeName, mgr := range v.visibilityMgrs {
		if mgr != nil {
			modes = append(modes, storeName)
		}
	}

	if len(modes) == 0 {
		return "INVALID_ADMIN_MODE"
	}

	return strings.Join(modes, ",")
}

func (v *visibilityHybridManager) chooseVisibilityManagerForWrite(ctx context.Context, visFunc func(string) error) error {
	var writeMode string
	if v.writeVisibilityStoreName != nil {
		writeMode = v.writeVisibilityStoreName()
	} else {
		key := VisibilityAdminDeletionKey("visibilityAdminDelete")
		if value := ctx.Value(key); value != nil && value.(bool) {
			writeMode = v.chooseVisibilityModeForAdmin()
		}
	}
	modes := strings.Split(writeMode, ",")
	for i := range modes {
		modes[i] = strings.ToLower(strings.TrimSpace(modes[i]))
	}

	var errors []string
	for _, mode := range modes {
		if mode == advancedWriteModeOff {
			mode = dbVisStoreName
		}
		if mgr, ok := v.visibilityMgrs[mode]; ok && mgr != nil {
			if err := visFunc(mode); err != nil {
				errors = append(errors, err.Error())
			}
		} else if mode != dbVisStoreName && !strings.Contains(writeMode, dbVisStoreName) {
			// If requested mode is not available and it's not already "db", fall back to "db"
			// when write mode already includes db, skip this step since it will perform the write in another loop
			v.logger.Warn("requested visibility mode is not available, falling back to db", tag.Value(mode))
			if err := visFunc(dbVisStoreName); err != nil {
				errors = append(errors, err.Error())
			}
		} else {
			// If the mode is "db" but not available, this is an error
			// This is the else case - when mode is "db" but the manager is not available
			errors = append(errors, fmt.Sprintf("DB visibility mode is not available: %s", mode))
		}
	}

	if len(errors) > 0 {
		return &types.InternalServiceError{
			Message: fmt.Sprintf("Error writing to visibility: %v", strings.Join(errors, "; ")),
		}
	}

	return nil
}

// For Pinot Migration uses. It will be a temporary usage
type userParameters struct {
	operation    string
	domainName   string
	workflowType string
	workflowID   string
	closeStatus  int // if it is -1, then will have --open flag in comparator workflow
	customQuery  string
	earliestTime int64
	latestTime   int64
}

// For Visibility Migration uses. It will be a temporary usage
// logUserQueryParameters will log user queries' parameters so that a comparator workflow can consume
func (v *visibilityHybridManager) logUserQueryParameters(userParam userParameters, domain string, override bool) {
	// Don't log if it is not enabled
	// don't log if it is a call from Pinot Response Comparator workflow
	if !v.logCustomerQueryParameter(domain) || override {
		return
	}

	randNum := rand.Intn(10)
	if randNum != 5 { // Intentionally to have 1/10 chance to log custom query parameters
		return
	}

	v.logger.Info("Logging user query parameters for visibility migration response comparator...",
		tag.OperationName(userParam.operation),
		tag.WorkflowDomainName(userParam.domainName),
		tag.WorkflowType(userParam.workflowType),
		tag.WorkflowID(userParam.workflowID),
		tag.WorkflowCloseStatus(userParam.closeStatus),
		tag.VisibilityQuery(filterAttrPrefix(userParam.customQuery)),
		tag.EarliestTime(userParam.earliestTime),
		tag.LatestTime(userParam.latestTime))

}

// This is for only logUserQueryParameters (for Pinot Response comparator) usage.
// Be careful because there's a low possibility that there'll be false positive cases (shown in unit tests)
func filterAttrPrefix(str string) string {
	str = strings.Replace(str, "`Attr.", "", -1)
	return strings.Replace(str, "`", "", -1)
}

func (v *visibilityHybridManager) ListOpenWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListOpenWorkflowExecutions, request, v.logger)
	}
	// return result from primary
	return manager.ListOpenWorkflowExecutions(ctx, request)
}

func (v *visibilityHybridManager) ListClosedWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListClosedWorkflowExecutions, request, v.logger)
	}
	return manager.ListClosedWorkflowExecutions(ctx, request)
}

func (v *visibilityHybridManager) ListOpenWorkflowExecutionsByType(
	ctx context.Context,
	request *ListWorkflowExecutionsByTypeRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListOpenWorkflowExecutionsByType, request, v.logger)
	}
	return manager.ListOpenWorkflowExecutionsByType(ctx, request)
}

func (v *visibilityHybridManager) ListClosedWorkflowExecutionsByType(
	ctx context.Context,
	request *ListWorkflowExecutionsByTypeRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListClosedWorkflowExecutionsByType, request, v.logger)
	}
	return manager.ListClosedWorkflowExecutionsByType(ctx, request)
}

func (v *visibilityHybridManager) ListOpenWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *ListWorkflowExecutionsByWorkflowIDRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListOpenWorkflowExecutionsByWorkflowID, request, v.logger)
	}
	return manager.ListOpenWorkflowExecutionsByWorkflowID(ctx, request)
}

func (v *visibilityHybridManager) ListClosedWorkflowExecutionsByWorkflowID(
	ctx context.Context,
	request *ListWorkflowExecutionsByWorkflowIDRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListClosedWorkflowExecutionsByWorkflowID, request, v.logger)
	}
	return manager.ListClosedWorkflowExecutionsByWorkflowID(ctx, request)
}

func (v *visibilityHybridManager) ListClosedWorkflowExecutionsByStatus(
	ctx context.Context,
	request *ListClosedWorkflowExecutionsByStatusRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListClosedWorkflowExecutionsByStatus, request, v.logger)
	}
	return manager.ListClosedWorkflowExecutionsByStatus(ctx, request)
}

func (v *visibilityHybridManager) GetClosedWorkflowExecution(
	ctx context.Context,
	request *GetClosedWorkflowExecutionRequest,
) (*GetClosedWorkflowExecutionResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.GetClosedWorkflowExecution, request, v.logger)
	}
	return manager.GetClosedWorkflowExecution(ctx, request)
}

func (v *visibilityHybridManager) ListWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsByQueryRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ListWorkflowExecutions, request, v.logger)
	}
	return manager.ListWorkflowExecutions(ctx, request)
}

func (v *visibilityHybridManager) ScanWorkflowExecutions(
	ctx context.Context,
	request *ListWorkflowExecutionsByQueryRequest,
) (*ListWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.ScanWorkflowExecutions, request, v.logger)
	}
	return manager.ScanWorkflowExecutions(ctx, request)
}

func (v *visibilityHybridManager) CountWorkflowExecutions(
	ctx context.Context,
	request *CountWorkflowExecutionsRequest,
) (*CountWorkflowExecutionsResponse, error) {
	manager, shadowMgr := v.chooseVisibilityManagerForRead(ctx, request.Domain)
	if shadowMgr != nil {
		go shadow(shadowMgr.CountWorkflowExecutions, request, v.logger)
	}
	return manager.CountWorkflowExecutions(ctx, request)
}

func (v *visibilityHybridManager) chooseVisibilityManagerForRead(ctx context.Context, domain string) (VisibilityManager, VisibilityManager) {
	var visibilityMgr, shadowMgr VisibilityManager
	stores := strings.Split(v.readVisibilityStoreName(domain), ",")
	for i := range stores {
		stores[i] = strings.ToLower(strings.TrimSpace(stores[i]))
	}
	readStore := stores[0] // if read stores have more than 1, the others will go shadow read
	if v.visibilityMgrs[readStore] != nil {
		visibilityMgr = v.visibilityMgrs[readStore]
	} else {
		v.logger.Warn("domain is configured to read from advanced visibility but it's not available, fall back to basic visibility",
			tag.WorkflowDomainName(domain))
		visibilityMgr = v.visibilityMgrs[dbVisStoreName] //db will always be available
	}

	if len(stores) > 1 {
		shadowMgr = v.visibilityMgrs[stores[1]]
	}

	return visibilityMgr, shadowMgr
}

func shadow[ReqT any, ResT any](f func(ctx context.Context, request ReqT) (ResT, error), request ReqT, logger log.Logger) {
	ctxNew, cancel := context.WithTimeout(context.Background(), 30*time.Second) // don't want f to run too long

	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			logger.Info(fmt.Sprintf("Recovered in Shadow function in double read: %v", r))
		}
	}()

	_, err := f(ctxNew, request)
	if err != nil {
		logger.Error(fmt.Sprintf("Error in Shadow function in double read: %s", err.Error()))
	}
}
