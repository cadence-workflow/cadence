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

package types

import (
	"context"
)

type CallerType int

const (
	CallerTypeUnknown CallerType = iota
	CallerTypeCLI
	CallerTypeUI
	CallerTypeSDK
	CallerTypeInternal
)

// CallerInfo captures request source information for observability and resource management.
//
// Intent:
//   - Track the source/origin/actor of API requests (CLI, UI, SDK, internal service calls)
//   - Enable client-specific behavior and resource allocation decisions
//   - Support future extensibility for additional caller metadata (e.g., identity, version)
//
// Consumers:
//   - Logging and audit systems for request attribution
//   - Metrics and monitoring for client-specific observability
//   - Rate limiting and resource management based on caller information
//
// Lifecycle:
//   - Should be set early in request processing, typically after authentication
//   - Expected for external API calls (CLI, UI, SDK)
//   - May be absent for internal service-to-service calls or unauthenticated endpoints
//   - Set by authentication/authorization middleware or API gateway components
type CallerInfo struct {
	callerType CallerType
}

// NewCallerInfo creates a new CallerInfo
func NewCallerInfo(callerType CallerType) *CallerInfo {
	return &CallerInfo{callerType: callerType}
}

// GetCallerType returns the CallerType, or CallerTypeUnknown if CallerInfo is nil
func (c *CallerInfo) GetCallerType() CallerType {
	if c == nil {
		return CallerTypeUnknown
	}
	return c.callerType
}

type callerInfoContextKey string

const callerInfoKey = callerInfoContextKey("caller-info")

func (c CallerType) String() string {
	switch c {
	case CallerTypeCLI:
		return "cli"
	case CallerTypeUI:
		return "ui"
	case CallerTypeSDK:
		return "sdk"
	case CallerTypeInternal:
		return "internal"
	default:
		return "unknown"
	}
}

// ParseCallerType converts a string to CallerType
func ParseCallerType(s string) CallerType {
	switch s {
	case "cli":
		return CallerTypeCLI
	case "ui":
		return CallerTypeUI
	case "sdk":
		return CallerTypeSDK
	case "internal":
		return CallerTypeInternal
	default:
		return CallerTypeUnknown
	}
}

// ContextWithCallerInfo adds CallerInfo to context
func ContextWithCallerInfo(ctx context.Context, callerInfo *CallerInfo) context.Context {
	if callerInfo == nil {
		return ctx
	}
	return context.WithValue(ctx, callerInfoKey, callerInfo)
}

// CallerInfoFromContext retrieves CallerInfo from context, returns nil if not set
func CallerInfoFromContext(ctx context.Context) *CallerInfo {
	if ctx == nil {
		return nil
	}
	callerInfo, _ := ctx.Value(callerInfoKey).(*CallerInfo)
	return callerInfo
}
