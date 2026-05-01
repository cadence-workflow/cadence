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

package isolationgroup

import "context"

const (
	GroupKey         = "isolation-group"
	OriginalGroupKey = "original-isolation-group"
	WorkflowIDKey    = "wf-id"
)

type configKey struct{}

type isolationGroupKey struct{}

// ConfigFromContext retrieves infomation about the partition config of the context
// which is used for tasklist isolation
func ConfigFromContext(ctx context.Context) map[string]string {
	val, ok := ctx.Value(configKey{}).(map[string]string)
	if !ok {
		return nil
	}
	return val
}

// ContextWithConfig stores the partition config of tasklist isolation into the given context
func ContextWithConfig(ctx context.Context, partitionConfig map[string]string) context.Context {
	return context.WithValue(ctx, configKey{}, partitionConfig)
}

// IsolationGroupFromContext retrieves the isolation group from the given context,
// which is used to identify which isolation group the poller is from
func IsolationGroupFromContext(ctx context.Context) string {
	val, ok := ctx.Value(isolationGroupKey{}).(string)
	if !ok {
		return ""
	}
	return val
}

// ContextWithIsolationGroup stores the isolation group into the given context
func ContextWithIsolationGroup(ctx context.Context, isolationGroup string) context.Context {
	return context.WithValue(ctx, isolationGroupKey{}, isolationGroup)
}
