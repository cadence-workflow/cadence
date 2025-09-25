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
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestDataBlobDeepCopy(t *testing.T) {
	tests := []struct {
		name  string
		input *DataBlob
	}{
		{
			name:  "nil",
			input: nil,
		},
		{
			name:  "empty",
			input: &DataBlob{},
		},
		{
			name: "thrift ok",
			input: &DataBlob{
				EncodingType: EncodingTypeThriftRW.Ptr(),
				Data:         []byte("some thrift data"),
			},
		},
		{
			name: "json ok",
			input: &DataBlob{
				EncodingType: EncodingTypeJSON.Ptr(),
				Data:         []byte("some json data"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.DeepCopy()
			assert.Equal(t, tc.input, got)
			if tc.input != nil && tc.input.Data != nil && identicalByteArray(tc.input.Data, got.Data) {
				t.Error("expected DeepCopy to return a new data slice")
			}
		})
	}
}

func TestActiveClustersConfigDeepCopy(t *testing.T) {
	normalConfig := &ActiveClusters{
		ActiveClustersByRegion: map[string]ActiveClusterInfo{
			"us-east-1": {
				ActiveClusterName: "us-east-1-cluster",
				FailoverVersion:   1,
			},
			"us-east-2": {
				ActiveClusterName: "us-east-2-cluster",
				FailoverVersion:   2,
			},
		},
	}

	tests := []struct {
		name   string
		input  *ActiveClusters
		expect *ActiveClusters
	}{
		{
			name:   "nil case",
			input:  nil,
			expect: nil,
		},
		{
			name:  "empty case",
			input: &ActiveClusters{},
			expect: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{},
			},
		},
		{
			name:   "normal case",
			input:  normalConfig,
			expect: normalConfig,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deepCopy := tc.input.DeepCopy()
			if diff := cmp.Diff(tc.expect, deepCopy); diff != "" {
				t.Errorf("DeepCopy() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// identicalByteArray returns true if a and b are the same slice, false otherwise.
func identicalByteArray(a, b []byte) bool {
	return len(a) == len(b) && unsafe.SliceData(a) == unsafe.SliceData(b)
}
