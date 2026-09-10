package failure

type ErrorType string

const (
	CustomError                 ErrorType = "The failure is caused by a specific custom error returned from the service code"
	GenericError                ErrorType = "The failure is because of an error returned from the service code"
	PanicError                  ErrorType = "The failure is caused by a panic in the service code"
	TimeoutError                ErrorType = "The failure is caused by a timeout during the execution"
	HeartBeatBlobSizeLimit      ErrorType = "Heartbeat details has exceeded the blob size limit"
	ActivityOutputBlobSizeLimit ErrorType = "Activity output has exceeded the blob size limit"
	DecisionBlobSizeLimit       ErrorType = "Decision result caused to exceed blob size limit"
	HistorySizeExceedsLimit     ErrorType = "The workflow history size or event count exceeded the configured limit"
)

func (e ErrorType) String() string {
	return string(e)
}

type FailureType string

const (
	ActivityFailed        FailureType = "Activity Failed"
	WorkflowFailed        FailureType = "Workflow Failed"
	DecisionCausedFailure FailureType = "Decision caused failure"
)

func (f FailureType) String() string {
	return string(f)
}

type FailureIssuesMetadata struct {
	Identity            string
	ActivityType        string
	ActivityScheduledID int64
	ActivityStartedID   int64
	FailedEventID       int64 `json:",omitempty"`
}

// BlobSizeMetadata includes the details of blob size limits
type BlobSizeMetadata struct {
	BlobSizeWarnLimit  int32
	BlobSizeErrorLimit int32
}

type FailureRootcauseMetadata struct {
	BlobSizeMetadata *BlobSizeMetadata
}
