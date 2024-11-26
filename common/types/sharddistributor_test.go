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

package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetShardOwnerRequest_GetShardKey(t *testing.T) {
	tests := []struct {
		name string
		req  *GetShardOwnerRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty request",
			req:  &GetShardOwnerRequest{},
			want: "",
		},
		{
			name: "has shard key",
			req:  &GetShardOwnerRequest{ShardKey: "shard-key"},
			want: "shard-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetShardKey()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetShardOwnerRequest_GetNamespace(t *testing.T) {
	tests := []struct {
		name string
		req  *GetShardOwnerRequest
		want string
	}{
		{
			name: "nil request",
			req:  nil,
			want: "",
		},
		{
			name: "empty request",
			req:  &GetShardOwnerRequest{},
			want: "",
		},
		{
			name: "has namespace",
			req:  &GetShardOwnerRequest{Namespace: "namespace"},
			want: "namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.req.GetNamespace()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetShardOwnerResponse_GetOwner(t *testing.T) {
	tests := []struct {
		name string
		resp *GetShardOwnerResponse
		want string
	}{
		{
			name: "nil response",
			resp: nil,
			want: "",
		},
		{
			name: "empty response",
			resp: &GetShardOwnerResponse{},
			want: "",
		},
		{
			name: "has owner",
			resp: &GetShardOwnerResponse{Owner: "owner"},
			want: "owner",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetOwner()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetShardOwnerResponse_GetNamespace(t *testing.T) {
	tests := []struct {
		name string
		resp *GetShardOwnerResponse
		want string
	}{
		{
			name: "nil response",
			resp: nil,
			want: "",
		},
		{
			name: "empty response",
			resp: &GetShardOwnerResponse{},
			want: "",
		},
		{
			name: "has namespace",
			resp: &GetShardOwnerResponse{Namespace: "namespace"},
			want: "namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.GetNamespace()
			assert.Equal(t, tt.want, got)
		})
	}
}
