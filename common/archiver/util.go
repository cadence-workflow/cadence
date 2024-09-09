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

package archiver

import (
	"errors"
	"fmt"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

var (
	errEmptyDomainID         = errors.New("DomainID is empty")
	errEmptyDomainName       = errors.New("DomainName is empty")
	errEmptyWorkflowID       = errors.New("WorkflowID is empty")
	errEmptyRunID            = errors.New("RunID is empty")
	errInvalidPageSize       = errors.New("PageSize should be greater than 0")
	errEmptyWorkflowTypeName = errors.New("WorkflowTypeName is empty")
	errEmptyStartTime        = errors.New("StartTimestamp is empty")
	errEmptyCloseTime        = errors.New("CloseTimestamp is empty")
	errEmptyQuery            = errors.New("Query string is empty")
)

// TagLoggerWithArchiveHistoryRequestAndURI tags logger with fields in the archive history request and the URI
func TagLoggerWithArchiveExecutionRequestAndURI(logger log.Logger, request *ArchiveExecutionRequest, URI string) log.Logger {
	return logger.WithTags(
		tag.ShardID(request.ShardID),
		tag.ArchivalRequestDomainID(request.DomainID),
		tag.ArchivalRequestDomainName(request.DomainName),
		tag.ArchivalRequestWorkflowID(request.WorkflowID),
		tag.ArchivalRequestRunID(request.RunID),
		tag.ArchivalURI(URI),
		tag.WorkflowID(request.WorkflowID),
		tag.WorkflowRunID(request.RunID),
	)
}

// TagLoggerWithArchiveHistoryRequestAndURI tags logger with fields in the archive history request and the URI
func TagLoggerWithArchiveHistoryRequestAndURI(logger log.Logger, request *ArchiveHistoryRequest, URI string) log.Logger {
	return logger.WithTags(
		tag.ShardID(request.ShardID),
		tag.ArchivalRequestDomainID(request.DomainID),
		tag.ArchivalRequestDomainName(request.DomainName),
		tag.ArchivalRequestWorkflowID(request.WorkflowID),
		tag.ArchivalRequestRunID(request.RunID),
		tag.ArchivalRequestBranchToken(request.BranchToken),
		tag.ArchivalRequestNextEventID(request.NextEventID),
		tag.ArchivalRequestCloseFailoverVersion(request.CloseFailoverVersion),
		tag.ArchivalURI(URI),
	)
}

// TagLoggerWithArchiveVisibilityRequestAndURI tags logger with fields in the archive visibility request and the URI
func TagLoggerWithArchiveVisibilityRequestAndURI(logger log.Logger, request *ArchiveVisibilityRequest, URI string) log.Logger {
	return logger.WithTags(
		tag.ArchivalRequestDomainID(request.DomainID),
		tag.ArchivalRequestDomainName(request.DomainName),
		tag.ArchivalRequestWorkflowID(request.WorkflowID),
		tag.ArchivalRequestRunID(request.RunID),
		tag.ArchvialRequestWorkflowType(request.WorkflowTypeName),
		tag.ArchivalRequestCloseTimestamp(request.CloseTimestamp),
		tag.ArchivalRequestCloseStatus(request.CloseStatus.String()),
		tag.ArchivalURI(URI),
	)
}

// ValidateHistoryArchiveRequest validates the archive history request
func ValidateHistoryArchiveRequest(request *ArchiveHistoryRequest) error {
	if request.DomainID == "" {
		return errEmptyDomainID
	}
	if request.WorkflowID == "" {
		return errEmptyWorkflowID
	}
	if request.RunID == "" {
		return errEmptyRunID
	}
	if request.DomainName == "" {
		return errEmptyDomainName
	}
	return nil
}

// ValidateExecutionArchiveRequest validates the archive execution request
func ValidateExecutionArchiveRequest(req *ArchiveExecutionRequest) error {
	if req == nil {
		return &ErrInvalidArchivalRequest{
			Message: "Invalid execution archival request",
			Request: req,
		}
	}
	if req.DomainID == "" {
		return errEmptyDomainID
	}
	if req.ShardID < 0 {
		return errEmptyDomainID
	}
	if req.WorkflowID == "" {
		return errEmptyWorkflowID
	}
	if req.RunID == "" {
		return errEmptyRunID
	}
	if req.DomainName == "" {
		return errEmptyDomainName
	}
	return nil
}

// ValidateExecutionArchiveResponse checks the workflow execution is sane to archive/delete
func ValidateExecutionArchiveResponse(res *persistence.GetWorkflowExecutionResponse) error {
	if res == nil || res.State == nil {
		return nil
	}
	if res.State.ExecutionInfo.State != persistence.WorkflowStateCompleted {
		return fmt.Errorf("workflow is not closed. Cannot archive workflow")
	}
	return nil
}

// ValidateGetRequest validates the get archived history request
func ValidateGetRequest(request *GetHistoryRequest) error {
	if request.DomainID == "" {
		return errEmptyDomainID
	}
	if request.WorkflowID == "" {
		return errEmptyWorkflowID
	}
	if request.RunID == "" {
		return errEmptyRunID
	}
	if request.PageSize == 0 {
		return errInvalidPageSize
	}
	return nil
}

// ValidateVisibilityArchivalRequest validates the archive visibility request
func ValidateVisibilityArchivalRequest(request *ArchiveVisibilityRequest) error {
	if request.DomainID == "" {
		return errEmptyDomainID
	}
	if request.DomainName == "" {
		return errEmptyDomainName
	}
	if request.WorkflowID == "" {
		return errEmptyWorkflowID
	}
	if request.RunID == "" {
		return errEmptyRunID
	}
	if request.WorkflowTypeName == "" {
		return errEmptyWorkflowTypeName
	}
	if request.StartTimestamp == 0 {
		return errEmptyStartTime
	}
	if request.CloseTimestamp == 0 {
		return errEmptyCloseTime
	}
	return nil
}

// ValidateQueryRequest validates the query visibility request
func ValidateQueryRequest(request *QueryVisibilityRequest) error {
	if request.DomainID == "" {
		return errEmptyDomainID
	}
	if request.PageSize == 0 {
		return errInvalidPageSize
	}
	if request.Query == "" {
		return errEmptyQuery
	}
	return nil
}

// ConvertSearchAttrToBytes converts search attribute value from string back to byte array
func ConvertSearchAttrToBytes(searchAttrStr map[string]string) map[string][]byte {
	searchAttr := make(map[string][]byte)
	for k, v := range searchAttrStr {
		searchAttr[k] = []byte(v)
	}
	return searchAttr
}

func IsHistoryMutated(request *ArchiveHistoryRequest, historyBatches []*types.History, isLast bool, logger log.Logger) (mutated bool) {
	lastBatch := historyBatches[len(historyBatches)-1].Events
	lastEvent := lastBatch[len(lastBatch)-1]
	lastFailoverVersion := lastEvent.Version
	defer func() {
		if mutated {
			logger.Warn(ArchiveNonRetriableErrorMsg+":history is mutated when during archival",
				tag.ArchivalArchiveFailReason(ErrReasonHistoryMutated),
				tag.FailoverVersion(lastFailoverVersion),
				tag.TokenLastEventID(lastEvent.ID))
		}
	}()
	if lastFailoverVersion > request.CloseFailoverVersion {
		return true
	}

	if !isLast {
		return false
	}
	lastEventID := lastEvent.ID
	return lastFailoverVersion != request.CloseFailoverVersion || lastEventID+1 != request.NextEventID
}
