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

package blobstore

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/backoff"
)

func runCRUDTest(
	t *testing.T,
	retryPolicy backoff.RetryPolicy,
	retryableError bool,
	req, resp any,
	expectFn func(*MockClient, any, any),
	callFn func(Client, context.Context, any) (any, error),
	assertFn func(*testing.T, any, any, any, error),
) {
	mockClient := NewMockClient(gomock.NewController(t))
	throttleRetryOptions := []backoff.ThrottleRetryOption{
		backoff.WithRetryPolicy(retryPolicy),
	}
	if retryableError {
		throttleRetryOptions = append(throttleRetryOptions, backoff.WithRetryableError(mockClient.IsRetryableError))
	}
	client := &retryableClient{
		client:        mockClient,
		throttleRetry: backoff.NewThrottleRetry(throttleRetryOptions...),
	}

	expectFn(mockClient, req, resp)
	result, err := callFn(client, context.Background(), req)
	assertFn(t, req, resp, result, err)
}

func TestRetryableClient(t *testing.T) {
	tests := []struct {
		name           string
		retryPolicy    backoff.RetryPolicy
		retryableError bool
		req            any
		resp           any
		expectFn       func(*MockClient, any, any)
		callFn         func(Client, context.Context, any) (any, error)
		assertFn       func(*testing.T, any, any, any, error)
	}{
		{
			name:           "Put",
			retryPolicy:    backoff.NewExponentialRetryPolicy(0),
			retryableError: false,
			req:            &PutRequest{},
			resp:           &PutResponse{},
			expectFn: func(m *MockClient, req, resp any) {
				m.EXPECT().Put(gomock.Any(), req.(*PutRequest)).Return(resp.(*PutResponse), nil).Times(1)
			},
			callFn: func(c Client, ctx context.Context, req any) (any, error) {
				return c.Put(ctx, req.(*PutRequest))
			},
			assertFn: func(t *testing.T, req any, resp any, result any, err error) {
				assert.NoError(t, err)
				assert.Equal(t, resp.(*PutResponse), result)
			},
		},
		{
			name:           "Get",
			retryPolicy:    backoff.NewExponentialRetryPolicy(0),
			retryableError: false,
			req:            &GetRequest{},
			resp:           &GetResponse{},
			expectFn: func(m *MockClient, req, resp any) {
				m.EXPECT().Get(gomock.Any(), req.(*GetRequest)).Return(resp.(*GetResponse), nil).Times(1)
			},
			callFn: func(c Client, ctx context.Context, req any) (any, error) {
				return c.Get(ctx, req.(*GetRequest))
			},
			assertFn: func(t *testing.T, req any, resp any, result any, err error) {
				assert.NoError(t, err)
				assert.Equal(t, resp.(*GetResponse), result)
			},
		},
		{
			name:           "Exists",
			retryPolicy:    backoff.NewExponentialRetryPolicy(0),
			retryableError: false,
			req:            &ExistsRequest{},
			resp:           &ExistsResponse{},
			expectFn: func(m *MockClient, req, resp any) {
				m.EXPECT().Exists(gomock.Any(), req.(*ExistsRequest)).Return(resp.(*ExistsResponse), nil).Times(1)
			},
			callFn: func(c Client, ctx context.Context, req any) (any, error) {
				return c.Exists(ctx, req.(*ExistsRequest))
			},
			assertFn: func(t *testing.T, req any, resp any, result any, err error) {
				assert.NoError(t, err)
				assert.Equal(t, resp.(*ExistsResponse), result)
			},
		},
		{
			name:           "Delete",
			retryPolicy:    backoff.NewExponentialRetryPolicy(0),
			retryableError: false,
			req:            &DeleteRequest{},
			resp:           &DeleteResponse{},
			expectFn: func(m *MockClient, req, resp any) {
				m.EXPECT().Delete(gomock.Any(), req.(*DeleteRequest)).Return(resp.(*DeleteResponse), nil).Times(1)
			},
			callFn: func(c Client, ctx context.Context, req any) (any, error) {
				return c.Delete(ctx, req.(*DeleteRequest))
			},
			assertFn: func(t *testing.T, req any, resp any, result any, err error) {
				assert.NoError(t, err)
				assert.Equal(t, resp.(*DeleteResponse), result)
			},
		},
		{
			name:           "RetryOnError",
			retryPolicy:    backoff.NewExponentialRetryPolicy(1),
			retryableError: true,
			req:            &PutRequest{},
			resp:           &PutResponse{},
			expectFn: func(m *MockClient, req, resp any) {
				retryableError := errors.New("retryable error")
				m.EXPECT().Put(gomock.Any(), req.(*PutRequest)).Return(nil, retryableError).Times(1)
				m.EXPECT().IsRetryableError(retryableError).Return(true).Times(1)
				m.EXPECT().Put(gomock.Any(), req.(*PutRequest)).Return(resp, nil).Times(1)
			},
			callFn: func(c Client, ctx context.Context, req any) (any, error) {
				return c.Put(ctx, req.(*PutRequest))
			},
			assertFn: func(t *testing.T, req any, resp any, result any, err error) {
				assert.NoError(t, err, "Expected no error on successful retry")
				assert.Equal(t, resp.(*PutResponse), result, "Expected the response to match")
			},
		},
		{
			name:           "NotRetryOnError",
			retryPolicy:    backoff.NewExponentialRetryPolicy(0),
			retryableError: true,
			req:            &PutRequest{},
			expectFn: func(m *MockClient, req, resp any) {
				nonRetryableError := errors.New("non-retryable error")
				m.EXPECT().Put(gomock.Any(), req.(*PutRequest)).Return(nil, nonRetryableError).Times(1)
				m.EXPECT().IsRetryableError(nonRetryableError).Return(false).Times(1)
			},
			callFn: func(c Client, ctx context.Context, req any) (any, error) {
				return c.Put(ctx, req.(*PutRequest))
			},
			assertFn: func(t *testing.T, req any, resp any, result any, err error) {
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runCRUDTest(
				t,
				tc.retryPolicy,
				tc.retryableError,
				tc.req,
				tc.resp,
				tc.expectFn,
				tc.callFn,
				tc.assertFn,
			)
		})
	}
}

func TestRetryableClient_IsRetryableError(t *testing.T) {
	mockClient := NewMockClient(gomock.NewController(t))
	client := &retryableClient{
		client: mockClient,
		throttleRetry: backoff.NewThrottleRetry(
			backoff.WithRetryPolicy(backoff.NewExponentialRetryPolicy(0)),
			backoff.WithRetryableError(mockClient.IsRetryableError),
		),
	}

	retryableError := errors.New("retryable error")
	mockClient.EXPECT().IsRetryableError(retryableError).Return(true).Times(1)

	isRetryableError := client.IsRetryableError(retryableError)
	assert.True(t, isRetryableError, "Expected error to be retryable")
}
