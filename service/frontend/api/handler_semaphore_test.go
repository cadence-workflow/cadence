package api

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/frontend/validate"
)

func TestCreateSemaphore(t *testing.T) {
	req := &types.CreateSemaphoreRequest{
		Domain:        "domain",
		SemaphoreName: "semaphore",
		Capacity:      10,
	}
	resp := &types.CreateSemaphoreResponse{Semaphore: &types.Semaphore{
		SemaphoreName:  "semaphore",
		Capacity:       10,
		BucketCapacity: 100,
	}}
	testCases := []struct {
		name          string
		shuttingDown  bool
		setupMocks    func(*mockDeps)
		want          *types.CreateSemaphoreResponse
		expectedError error
	}{
		{
			name: "success",
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateCreateSemaphoreRequest(gomock.Any(), req).Return(nil)
				deps.mockSemaphoreHandler.EXPECT().CreateSemaphore(gomock.Any(), req).Return(resp, nil)
			},
			want: resp,
		},
		{
			name:          "shutting down",
			shuttingDown:  true,
			setupMocks:    func(deps *mockDeps) {},
			expectedError: validate.ErrShuttingDown,
		},
		{
			name: "validator error",
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateCreateSemaphoreRequest(gomock.Any(), req).Return(errors.New("validator error"))
			},
			expectedError: errors.New("validator error"),
		},
		{
			name: "semaphore handler error",
			setupMocks: func(deps *mockDeps) {
				deps.mockRequestValidator.EXPECT().ValidateCreateSemaphoreRequest(gomock.Any(), req).Return(nil)
				deps.mockSemaphoreHandler.EXPECT().CreateSemaphore(gomock.Any(), req).Return(nil, errors.New("handler error"))
			},
			expectedError: errors.New("handler error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wh, deps := setupMocksForWorkflowHandler(t)
			if tc.shuttingDown {
				wh.Stop()
			}
			tc.setupMocks(deps)

			got, err := wh.CreateSemaphore(context.Background(), req)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
