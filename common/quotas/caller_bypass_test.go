// Copyright (c) 2025 Uber Technologies, Inc.
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

package quotas

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

func TestCallerBypass_AllowLimiter(t *testing.T) {
	tests := []struct {
		name              string
		limiterAllows     bool
		bypassCallerTypes []interface{}
		callerType        types.CallerType
		expected          bool
	}{
		{
			name:              "Limiter allows - bypass not checked",
			limiterAllows:     true,
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeSDK,
			expected:          true,
		},
		{
			name:              "Limiter blocks but caller bypasses",
			limiterAllows:     false,
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeCLI,
			expected:          true,
		},
		{
			name:              "Limiter blocks and caller doesn't bypass",
			limiterAllows:     false,
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeSDK,
			expected:          false,
		},
		{
			name:              "Empty bypass list blocks",
			limiterAllows:     false,
			bypassCallerTypes: []interface{}{},
			callerType:        types.CallerTypeCLI,
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLimiter := NewMockLimiter(ctrl)
			mockLimiter.EXPECT().Allow().Return(tt.limiterAllows)

			bypassFn := func(...dynamicproperties.FilterOption) []interface{} {
				return tt.bypassCallerTypes
			}
			callerBypass := NewCallerBypass(bypassFn)

			ctx := types.ContextWithCallerInfo(context.Background(), types.NewCallerInfo(tt.callerType))
			result := callerBypass.AllowLimiter(ctx, mockLimiter)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCallerBypass_AllowPolicy(t *testing.T) {
	tests := []struct {
		name              string
		policyAllows      bool
		bypassCallerTypes []interface{}
		callerType        types.CallerType
		expected          bool
	}{
		{
			name:              "Policy allows - bypass not checked",
			policyAllows:      true,
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeSDK,
			expected:          true,
		},
		{
			name:              "Policy blocks but caller bypasses",
			policyAllows:      false,
			bypassCallerTypes: []interface{}{"ui", "cli"},
			callerType:        types.CallerTypeUI,
			expected:          true,
		},
		{
			name:              "Policy blocks and caller doesn't bypass",
			policyAllows:      false,
			bypassCallerTypes: []interface{}{"internal"},
			callerType:        types.CallerTypeSDK,
			expected:          false,
		},
		{
			name:              "Empty bypass list blocks",
			policyAllows:      false,
			bypassCallerTypes: []interface{}{},
			callerType:        types.CallerTypeCLI,
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPolicy := NewMockPolicy(ctrl)
			mockPolicy.EXPECT().Allow(Info{Domain: "test"}).Return(tt.policyAllows)

			bypassFn := func(...dynamicproperties.FilterOption) []interface{} {
				return tt.bypassCallerTypes
			}
			callerBypass := NewCallerBypass(bypassFn)

			ctx := types.ContextWithCallerInfo(context.Background(), types.NewCallerInfo(tt.callerType))
			result := callerBypass.AllowPolicy(ctx, mockPolicy, Info{Domain: "test"})

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCallerBypass_ShouldBypass(t *testing.T) {
	tests := []struct {
		name              string
		bypassCallerTypes []interface{}
		callerType        types.CallerType
		expected          bool
	}{
		{
			name:              "CLI bypasses when configured",
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeCLI,
			expected:          true,
		},
		{
			name:              "Multiple types in list",
			bypassCallerTypes: []interface{}{"cli", "ui", "internal"},
			callerType:        types.CallerTypeInternal,
			expected:          true,
		},
		{
			name:              "Type not in list doesn't bypass",
			bypassCallerTypes: []interface{}{"cli"},
			callerType:        types.CallerTypeSDK,
			expected:          false,
		},
		{
			name:              "Unknown type doesn't bypass",
			bypassCallerTypes: []interface{}{"cli", "ui"},
			callerType:        types.CallerTypeUnknown,
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bypassFn := func(...dynamicproperties.FilterOption) []interface{} {
				return tt.bypassCallerTypes
			}
			callerBypass := NewCallerBypass(bypassFn)

			ctx := types.ContextWithCallerInfo(context.Background(), types.NewCallerInfo(tt.callerType))
			result := callerBypass.ShouldBypass(ctx)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCallerBypass_NilBypassFunction(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	callerBypass := NewCallerBypass(nil)
	ctx := types.ContextWithCallerInfo(context.Background(), types.NewCallerInfo(types.CallerTypeCLI))

	if callerBypass.ShouldBypass(ctx) {
		t.Error("expected ShouldBypass to return false with nil bypass function")
	}

	mockLimiter := NewMockLimiter(ctrl)
	mockLimiter.EXPECT().Allow().Return(false)
	if callerBypass.AllowLimiter(ctx, mockLimiter) {
		t.Error("expected AllowLimiter to return false when limiter blocks and bypass function is nil")
	}
}
