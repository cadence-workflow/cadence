package semaphore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/clock"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
)

// Tests when taskGC deletes, after a delete up to lastDeletedLevel.
func TestTaskGC(t *testing.T) {
	const lastDeletedLevel = 10
	testCases := []struct {
		name                 string
		timeSinceLastDelete  time.Duration
		ackLevel             int64
		runNow               bool
		deleteErr            error
		expectDelete         bool
		expectedDeletedLevel int64
	}{
		{
			name:                 "nothing finished since the last delete, nothing deleted",
			timeSinceLastDelete:  time.Hour,
			ackLevel:             lastDeletedLevel,
			runNow:               true,
			expectedDeletedLevel: lastDeletedLevel,
		},
		{
			name:                 "too soon after the last delete, nothing deleted",
			timeSinceLastDelete:  maxTimeBetweenTaskDeletes / 2,
			ackLevel:             20,
			expectedDeletedLevel: lastDeletedLevel,
		},
		{
			name:                 "waited long enough, deleted",
			timeSinceLastDelete:  2 * maxTimeBetweenTaskDeletes,
			ackLevel:             20,
			expectDelete:         true,
			expectedDeletedLevel: 20,
		},
		{
			name:                 "runNow, too soon, deleted",
			ackLevel:             20,
			runNow:               true,
			expectDelete:         true,
			expectedDeletedLevel: 20,
		},
		{
			name:                 "delete fails, level stays",
			ackLevel:             20,
			runNow:               true,
			deleteErr:            assert.AnError,
			expectDelete:         true,
			expectedDeletedLevel: lastDeletedLevel,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := persistence.NewMockSemaphoreTaskManager(gomock.NewController(t))
			timeSource := clock.NewMockedTimeSource()
			gc := newTaskGC(newTaskDB(testBucketID, store, testlogger.New(t)), timeSource)
			gc.ackLevel = lastDeletedLevel
			gc.lastDeleteTime = timeSource.Now().Add(-tc.timeSinceLastDelete)
			if tc.expectDelete {
				store.EXPECT().RangeCompleteSemaphoreTasks(gomock.Any(), &persistence.RangeCompleteSemaphoreTasksRequest{
					DomainID:      testBucketID.DomainID,
					SemaphoreName: testBucketID.SemaphoreName,
					Bucket:        testBucketID.Bucket,
					AckLevel:      tc.ackLevel,
				}).Times(1).Return(&persistence.RangeCompleteSemaphoreTasksResponse{}, tc.deleteErr)
			}

			if tc.runNow {
				gc.runNow(tc.ackLevel)
			} else {
				gc.run(tc.ackLevel)
			}
			assert.Equal(t, tc.expectedDeletedLevel, gc.ackLevel)
			assert.Equal(t, tc.expectDelete, gc.lastDeleteTime.Equal(timeSource.Now()), "a delete, failed or not, starts the wait")
		})
	}
}
