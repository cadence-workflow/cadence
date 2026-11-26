package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func (db *mdb) InsertAsyncWorkflowMessage(ctx context.Context, row *nosqlplugin.AsyncWorkflowMessageRow) error {
	return fmt.Errorf("InsertAsyncWorkflowMessage not implemented for MongoDB")
}

func (db *mdb) SelectAsyncWorkflowMessages(ctx context.Context, shardID int, sourceCluster string, exclusiveMinMessageID int64, pageSize int) ([]*nosqlplugin.AsyncWorkflowMessageRow, error) {
	return nil, fmt.Errorf("SelectAsyncWorkflowMessages not implemented for MongoDB")
}

func (db *mdb) SelectAsyncWorkflowAckLevels(ctx context.Context, shardID int) (map[string]int64, error) {
	return nil, fmt.Errorf("SelectAsyncWorkflowAckLevels not implemented for MongoDB")
}

func (db *mdb) UpsertAsyncWorkflowAckLevel(ctx context.Context, shardID int, sourceCluster string, ackLevel int64, now time.Time) error {
	return fmt.Errorf("UpsertAsyncWorkflowAckLevel not implemented for MongoDB")
}

func (db *mdb) RangeDeleteAsyncWorkflowMessages(ctx context.Context, shardID int, sourceCluster string, inclusiveMaxMessageID int64) error {
	return fmt.Errorf("RangeDeleteAsyncWorkflowMessages not implemented for MongoDB")
}
