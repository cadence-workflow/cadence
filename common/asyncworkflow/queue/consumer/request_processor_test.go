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

package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/frontend"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func newTestRequestProcessor(t *testing.T, frontendClient frontend.Client) *RequestProcessor {
	return NewRequestProcessor(frontendClient, time.Second, testlogger.New(t))
}

func TestDecodeEnvelope(t *testing.T) {
	p := newTestRequestProcessor(t, nil)

	t.Run("corrupt envelope", func(t *testing.T) {
		_, err := p.DecodeEnvelope([]byte("not-thriftrw"))
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrCorruptMessage)
	})

	t.Run("valid envelope", func(t *testing.T) {
		env, err := p.DecodeEnvelope(mustGenerateStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, true))
		require.NoError(t, err)
		assert.Equal(t, "StartWorkflowExecutionAsyncRequest", env.GetType().String())
	})
}

func TestPrepareCorruptCases(t *testing.T) {
	p := newTestRequestProcessor(t, nil)

	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name:    "unsupported request type",
			payload: mustGenerateUnsupportedRequestMsg(t),
		},
		{
			name:    "start workflow with unsupported encoding",
			payload: mustGenerateStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeJSON, true),
		},
		{
			name:    "start workflow with invalid inner payload",
			payload: mustGenerateStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, false),
		},
		{
			name:    "signal with start with unsupported encoding",
			payload: mustGenerateSignalWithStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeJSON, true),
		},
		{
			name:    "signal with start with invalid inner payload",
			payload: mustGenerateSignalWithStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, false),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env, err := p.DecodeEnvelope(tc.payload)
			require.NoError(t, err)

			_, err = p.Prepare(env)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrCorruptMessage)
		})
	}
}

func TestPrepareAndInvoke(t *testing.T) {
	tests := []struct {
		name        string
		payload     func(t *testing.T) []byte
		mock        func(m *frontend.MockClient)
		wantType    string
		wantRunID   string
		wantErr     bool
		wantErrText string
	}{
		{
			name: "start workflow success",
			payload: func(t *testing.T) []byte {
				return mustGenerateStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, true)
			},
			mock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "run-1"}, nil).Times(1)
			},
			wantType:  "StartWorkflowExecutionAsyncRequest",
			wantRunID: "run-1",
		},
		{
			name: "start workflow already started is success",
			payload: func(t *testing.T) []byte {
				return mustGenerateStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, true)
			},
			mock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{RunID: "existing-run"}).Times(1)
			},
			wantType:  "StartWorkflowExecutionAsyncRequest",
			wantRunID: "existing-run",
		},
		{
			name: "start workflow frontend error",
			payload: func(t *testing.T) []byte {
				return mustGenerateStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, true)
			},
			mock: func(m *frontend.MockClient) {
				m.EXPECT().StartWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &types.InternalServiceError{Message: "boom"}).Times(1)
			},
			wantType: "StartWorkflowExecutionAsyncRequest",
			wantErr:  true,
		},
		{
			name: "signal with start success",
			payload: func(t *testing.T) []byte {
				return mustGenerateSignalWithStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, true)
			},
			mock: func(m *frontend.MockClient) {
				m.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&types.StartWorkflowExecutionResponse{RunID: "run-2"}, nil).Times(1)
			},
			wantType:  "SignalWithStartWorkflowExecutionAsyncRequest",
			wantRunID: "run-2",
		},
		{
			name: "signal with start already started is success",
			payload: func(t *testing.T) []byte {
				return mustGenerateSignalWithStartWorkflowExecutionRequestMsg(t, constants.EncodingTypeThriftRW, true)
			},
			mock: func(m *frontend.MockClient) {
				m.EXPECT().SignalWithStartWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, &types.WorkflowExecutionAlreadyStartedError{RunID: "existing-run-2"}).Times(1)
			},
			wantType:  "SignalWithStartWorkflowExecutionAsyncRequest",
			wantRunID: "existing-run-2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockFrontend := frontend.NewMockClient(ctrl)
			tc.mock(mockFrontend)
			p := newTestRequestProcessor(t, mockFrontend)

			env, err := p.DecodeEnvelope(tc.payload(t))
			require.NoError(t, err)

			prepared, err := p.Prepare(env)
			require.NoError(t, err)
			assert.Equal(t, tc.wantType, prepared.RequestType)
			assert.Equal(t, "test-domain", prepared.Domain)
			assert.Equal(t, "test-workflow-id", prepared.WorkflowID)

			runID, err := prepared.Invoke(context.Background())
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantRunID, runID)
		})
	}
}
