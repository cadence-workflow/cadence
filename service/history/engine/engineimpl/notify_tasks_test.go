package engineimpl

import (
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	hcommon "github.com/uber/cadence/service/history/common"
	"github.com/uber/cadence/service/history/queue"
	"github.com/uber/cadence/service/history/shard"
)

func TestNotifyNewVisibilityTasks(t *testing.T) {
	info := &hcommon.NotifyTaskInfo{
		Tasks: []persistence.Task{
			&persistence.RecordWorkflowStartedTask{
				TaskData: persistence.TaskData{
					TaskID:  1,
					Version: cluster.TestCurrentClusterInitialFailoverVersion,
				},
			},
		},
	}

	tests := map[string]struct {
		info                *hcommon.NotifyTaskInfo
		registerProcessor   bool
		wantClusterLookup   bool
		wantProcessorNotify bool
	}{
		"notifies the visibility processor": {
			info:                info,
			registerProcessor:   true,
			wantClusterLookup:   true,
			wantProcessorNotify: true,
		},
		"no tasks is a no-op": {
			info:              &hcommon.NotifyTaskInfo{},
			registerProcessor: true,
		},
		"missing visibility processor does not panic": {
			info:              info,
			wantClusterLookup: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockShard := shard.NewMockContext(ctrl)
			if tc.wantClusterLookup {
				mockShard.EXPECT().GetClusterMetadata().Return(cluster.TestActiveClusterMetadata).Times(1)
			}
			queueProcessors := map[persistence.HistoryTaskCategory]queue.Processor{}
			if tc.registerProcessor {
				mockProcessor := queue.NewMockProcessor(ctrl)
				if tc.wantProcessorNotify {
					mockProcessor.EXPECT().NotifyNewTask(cluster.TestCurrentClusterName, tc.info).Times(1)
				}
				queueProcessors[persistence.HistoryTaskCategoryVisibility] = mockProcessor
			}
			e := &historyEngineImpl{
				shard:           mockShard,
				logger:          testlogger.New(t),
				queueProcessors: queueProcessors,
			}

			e.NotifyNewVisibilityTasks(tc.info)
		})
	}
}
