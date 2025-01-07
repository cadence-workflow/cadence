// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

package invariant

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	c2 "github.com/uber/cadence/common"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/mocks"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
)

type InactiveInactiveDomainExistsSuite struct {
	*require.Assertions
	suite.Suite
}

func TestInactiveInactiveDomainExistsSuite(t *testing.T) {
	suite.Run(t, new(InactiveInactiveDomainExistsSuite))
}

func (s *InactiveInactiveDomainExistsSuite) SetupTest() {
	s.Assertions = require.New(s.T())
}

func (s *InactiveInactiveDomainExistsSuite) TestCheck() {
	testCases := []struct {
		getExecErr     error
		getExecResp    *persistence.GetWorkflowExecutionResponse
		getDomainErr   error
		expectedResult CheckResult
	}{
		{
			getExecErr: &types.EntityNotExistsError{},
			expectedResult: CheckResult{
				CheckResultType: CheckResultTypeHealthy,
				InvariantName:   InactiveDomainExists,
				Info:            "workflow's domain is active",
				InfoDetails:     "workflow's domain is active",
			},
		},
	}

	ctrl := gomock.NewController(s.T())
	domainCache := cache.NewMockDomainCache(ctrl)
	for _, tc := range testCases {
		execManager := &mocks.ExecutionManager{}
		execManager.On("GetWorkflowExecution", mock.Anything, mock.Anything).Return(tc.getExecResp, tc.getExecErr)
		domainCache.EXPECT().GetDomainName(gomock.Any()).Return("test-domain-name", nil).AnyTimes()
		domainCache.EXPECT().GetDomainByID(gomock.Any()).Return(constants.TestGlobalDomainEntry, nil).AnyTimes()
		i := NewInactiveDomainExists(persistence.NewPersistenceRetryer(execManager, nil, c2.CreatePersistenceRetryPolicy()), domainCache)
		result := i.Check(context.Background(), getOpenConcreteExecution())
		s.Equal(tc.expectedResult, result)

	}
}
