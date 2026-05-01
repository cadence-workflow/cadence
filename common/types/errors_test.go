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

package types

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func Test_Error(t *testing.T) {
	errMessage := "test"
	tests := []struct {
		name string
		err  error
	}{
		{
			err: AccessDeniedError{
				Message: errMessage,
			},
		},
		{
			err: BadRequestError{
				Message: errMessage,
			},
		},
		{
			err: CancellationAlreadyRequestedError{
				Message: errMessage,
			},
		},
		{
			err: DomainAlreadyExistsError{
				Message: errMessage,
			},
		},
		{
			err: EntityNotExistsError{
				Message: errMessage,
			},
		},
		{
			err: InternalDataInconsistencyError{
				Message: errMessage,
			},
		},
		{
			err: WorkflowExecutionAlreadyCompletedError{
				Message: errMessage,
			},
		},
		{
			err: LimitExceededError{
				Message: errMessage,
			},
		},
		{
			err: QueryFailedError{
				Message: errMessage,
			},
		},
		{
			err: RemoteSyncMatchedError{
				Message: errMessage,
			},
		},
		{
			err: ServiceBusyError{
				Message: errMessage,
			},
		},
		{
			err: EventAlreadyStartedError{
				Message: errMessage,
			},
		},
		{
			err: StickyWorkerUnavailableError{
				Message: errMessage,
			},
		},
		{
			err: ReadOnlyPartitionError{
				Message: errMessage,
			},
		},
		{
			err: InternalServiceError{
				Message: errMessage,
			},
		},
	}
	for _, tt := range tests {
		t.Run(reflect.TypeOf(tt.err).String(), func(t *testing.T) {
			require.Equal(t, errMessage, tt.err.Error())
		})
	}
}

func Test_ClientVersionNotSupportedError(t *testing.T) {
	err := ClientVersionNotSupportedError{
		FeatureVersion:    "1.0",
		ClientImpl:        "1.0",
		SupportedVersions: "1.2",
	}
	require.Equal(t, "client version not supported", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}

func Test_FeatureNotEnabledError(t *testing.T) {
	err := FeatureNotEnabledError{FeatureFlag: "test"}
	require.Equal(t, "feature not enabled", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}

func Test_CurrentBranchChangedError(t *testing.T) {
	err := CurrentBranchChangedError{Message: "test", CurrentBranchToken: []byte{}}
	require.Equal(t, "test", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}

func Test_DomainNotActiveError(t *testing.T) {
	err := DomainNotActiveError{Message: "test", DomainName: "test-domain"}
	require.Equal(t, "test", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}

func Test_RetryTaskV2Error(t *testing.T) {
	testID := int64(1)
	testVersion := int64(1.0)
	err := RetryTaskV2Error{
		Message:           "test",
		DomainID:          "test-domain-id",
		WorkflowID:        "wid",
		RunID:             "rid",
		StartEventID:      &testID,
		StartEventVersion: &testVersion,
		EndEventID:        &testID,
		EndEventVersion:   &testVersion,
	}
	require.Equal(t, "test", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}

func Test_WorkflowExecutionAlreadyStartedError(t *testing.T) {
	err := WorkflowExecutionAlreadyStartedError{Message: "test"}
	require.Equal(t, "test", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}

func Test_ShardOwnershipLostError(t *testing.T) {
	err := ShardOwnershipLostError{Message: "test"}
	require.Equal(t, "test", err.Error())
	require.NoError(t, err.MarshalLogObject(zapcore.NewMapObjectEncoder()))
}
