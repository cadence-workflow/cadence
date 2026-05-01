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

package errorinjectors

import (
	"context"
	stdErrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/common/errors"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/types"
)

func TestInjector(t *testing.T) {
	// All wrappers follow the same logic, so in this test we only test one of them.
	t.Run("no error is forwarded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := admin.NewMockClient(ctrl)

		clientMock.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		injector := NewAdminClient(clientMock, 1, testlogger.New(t)).(*adminClient)

		injector.fakeErrFn = func(float64) error {
			return nil
		}
		injector.forwardCallFn = func(err error) bool {
			return err == nil
		}
		_, err := injector.DescribeWorkflowExecution(context.Background(), &types.AdminDescribeWorkflowExecutionRequest{})
		assert.NoError(t, err)
	})
	t.Run("no fake error, but client returns an error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := admin.NewMockClient(ctrl)

		clientErr := fmt.Errorf("fail")

		clientMock.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, clientErr).Times(1)

		injector := NewAdminClient(clientMock, 1, testlogger.New(t)).(*adminClient)

		injector.fakeErrFn = func(float64) error {
			return nil
		}
		injector.forwardCallFn = func(err error) bool {
			return err == nil
		}
		_, err := injector.DescribeWorkflowExecution(context.Background(), &types.AdminDescribeWorkflowExecutionRequest{})
		assert.Error(t, err)
		assert.True(t, stdErrors.Is(clientErr, err))
	})
	t.Run("fake error overrides client error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := admin.NewMockClient(ctrl)

		clientErr := fmt.Errorf("fail")

		clientMock.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, clientErr).Times(1)

		injector := NewAdminClient(clientMock, 1, testlogger.New(t)).(*adminClient)

		injector.fakeErrFn = func(float64) error {
			return errors.ErrFakeServiceBusy
		}
		injector.forwardCallFn = func(err error) bool {
			return true
		}
		_, err := injector.DescribeWorkflowExecution(context.Background(), &types.AdminDescribeWorkflowExecutionRequest{})
		assert.Error(t, err)
		assert.True(t, stdErrors.Is(errors.ErrFakeServiceBusy, err))
	})
	t.Run("failed but not forwarded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := admin.NewMockClient(ctrl)

		clientMock.EXPECT().DescribeWorkflowExecution(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		injector := NewAdminClient(clientMock, 1, testlogger.New(t)).(*adminClient)

		injector.fakeErrFn = func(float64) error {
			return errors.ErrFakeServiceBusy
		}
		injector.forwardCallFn = func(err error) bool {
			return true
		}
		_, err := injector.DescribeWorkflowExecution(context.Background(), &types.AdminDescribeWorkflowExecutionRequest{})
		assert.Error(t, err)
	})
	t.Run("failed and not forwarded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		clientMock := admin.NewMockClient(ctrl)

		injector := NewAdminClient(clientMock, 1, testlogger.New(t)).(*adminClient)

		injector.fakeErrFn = func(float64) error {
			return errors.ErrFakeServiceBusy
		}
		injector.forwardCallFn = func(err error) bool {
			return false
		}
		_, err := injector.DescribeWorkflowExecution(context.Background(), &types.AdminDescribeWorkflowExecutionRequest{})
		assert.Error(t, err)
	})
}
