// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package workercommon

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/cadence/client"

	"github.com/uber/cadence/common/backoff"
	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/resource"
)

func StartWorkflowWithRetry(
	workflowType string,
	startUpDelay time.Duration,
	resource resource.Resource,
	startWorkflow func(client client.Client) error,
) error {
	// let history / matching service warm up
	time.Sleep(startUpDelay)
	sdkClient := client.NewClient(
		resource.GetSDKClient(),
		constants.SystemLocalDomainName,
		nil, /* &client.Options{} */
	)
	policy := backoff.NewExponentialRetryPolicy(time.Second)
	policy.SetMaximumInterval(time.Minute)
	policy.SetExpirationInterval(backoff.NoInterval)
	throttleRetry := backoff.NewThrottleRetry(
		backoff.WithRetryPolicy(policy),
		backoff.WithRetryableError(func(_ error) bool { return true }),
	)
	err := throttleRetry.Do(context.Background(), func(ctx context.Context) error {
		return startWorkflow(sdkClient)
	})
	if err != nil {
		panic(fmt.Sprintf("unreachable: %#v", err))
	} else {
		resource.GetLogger().Info("starting workflow", tag.WorkflowType(workflowType))
	}
	return err
}
