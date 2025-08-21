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

package metered

import (
	"context"
	"errors"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/types"
)

func handleErr(err error, scope metrics.Scope, logger log.Logger) error {
	logger = logger.Helper()
	switch {
	case errors.As(err, new(*types.InternalServiceError)):
		scope.IncCounter(metrics.ShardDistributorFailures)
		logger.Error("Internal service error", tag.Error(err))
		return err
	case errors.As(err, new(*types.NamespaceNotFoundError)):
		scope.IncCounter(metrics.ShardDistributorErrNamespaceNotFound)
		return err
	case errors.As(err, new(*types.ShardNotFoundError)):
		scope.IncCounter(metrics.ShardDistributorErrShardNotFound)
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		logger.Error("request timeout", tag.Error(err))
		scope.IncCounter(metrics.ShardDistributorErrContextTimeoutCounter)
		return err
	}

	logger.Error("internal uncategorized error", tag.Error(err))
	scope.IncCounter(metrics.ShardDistributorFailures)
	return err
}
