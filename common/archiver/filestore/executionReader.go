package filestore

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/util"
	"path/filepath"
)

type executionReader struct {
	log         log.Logger
	metrics     metrics.Client
	domainCache cache.DomainCache
}

// NewExecutionReader is used for reading execution values
func NewExecutionReader(
	logger log.Logger,
	metrics metrics.Client,
	domainCache cache.DomainCache,
) (archiver.ExecutionReader, error) {
	return &executionReader{
		log:         logger,
		metrics:     metrics,
		domainCache: domainCache,
	}, nil
}

func (e *executionReader) GetWorkflowExecutionForPersistence(ctx context.Context, req *archiver.GetExecutionRequest) (*archiver.GetExecutionResponse, error) {
	domain, err := e.domainCache.GetDomainByID(req.DomainID)
	if err != nil {
		return nil, err
	}

	uri, err := archiver.NewURI(domain.GetConfig().HistoryArchivalURI)
	if err != nil {
		return nil, err
	}
	return e.GetWorkflowExecution(ctx, uri, req)
}

func (e *executionReader) GetWorkflowExecution(ctx context.Context, URI archiver.URI, req *archiver.GetExecutionRequest) (*archiver.GetExecutionResponse, error) {
	dirPath := URI.Path()
	wfPath := constructExecutionFilename(req.DomainID, req.WorkflowID, req.RunID)
	data, err := util.ReadFile(filepath.Join(dirPath, wfPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow execution from archived filepath: %w", err)
	}

	var ms persistence.WorkflowMutableState
	json.Unmarshal(data, &ms)
	return &archiver.GetExecutionResponse{
		MutableState: &ms,
	}, nil
}
