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

package warm_storage

//
//import (
//	"context"
//	"errors"
//
//	"github.com/uber/cadence/common/archiver"
//	"github.com/uber/cadence/common/dynamicconfig"
//	"github.com/uber/cadence/common/log"
//	"github.com/uber/cadence/common/metrics"
//	"github.com/uber/cadence/common/persistence"
//	"github.com/uber/cadence/common/types"
//)
//
//func NewWarmStorageHistoryManager(
//	log log.Logger,
//	metrics metrics.Client,
//) (WarmStorageHistoryManager, error) {
//	return &historyWarmStorageImpl{
//		log:     log,
//		metrics: metrics,
//	}, nil
//}
//
//type historyWarmStorageImpl struct {
//	persistence.HistoryManager
//
//	dc       dynamicconfig.Client
//	log      log.Logger
//	metrics  metrics.Client
//	archiver archiver.HistoryArchiver
//}
//
//var _ persistence.HistoryManager = &historyWarmStorageImpl{}
//
//func (w *historyWarmStorageImpl) GetHistoryBranch(
//	ctx context.Context,
//	request *persistence.ReadHistoryBranchRequest,
//) (*persistence.ReadHistoryBranchResponse, error) {
//
//	// 'happy-path', the workflow's still running and it's found
//	// in the database. Return it immediately
//	res, lookupErr := w.HistoryManager.ReadHistoryBranch(ctx, request)
//	if lookupErr == nil {
//		return res, nil
//	}
//
//	// if the lookup *is not* a not-found error, handle that by
//	// returning it unchanged
//	var e *types.EntityNotExistsError
//	if !errors.As(lookupErr, &e) {
//		return nil, lookupErr
//	}
//
//	// the error is a not-found error. Look for it in warm storage
//	req := archiver.GetHistoryRequest{
//		DomainID:             "",
//		RunID:                "",
//		CloseFailoverVersion: nil,
//		NextPageToken:        nil,
//		PageSize:             0,
//	}
//
//	archiverRes, err := w.archiver.GetWorkflowHistoryForPersistence(ctx, &req)
//	if err != nil {
//		return nil, err
//	}
//	return &persistence.GetWorkflowExecutionResponse{
//		State: archiverRes.MutableState,
//	}, nil
//}
