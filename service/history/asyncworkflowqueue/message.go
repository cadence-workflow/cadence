package asyncworkflowqueue

import (
	"errors"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

// Request is a decoded async workflow queue row.
type Request struct {
	Message         *persistence.AsyncWorkflowMessage
	DomainID        string
	Start           *types.StartWorkflowExecutionRequest           // set iff RequestType == StartWorkflow
	SignalWithStart *types.SignalWithStartWorkflowExecutionRequest // set iff RequestType == SignalWithStart
	ActiveCluster   string                                         // set by processorWrapper before delegating to the not-active processor
	Headers         map[string][]byte                              // yarpc headers captured by the frontend at accept; nil when absent
}

// ErrCorruptMessage marks a queue row that can never be decoded, regardless of retries.
var ErrCorruptMessage = errors.New("async workflow queue message is corrupt")
