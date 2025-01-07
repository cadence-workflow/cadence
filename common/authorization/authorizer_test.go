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

package authorization

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

func Test_validatePermission(t *testing.T) {

	readRequestAttr := &Attributes{Permission: PermissionRead}
	writeRequestAttr := &Attributes{Permission: PermissionWrite}

	readWriteDomainData := domainData{
		common.DomainDataKeyForReadGroups:  "read1",
		common.DomainDataKeyForWriteGroups: "write1",
	}

	readDomainData := domainData{
		common.DomainDataKeyForReadGroups: "read1",
	}

	emptyDomainData := domainData{}

	tests := []struct {
		name       string
		claims     *JWTClaims
		attributes *Attributes
		data       map[string]string
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "no args should always fail",
			claims:     &JWTClaims{},
			attributes: &Attributes{},
			data:       emptyDomainData,
			wantErr:    assert.Error,
		},
		{
			name:       "Empty claims will be denied even when domain data has no groups",
			claims:     &JWTClaims{},
			attributes: writeRequestAttr,
			data:       emptyDomainData,
			wantErr:    assert.Error,
		},
		{
			name:       "Empty claims will be denied when domain data has at least one group",
			claims:     &JWTClaims{},
			attributes: writeRequestAttr,
			data:       readDomainData,
			wantErr:    assert.Error,
		},
		{
			name:       "Read-only groups should not get access to write groups",
			claims:     &JWTClaims{Groups: "read1"},
			attributes: writeRequestAttr,
			data:       readWriteDomainData,
			wantErr:    assert.Error,
		},
		{
			name:       "Write-only groups should get access to read groups",
			claims:     &JWTClaims{Groups: "write1"},
			attributes: readRequestAttr,
			data:       readWriteDomainData,
			wantErr:    assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validatePermission(tt.claims, tt.attributes, tt.data))
		})
	}
}

func TestSignalWithStartWorkflowExecutionRequestSerializeForLogging(t *testing.T) {
	tests := map[string]struct {
		input               interface{}
		expectedOutput      string
		expectedErrorOutput error
	}{
		"complete request without error": {
			input:          createNewSignalWithStartWorkflowExecutionRequest(),
			expectedOutput: "{\"domain\":\"testDomain\",\"workflowId\":\"testWorkflowID\",\"workflowType\":{\"name\":\"testWorkflowType\"},\"taskList\":{\"name\":\"testTaskList\",\"kind\":\"STICKY\"},\"executionStartToCloseTimeoutSeconds\":1,\"taskStartToCloseTimeoutSeconds\":1,\"identity\":\"testIdentity\",\"requestId\":\"DF66E35D-A5B0-425D-8731-6AAC4A4B6368\",\"workflowIdReusePolicy\":\"AllowDuplicate\",\"signalName\":\"testRequest\",\"control\":\"dGVzdENvbnRyb2w=\",\"retryPolicy\":{\"initialIntervalInSeconds\":1,\"backoffCoefficient\":1,\"maximumIntervalInSeconds\":1,\"maximumAttempts\":1,\"nonRetriableErrorReasons\":[\"testArray\"],\"expirationIntervalInSeconds\":1},\"cronSchedule\":\"testSchedule\",\"header\":{},\"delayStartSeconds\":1,\"jitterStartSeconds\":1,\"firstRunAtTimestamp\":1}",
		},

		"non marchalable struct should error": {
			input:               make(chan struct{}),
			expectedErrorOutput: &json.UnsupportedTypeError{},
		},

		"empty request without error": {
			input:          &types.SignalWithStartWorkflowExecutionRequest{},
			expectedOutput: "{}",
		},

		"typed nil request without error": {
			input:          (*types.SignalWithStartWorkflowExecutionRequest)(nil),
			expectedOutput: "",
		},

		"nil request without error": {
			input:          nil,
			expectedOutput: "",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				wrappedInput := NewFilteredRequestBody(test.input)
				output, err := wrappedInput.SerializeForLogging()
				assert.Equal(t, test.expectedOutput, output)
				if test.expectedErrorOutput != nil {
					assert.ErrorAs(t, err, &test.expectedErrorOutput)
				} else {
					assert.NoError(t, err)
				}

				assert.NotContains(t, output, "PII")
			})
		})
	}
}

func createNewSignalWithStartWorkflowExecutionRequest() *types.SignalWithStartWorkflowExecutionRequest {
	testTasklistKind := types.TaskListKind(1)
	testExecutionStartToCloseTimeoutSeconds := int32(1)
	testTaskStartToCloseTimeoutSeconds := int32(1)
	testWorkflowIDReusePolicy := types.WorkflowIDReusePolicy(1)
	testDelayStartSeconds := int32(1)
	testJitterStartSeconds := int32(1)
	testFirstRunAtTimestamp := int64(1)
	piiTestArray := []byte("testInputPII")
	piiTestMap := make(map[string][]byte)
	piiTestMap["PII"] = piiTestArray

	testReq := &types.SignalWithStartWorkflowExecutionRequest{
		Domain:       "testDomain",
		WorkflowID:   "testWorkflowID",
		WorkflowType: &types.WorkflowType{Name: "testWorkflowType"},
		TaskList: &types.TaskList{
			Name: "testTaskList",
			Kind: &testTasklistKind,
		},
		Input:                               piiTestArray,
		ExecutionStartToCloseTimeoutSeconds: &testExecutionStartToCloseTimeoutSeconds,
		TaskStartToCloseTimeoutSeconds:      &testTaskStartToCloseTimeoutSeconds,
		Identity:                            "testIdentity",
		RequestID:                           "DF66E35D-A5B0-425D-8731-6AAC4A4B6368",
		WorkflowIDReusePolicy:               &testWorkflowIDReusePolicy,
		SignalName:                          "testRequest",
		SignalInput:                         piiTestArray,
		Control:                             []byte("testControl"),
		RetryPolicy: &types.RetryPolicy{
			InitialIntervalInSeconds:    1,
			BackoffCoefficient:          1,
			MaximumIntervalInSeconds:    1,
			MaximumAttempts:             1,
			NonRetriableErrorReasons:    []string{"testArray"},
			ExpirationIntervalInSeconds: 1,
		},
		CronSchedule:        "testSchedule",
		Memo:                &types.Memo{Fields: piiTestMap},
		SearchAttributes:    &types.SearchAttributes{IndexedFields: piiTestMap},
		Header:              &types.Header{Fields: map[string][]byte{}},
		DelayStartSeconds:   &testDelayStartSeconds,
		JitterStartSeconds:  &testJitterStartSeconds,
		FirstRunAtTimestamp: &testFirstRunAtTimestamp,
	}
	return testReq
}
