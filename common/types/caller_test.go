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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallerType_String(t *testing.T) {
	tests := []struct {
		name       string
		callerType CallerType
		want       string
	}{
		{"CLI", CallerTypeCLI, "cli"},
		{"UI", CallerTypeUI, "ui"},
		{"SDK", CallerTypeSDK, "sdk"},
		{"Internal", CallerTypeInternal, "internal"},
		{"Unknown", CallerTypeUnknown, "unknown"},
		{"Empty string", CallerType(""), ""},
		{"Invalid", CallerType("invalid"), "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.callerType.String())
		})
	}
}

func TestParseCallerType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  CallerType
	}{
		{"cli", "cli", CallerTypeCLI},
		{"ui", "ui", CallerTypeUI},
		{"sdk", "sdk", CallerTypeSDK},
		{"internal", "internal", CallerTypeInternal},
		{"unknown", "unknown", CallerTypeUnknown},
		{"empty", "", CallerTypeUnknown},
		{"custom value", "my-custom-tool", CallerType("my-custom-tool")},
		{"uppercase", "CLI", CallerType("CLI")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ParseCallerType(tt.input))
		})
	}
}

func TestCallerTypeRoundTrip(t *testing.T) {
	tests := []CallerType{
		CallerTypeCLI,
		CallerTypeUI,
		CallerTypeSDK,
		CallerTypeInternal,
		CallerTypeUnknown,
	}

	for _, ct := range tests {
		t.Run(ct.String(), func(t *testing.T) {
			str := ct.String()
			parsed := ParseCallerType(str)
			assert.Equal(t, ct, parsed)
		})
	}
}

func TestNewCallerInfo(t *testing.T) {
	tests := []struct {
		name       string
		callerType CallerType
		want       CallerType
	}{
		{
			name:       "CLI",
			callerType: CallerTypeCLI,
			want:       CallerTypeCLI,
		},
		{
			name:       "SDK",
			callerType: CallerTypeSDK,
			want:       CallerTypeSDK,
		},
		{
			name:       "Unknown",
			callerType: CallerTypeUnknown,
			want:       CallerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := NewCallerInfo(tt.callerType)
			assert.Equal(t, tt.want, info.GetCallerType())
		})
	}
}

func TestCallerInfo_GetCallerType(t *testing.T) {
	tests := []struct {
		name string
		info CallerInfo
		want CallerType
	}{
		{
			name: "CLI CallerInfo",
			info: NewCallerInfo(CallerTypeCLI),
			want: CallerTypeCLI,
		},
		{
			name: "SDK CallerInfo",
			info: NewCallerInfo(CallerTypeSDK),
			want: CallerTypeSDK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.info.GetCallerType())
		})
	}
}

func TestContextWithCallerInfo(t *testing.T) {
	tests := []struct {
		name       string
		callerType CallerType
		want       CallerType
	}{
		{
			name:       "CLI caller info",
			callerType: CallerTypeCLI,
			want:       CallerTypeCLI,
		},
		{
			name:       "SDK caller info",
			callerType: CallerTypeSDK,
			want:       CallerTypeSDK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = ContextWithCallerInfo(ctx, NewCallerInfo(tt.callerType))

			got := GetCallerInfoFromContext(ctx)
			assert.Equal(t, tt.want, got.GetCallerType())
		})
	}
}

func TestGetCallerInfoFromContext(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		wantCaller CallerType
	}{
		{
			name:       "nil context",
			ctx:        nil,
			wantCaller: CallerTypeUnknown,
		},
		{
			name:       "context without caller info",
			ctx:        context.Background(),
			wantCaller: CallerTypeUnknown,
		},
		{
			name:       "context with CLI caller info",
			ctx:        ContextWithCallerInfo(context.Background(), NewCallerInfo(CallerTypeCLI)),
			wantCaller: CallerTypeCLI,
		},
		{
			name:       "context with SDK caller info",
			ctx:        ContextWithCallerInfo(context.Background(), NewCallerInfo(CallerTypeSDK)),
			wantCaller: CallerTypeSDK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCallerInfoFromContext(tt.ctx)
			assert.Equal(t, tt.wantCaller, got.GetCallerType())
		})
	}
}

type mockHeaders map[string]string

func (m mockHeaders) Get(key string) (string, bool) {
	val, ok := m[key]
	return val, ok
}

func TestNewCallerInfoFromTransportHeaders(t *testing.T) {
	tests := []struct {
		name       string
		headers    mockHeaders
		wantCaller CallerType
	}{
		{
			name:       "CLI caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: "cli"},
			wantCaller: CallerTypeCLI,
		},
		{
			name:       "UI caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: "ui"},
			wantCaller: CallerTypeUI,
		},
		{
			name:       "SDK caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: "sdk"},
			wantCaller: CallerTypeSDK,
		},
		{
			name:       "Internal caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: "internal"},
			wantCaller: CallerTypeInternal,
		},
		{
			name:       "empty caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: ""},
			wantCaller: CallerTypeUnknown,
		},
		{
			name:       "missing caller-type header",
			headers:    mockHeaders{},
			wantCaller: CallerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCallerInfoFromTransportHeaders(tt.headers)
			assert.Equal(t, tt.wantCaller, got.GetCallerType())
		})
	}
}

func TestGetContextWithCallerInfoFromHeaders(t *testing.T) {
	tests := []struct {
		name       string
		headers    mockHeaders
		wantCaller CallerType
	}{
		{
			name:       "CLI caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: "cli"},
			wantCaller: CallerTypeCLI,
		},
		{
			name:       "SDK caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: "sdk"},
			wantCaller: CallerTypeSDK,
		},
		{
			name:       "empty caller-type header",
			headers:    mockHeaders{CallerTypeHeaderName: ""},
			wantCaller: CallerTypeUnknown,
		},
		{
			name:       "missing caller-type header",
			headers:    mockHeaders{},
			wantCaller: CallerTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultCtx := GetContextWithCallerInfoFromHeaders(context.Background(), tt.headers)

			callerInfo := GetCallerInfoFromContext(resultCtx)
			assert.Equal(t, tt.wantCaller, callerInfo.GetCallerType())
		})
	}
}
