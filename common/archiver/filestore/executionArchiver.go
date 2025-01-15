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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/util"
)

type executionArchiver struct {
	container archiver.ExecutionArchiverBootstrapContainer

	fileMode os.FileMode
	dirMode  os.FileMode
}

// NewHistoryArchiver creates a new archiver.HistoryArchiver based on filestore
func NewExecutionArchiver(
	container archiver.ExecutionArchiverBootstrapContainer,
	config *config.FilestoreArchiver,
) (archiver.ExecutionArchiver, error) {
	return newExecutionArchiver(container, config)
}

func newExecutionArchiver(
	container archiver.ExecutionArchiverBootstrapContainer,
	config *config.FilestoreArchiver,
) (*executionArchiver, error) {
	fileMode, err := strconv.ParseUint(config.FileMode, 0, 32)
	if err != nil {
		return nil, errInvalidFileMode
	}
	dirMode, err := strconv.ParseUint(config.DirMode, 0, 32)
	if err != nil {
		return nil, errInvalidDirMode
	}
	return &executionArchiver{
		container: container,
		fileMode:  os.FileMode(fileMode),
		dirMode:   os.FileMode(dirMode),
	}, nil
}

func (e *executionArchiver) ValidateExecutionURI(URI archiver.URI) error {
	if URI == nil || URI.Path() == "" {
		return archiver.ErrInvalidURI
	}
	return nil
}

func (e *executionArchiver) ValidateExecutionArchiveRequest(req *archiver.ArchiveHistoryRequest, uri archiver.URI) error {
	if req == nil {
		return &archiver.ErrInvalidArchivalRequest{
			Message: "Invalid execution archival request",
			URI:     uri,
			Request: req,
		}
	}
	return nil
}

func (e *executionArchiver) ArchiveExecution(ctx context.Context, URI archiver.URI, req *archiver.ArchiveExecutionRequest, opts ...archiver.ArchiveOption) (err error) {

	featureCatalog := archiver.GetFeatureCatalog(opts...)
	defer func() {
		if err != nil && !persistence.IsTransientError(err) && featureCatalog.NonRetriableError != nil {
			err = featureCatalog.NonRetriableError()
		}
	}()

	logger := archiver.TagLoggerWithArchiveExecutionRequestAndURI(e.container.Logger, req, URI.String())

	if err := e.ValidateExecutionURI(URI); err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(archiver.ErrReasonInvalidURI), tag.Error(err))
		return err
	}

	if err := archiver.ValidateExecutionArchiveRequest(req); err != nil {
		logger.Error(archiver.ArchiveNonRetriableErrorMsg, tag.ArchivalArchiveFailReason(archiver.ErrReasonInvalidArchiveRequest), tag.Error(err))
		return err
	}

	executionMgr, err := e.container.ExecutionMgr(req.ShardID)
	if err != nil {
		return fmt.Errorf("could not get shard manager: %w", err)
	}

	execution, err := executionMgr.GetWorkflowExecution(ctx, &persistence.GetWorkflowExecutionRequest{
		DomainID: req.DomainID,
		Execution: types.WorkflowExecution{
			WorkflowID: req.WorkflowID,
			RunID:      req.RunID,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get workflow for archival: %w", err)
	}

	err = archiver.ValidateExecutionArchiveResponse(execution)
	if err != nil {
		logger.Error("couldn't not archive workflow, it failed validation", tag.Error(err))
		return fmt.Errorf("could not archive workflow execution: %w", err)
	}

	jsonEncoded, err := json.Marshal(execution.State)
	if err != nil {
		logger.Error("couldn't not archive workflow, encoding failed", tag.Error(err))
		return fmt.Errorf("could not encode workflow execution for archival: %w", err)
	}

	dirPath := URI.Path()

	wfPath := constructExecutionFilename(req.DomainID, req.WorkflowID, req.RunID)
	err = util.MkdirAll(dirPath, e.dirMode)
	if err != nil {
		logger.Error("couldn't not create folder for archival", tag.Error(err), tag.Dynamic("filepath", dirPath))
		return fmt.Errorf("could not create folder for archival: %w", err)
	}

	err = util.WriteFile(filepath.Join(dirPath, wfPath), jsonEncoded, e.fileMode)
	if err != nil {
		logger.Error("failed to write file for archival", tag.Error(err))
		return fmt.Errorf("failed to write file to %q, error: %w", filepath.Join(dirPath, wfPath), err)
	}

	return nil
}
