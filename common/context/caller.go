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

package context

import (
	"context"
)

type CallerType int

const (
	CallerTypeUnknown CallerType = iota
	CallerTypeCLI
	CallerTypeUI
	CallerTypeSDK
	CallerTypeService
)

type CallerInfo struct {
	Subject    string
	Name       string
	CallerType CallerType
	IsAdmin    bool
}

type contextKey string

const callerInfoKey = contextKey("caller.Info")

func WithCallerInfo(ctx context.Context, info *CallerInfo) context.Context {
	if info == nil {
		return ctx
	}
	return context.WithValue(ctx, callerInfoKey, info)
}

func GetCallerInfo(ctx context.Context) *CallerInfo {
	if ctx == nil {
		return nil
	}
	info, _ := ctx.Value(callerInfoKey).(*CallerInfo)
	return info
}

func (c CallerType) String() string {
	switch c {
	case CallerTypeCLI:
		return "cli"
	case CallerTypeUI:
		return "ui"
	case CallerTypeSDK:
		return "sdk"
	case CallerTypeService:
		return "service"
	default:
		return "unknown"
	}
}

func ParseCallerType(s string) CallerType {
	switch s {
	case "cli":
		return CallerTypeCLI
	case "ui":
		return CallerTypeUI
	case "sdk":
		return CallerTypeSDK
	case "service":
		return CallerTypeService
	default:
		return CallerTypeUnknown
	}
}
