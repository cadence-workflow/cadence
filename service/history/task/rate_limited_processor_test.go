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

package task

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
)

type rateLimitedProcessorMockDeps struct {
	mockProcessor   *MockProcessor
	mockRateLimiter *MockRateLimiter
}

func setupMocksForRateLimitedProcessor(t *testing.T) (*rateLimitedProcessor, *rateLimitedProcessorMockDeps) {
	ctrl := gomock.NewController(t)

	deps := &rateLimitedProcessorMockDeps{
		mockProcessor:   NewMockProcessor(ctrl),
		mockRateLimiter: NewMockRateLimiter(ctrl),
	}

	processor := NewRateLimitedProcessor(deps.mockProcessor, deps.mockRateLimiter)
	rp, ok := processor.(*rateLimitedProcessor)
	require.True(t, ok)
	return rp, deps
}

func TestRateLimitedProcessorLifecycle(t *testing.T) {
	rp, deps := setupMocksForRateLimitedProcessor(t)

	deps.mockProcessor.EXPECT().Start().Times(1)
	rp.Start()

	deps.mockProcessor.EXPECT().Stop().Times(1)
	rp.Stop()
}

func TestRateLimitedProcessorSubmit(t *testing.T) {
	testCases := []struct {
		name          string
		task          Task
		setupMocks    func(*rateLimitedProcessorMockDeps)
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			task: &noopTask{},
			setupMocks: func(deps *rateLimitedProcessorMockDeps) {
				deps.mockRateLimiter.EXPECT().Wait(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				deps.mockProcessor.EXPECT().Submit(gomock.Any()).Return(nil).Times(1)
			},
		},
		{
			name: "rate limiter error",
			task: &noopTask{},
			setupMocks: func(deps *rateLimitedProcessorMockDeps) {
				deps.mockRateLimiter.EXPECT().Wait(gomock.Any(), gomock.Any()).Return(errors.New("rate limited")).Times(1)
			},
			expectError:   true,
			expectedError: "rate limited",
		},
		{
			name: "processor error",
			task: &noopTask{},
			setupMocks: func(deps *rateLimitedProcessorMockDeps) {
				deps.mockRateLimiter.EXPECT().Wait(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				deps.mockProcessor.EXPECT().Submit(gomock.Any()).Return(errors.New("processor error")).Times(1)
			},
			expectError:   true,
			expectedError: "processor error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp, deps := setupMocksForRateLimitedProcessor(t)
			tc.setupMocks(deps)

			err := rp.Submit(tc.task)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRateLimitedProcessorTrySubmit(t *testing.T) {
	testCases := []struct {
		name          string
		task          Task
		setupMocks    func(*rateLimitedProcessorMockDeps)
		expected      bool
		expectError   bool
		expectedError string
	}{
		{
			name: "success",
			task: &noopTask{},
			setupMocks: func(deps *rateLimitedProcessorMockDeps) {
				deps.mockRateLimiter.EXPECT().Allow(gomock.Any()).Return(true)
				deps.mockProcessor.EXPECT().TrySubmit(gomock.Any()).Return(true, nil)
			},
			expected: true,
		},
		{
			name: "rate limited",
			task: &noopTask{},
			setupMocks: func(deps *rateLimitedProcessorMockDeps) {
				deps.mockRateLimiter.EXPECT().Allow(gomock.Any()).Return(false)
			},
			expected: false,
		},
		{
			name: "error",
			task: &noopTask{},
			setupMocks: func(deps *rateLimitedProcessorMockDeps) {
				deps.mockRateLimiter.EXPECT().Allow(gomock.Any()).Return(true)
				deps.mockProcessor.EXPECT().TrySubmit(gomock.Any()).Return(false, errors.New("submit error"))
			},
			expected:      false,
			expectError:   true,
			expectedError: "submit error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rp, deps := setupMocksForRateLimitedProcessor(t)
			tc.setupMocks(deps)

			submitted, err := rp.TrySubmit(tc.task)
			if tc.expectError {
				assert.ErrorContains(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, submitted)
			}
		})
	}
}
