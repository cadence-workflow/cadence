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
}

// BlobSizeMetadata includes the details of blob size limits
type BlobSizeMetadata struct {
	BlobSizeWarnLimit  int32
	BlobSizeErrorLimit int32
}

type FailureRootcauseMetadata struct {
	BlobSizeMetadata *BlobSizeMetadata
}
