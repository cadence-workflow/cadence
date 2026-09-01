// Copyright (c) 2026 Uber Technologies, Inc.
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

package semaphore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIdentifier(t *testing.T) {
	tests := []struct {
		name          string
		domainID      string
		semaphoreName string
		bucket        int
		wantErr       bool
		wantString    string
	}{
		{
			name:          "valid",
			domainID:      "domain-1",
			semaphoreName: "sem-1",
			bucket:        2,
			wantString:    "domain-1/sem-1/2",
		},
		{
			name:          "bucket zero is valid",
			domainID:      "domain-1",
			semaphoreName: "sem-1",
			bucket:        0,
			wantString:    "domain-1/sem-1/0",
		},
		{name: "empty domain id", semaphoreName: "sem-1", wantErr: true},
		{name: "empty semaphore name", domainID: "domain-1", wantErr: true},
		{name: "negative bucket", domainID: "domain-1", semaphoreName: "sem-1", bucket: -1, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := NewIdentifier(tc.domainID, tc.semaphoreName, tc.bucket)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantString, id.String())
		})
	}
}

func TestIdentifierIsUsableAsAMapKey(t *testing.T) {
	// Buckets are looked up by value, so the struct has to stay comparable — adding a
	// slice or map field to it would break this at compile time, which is the point.
	a, err := NewIdentifier("domain-1", "sem-1", 0)
	require.NoError(t, err)
	b, err := NewIdentifier("domain-1", "sem-1", 1)
	require.NoError(t, err)

	buckets := map[Identifier]string{a: "first", b: "second"}
	same, err := NewIdentifier("domain-1", "sem-1", 0)
	require.NoError(t, err)
	assert.Equal(t, "first", buckets[same])
	assert.Len(t, buckets, 2)
}
