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

package filestore

import (
	"context"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/util"
	"path"

	"github.com/uber/cadence/common/archiver"
)

type executionArchiver struct {
	// todo
}

func (e *executionArchiver) ArchiveExecution(ctx context.Context, URI archiver.URI, req *archiver.ArchiveExecutionRequest, opts ...archiver.ArchiveOption) (err error) {

	featureCatalog := archiver.GetFeatureCatalog(opts...)
	defer func() {
		if err != nil && !persistence.IsTransientError(err) && featureCatalog.NonRetriableError != nil {
			err = featureCatalog.NonRetriableError()
		}
	}()

	logger := archiver.TagLoggerWithArchiveHistoryRequestAndURI(h.container.Logger, request, URI.String())

	if err := h.ValidateURI(URI); err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(archiver.ErrReasonInvalidURI), tag.Error(err))
		return err
	}

	if err := archiver.ValidateHistoryArchiveRequest(request); err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(archiver.ErrReasonInvalidArchiveRequest), tag.Error(err))
		return err
	}

	historyIterator := h.historyIterator
	if historyIterator == nil { // will only be set by testing code
		historyIterator = archiver.NewHistoryIterator(ctx, request, h.container.HistoryV2Manager, targetHistoryBlobSize)
	}

	historyBatches := []*types.History{}
	for historyIterator.HasNext() {
		historyBlob, err := getNextHistoryBlob(ctx, historyIterator)
		if err != nil {
			if common.IsEntityNotExistsError(err) {
				// workflow history no longer exists, may due to duplicated archival signal
				// this may happen even in the middle of iterating history as two archival signals
				// can be processed concurrently.
				logger.Info(archiver.ArchiveSkippedInfoMsg)
				return nil
			}

			logger := logger.WithTags(tag.ArchivalArchiveFailReason(archiver.ErrReasonReadHistory), tag.Error(err))
			if !persistence.IsTransientError(err) {
				logger.Error(archiver.ArchiveNonRetriableErrorMsg)
			} else {
				logger.Error(archiver.ArchiveTransientErrorMsg)
			}
			return err
		}

		if archiver.IsHistoryMutated(request, historyBlob.Body, *historyBlob.Header.IsLast, logger) {
			if !featureCatalog.ArchiveIncompleteHistory() {
				return archiver.ErrHistoryMutated
			}
		}

		historyBatches = append(historyBatches, historyBlob.Body...)
	}

	encodedHistoryBatches, err := encode(historyBatches)
	if err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(errEncodeHistory), tag.Error(err))
		return err
	}

	dirPath := URI.Path()
	if err = util.MkdirAll(dirPath, h.dirMode); err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(errMakeDirectory), tag.Error(err))
		return err
	}

	filename := constructHistoryFilename(request.DomainID, request.WorkflowID, request.RunID, request.CloseFailoverVersion)
	if err := util.WriteFile(path.Join(dirPath, filename), encodedHistoryBatches, h.fileMode); err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(errWriteFile), tag.Error(err))
		return err
	}

	panic("not implemented")
}
func (e *executionArchiver) GetWorkflowExecution(context.Context, archiver.URI, *archiver.GetExecutionRequest) (*archiver.GetExecutionResponse, error) {
	panic("not implementd")
}
func (e *executionArchiver) ValidateExecutionURI(archiver.URI) error {
	panic("not implemented")
}
