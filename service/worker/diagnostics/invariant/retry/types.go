package retry

import "github.com/uber/cadence/common/types"

type RetryType string

const (
	WorkflowRetryIssue     RetryType = "Workflow Retry configured but invalid"
	ActivityRetryIssue     RetryType = "Activity Retry configured but invalid"
	ActivityHeartbeatIssue RetryType = "Activity Heartbeat configured but invalid"
)

func (r RetryType) String() string {
	return string(r)
}

type IssueType string

const (
	RetryPolicyValidationMaxAttempts           IssueType = "MaximumAttempts set to 1 will not retry since maximum attempts includes the first attempt."
	RetryPolicyValidationExpInterval           IssueType = "ExpirationIntervalInSeconds less than  InitialIntervalInSeconds  will not retry."
	HeartBeatTimeoutEqualToStartToCloseTimeout IssueType = "Heartbeat timeout being equal or higher than StartToClose timeout will not provide any benefit."
)

func (i IssueType) String() string {
	return string(i)
}

type RetryMetadata struct {
	EventID     int64
	RetryPolicy *types.RetryPolicy
}
