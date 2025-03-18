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

package dynamicconfig

import (
	"testing"

	"github.com/dgryski/go-farm"
)

func TestConstraintsKey(t *testing.T) {

	tests := map[string]struct {
		constraints Constraints
		expected    uint64
	}{
		"empty constraints": {
			constraints: Constraints{},
			expected:    farm.Hash64([]byte("-")),
		},
		"empty constraint value": {
			constraints: Constraints{
				"domain": "",
			},
			expected: farm.Hash64([]byte("domain-")),
		},
		"single constraint": {
			constraints: Constraints{
				"domain": "test-domain",
			},
			expected: farm.Hash64([]byte("domain-test-domain")),
		},
		"multiple constraints unsorted": {
			constraints: Constraints{
				"domain":    "test-domain",
				"taskList":  "test-tasklist",
				"namespace": "test-namespace",
			},
			expected: farm.Hash64([]byte("domain,namespace,taskList-test-domain,test-namespace,test-tasklist")),
		},
		"multiple constraints different order same hash": {
			constraints: Constraints{
				"namespace": "test-namespace",
				"domain":    "test-domain",
				"taskList":  "test-tasklist",
			},
			expected: farm.Hash64([]byte("domain,namespace,taskList-test-domain,test-namespace,test-tasklist")),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			key := tt.constraints.Key()
			if key != tt.expected {
				t.Errorf("Key() = %v, want %v", key, tt.expected)
			}
		})
	}
}
