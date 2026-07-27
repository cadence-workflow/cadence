// Copyright (c) 2026 Uber Technologies, Inc.
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

package mongodb

import (
	"context"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func (db *mdb) InsertIntoAsyncWorkflowQueue(ctx context.Context, row *nosqlplugin.AsyncWorkflowQueueMessageRow) error {
	panic("TODO")
}

func (db *mdb) SelectLastAsyncWorkflowMessageID(ctx context.Context, shardID int) (int64, error) {
	panic("TODO")
}

func (db *mdb) SelectAsyncWorkflowMessagesFrom(ctx context.Context, shardID int, exclusiveBeginMessageID int64, maxRows int) ([]*nosqlplugin.AsyncWorkflowQueueMessageRow, error) {
	panic("TODO")
}

func (db *mdb) RangeDeleteAsyncWorkflowMessages(ctx context.Context, shardID int, inclusiveEndMessageID int64) error {
	panic("TODO")
}

func (db *mdb) InsertAsyncWorkflowQueueMetadata(ctx context.Context, row nosqlplugin.AsyncWorkflowQueueMetadataRow) error {
	panic("TODO")
}

func (db *mdb) UpdateAsyncWorkflowQueueMetadataCas(ctx context.Context, row nosqlplugin.AsyncWorkflowQueueMetadataRow) error {
	panic("TODO")
}

func (db *mdb) SelectAsyncWorkflowQueueMetadata(ctx context.Context, shardID int) (*nosqlplugin.AsyncWorkflowQueueMetadataRow, error) {
	panic("TODO")
}

func (db *mdb) InsertIntoAsyncWorkflowDLQ(ctx context.Context, row *nosqlplugin.AsyncWorkflowQueueMessageRow) error {
	panic("TODO")
}

func (db *mdb) SelectLastAsyncWorkflowDLQMessageID(ctx context.Context, shardID int) (int64, error) {
	panic("TODO")
}

func (db *mdb) SelectAsyncWorkflowDLQMessagesFrom(ctx context.Context, shardID int, exclusiveBeginMessageID int64, maxRows int) ([]*nosqlplugin.AsyncWorkflowQueueMessageRow, error) {
	panic("TODO")
}

func (db *mdb) RangeDeleteAsyncWorkflowDLQMessages(ctx context.Context, shardID int, inclusiveEndMessageID int64) error {
	panic("TODO")
}
