// The MIT License (MIT)
//
// Copyright (c) 2020 Uber Technologies, Inc.
//
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
package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/blobstore/filestore"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/pagination"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/entity"
	"github.com/uber/cadence/common/reconciliation/fetcher"
)

var (
	validBranchToken  = []byte{89, 11, 0, 10, 0, 0, 0, 12, 116, 101, 115, 116, 45, 116, 114, 101, 101, 45, 105, 100, 11, 0, 20, 0, 0, 0, 14, 116, 101, 115, 116, 45, 98, 114, 97, 110, 99, 104, 45, 105, 100, 0}
	executionPageSize = 10
	testShardID       = 1
)

func TestWriterIterator(t *testing.T) {
	testCases := []struct {
		name          string
		pages         int
		countPerPage  int
		expectedCount int // Add this field
	}{
		{"StandardCase", 10, 10, 100},
		{"FewPages", 3, 15, 45}, // Set expectedCount as pages * countPerPage
		{"SingleLargePage", 1, 100, 100},
		{"ManySmallPages", 20, 2, 40},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assertions := require.New(t)
			pr := persistence.NewPersistenceRetryer(getMockExecutionManager(tc.pages, tc.countPerPage), nil, common.CreatePersistenceRetryPolicy())
			pItr := fetcher.ConcreteExecutionIterator(context.Background(), pr, executionPageSize)

			uuid := "uuid"
			extension := Extension("test")
			outputDir := t.TempDir()
			cfg := &config.FileBlobstore{
				OutputDirectory: outputDir,
			}
			blobstore, err := filestore.NewFilestoreClient(cfg)
			assertions.NoError(err)
			blobstoreWriter := NewBlobstoreWriter(uuid, extension, blobstore, 10)

			var outputs []*ScanOutputEntity
			for pItr.HasNext() {
				exec, err := pItr.Next()
				assertions.NoError(err)
				soe := &ScanOutputEntity{
					Execution: exec,
				}
				outputs = append(outputs, soe)
				assertions.NoError(blobstoreWriter.Add(soe))
			}
			assertions.NoError(blobstoreWriter.Flush())
			assertions.Len(outputs, tc.pages*tc.countPerPage)
			assertions.False(pItr.HasNext())
			_, err = pItr.Next()
			assertions.Equal(pagination.ErrIteratorFinished, err)
			flushedKeys := blobstoreWriter.FlushedKeys()
			assertions.Equal(uuid, flushedKeys.UUID)
			assertions.Equal(0, flushedKeys.MinPage)
			if tc.expectedCount > 0 {
				assertions.Equal((tc.expectedCount-1)/10, flushedKeys.MaxPage)
			} else {
				assertions.Equal(0, flushedKeys.MaxPage)
			}
			assertions.Equal(Extension("test"), flushedKeys.Extension)

			blobstoreItr := NewBlobstoreIterator(context.Background(), blobstore, *flushedKeys, &entity.ConcreteExecution{})
			i := 0
			for blobstoreItr.HasNext() {
				scanOutputEntity, err := blobstoreItr.Next()
				if err != nil {
					assertions.Fail(fmt.Sprintf("Error iterating blobstore: %v", err))
					break
				}
				if scanOutputEntity == nil {
					break // No more items
				}
				if i < len(outputs) {
					// Compare the Execution field of ScanOutputEntity with the expected ConcreteExecution
					assertions.Equal(outputs[i].Execution, scanOutputEntity.Execution)
				}
				i++
			}
			assertions.Equal(tc.expectedCount, i, "Number of items from blobstore iterator mismatch")
		})
	}

}

func getMockExecutionManager(pages int, countPerPage int) persistence.ExecutionManager {
	execManager := &mocks.ExecutionManager{}
	for i := 0; i < pages; i++ {
		req := &persistence.ListConcreteExecutionsRequest{
			PageToken: []byte(fmt.Sprintf("token_%v", i)),
			PageSize:  executionPageSize,
		}
		if i == 0 {
			req.PageToken = nil
		}
		resp := &persistence.ListConcreteExecutionsResponse{
			Executions: getExecutions(countPerPage),
			PageToken:  []byte(fmt.Sprintf("token_%v", i+1)),
		}
		if i == pages-1 {
			resp.PageToken = nil
		}
		execManager.On("ListConcreteExecutions", mock.Anything, req).Return(resp, nil)
		execManager.On("GetShardID").Return(testShardID)
	}
	return execManager
}

func getExecutions(count int) []*persistence.ListConcreteExecutionsEntity {
	var result []*persistence.ListConcreteExecutionsEntity
	for i := 0; i < count; i++ {
		execution := &persistence.ListConcreteExecutionsEntity{
			ExecutionInfo: &persistence.WorkflowExecutionInfo{
				DomainID:    uuid.New(),
				WorkflowID:  uuid.New(),
				RunID:       uuid.New(),
				BranchToken: validBranchToken,
				State:       0,
			},
		}
		if i%2 == 0 {
			execution.ExecutionInfo.BranchToken = nil
			execution.VersionHistories = &persistence.VersionHistories{
				CurrentVersionHistoryIndex: 0,
				Histories: []*persistence.VersionHistory{
					{
						BranchToken: validBranchToken,
					},
				},
			}
		}
		result = append(result, execution)
	}
	return result
}
