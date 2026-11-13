package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/dynamicconfig"
)

func TestAsyncWorkflowQueueConfigDefaults(t *testing.T) {
	cfg := New(dynamicconfig.NewNopCollection(), 16, 1024*1024, false, "test-host")

	tests := []struct {
		name string
		got  func() interface{}
		want interface{}
	}{
		{"AsyncWorkflowQueueConsumerEnabled", func() interface{} { return cfg.AsyncWorkflowQueueConsumerEnabled(3) }, false},
		{"AsyncWorkflowQueuePollInterval", func() interface{} { return cfg.AsyncWorkflowQueuePollInterval(3) }, time.Second},
		{"AsyncWorkflowQueueCommitInterval", func() interface{} { return cfg.AsyncWorkflowQueueCommitInterval(3) }, 5 * time.Second},
		{"AsyncWorkflowQueueGCInterval", func() interface{} { return cfg.AsyncWorkflowQueueGCInterval(3) }, time.Minute},
		{"AsyncWorkflowQueuePageSize", func() interface{} { return cfg.AsyncWorkflowQueuePageSize(3) }, 100},
		{"AsyncWorkflowQueueWorkerCount", func() interface{} { return cfg.AsyncWorkflowQueueWorkerCount() }, 100},
		{"AsyncWorkflowQueueSchedulerQueueSize", func() interface{} { return cfg.AsyncWorkflowQueueSchedulerQueueSize() }, 10000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.got())
		})
	}
}
