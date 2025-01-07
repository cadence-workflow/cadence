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

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewHistogram tests the creation of a new Histogram
func TestNewHistogram(t *testing.T) {
	h := NewHistogram()
	if h.maxCount != defaultWidth || h.maxKey != "Bucket" {
		t.Errorf("NewHistogram() failed. Got maxCount = %d, maxKey = %s, want maxCount = %d, maxKey = Bucket",
			h.maxCount, h.maxKey, defaultWidth)
	}
}

// TestAdd tests the Add function of Histogram
func TestHistogram_Add(t *testing.T) {
	h := NewHistogram()

	h.Add("key1")
	if len(h.counters) != 1 {
		t.Errorf("Add() failed. Expected 1 counter, got %d", len(h.counters))
	}

	h.Add("key1")
	if h.counters[0].count != 2 {
		t.Errorf("Add() failed. Expected count of 2, got %d", h.counters[0].count)
	}

	h.Add("key2")
	if len(h.counters) != 2 {
		t.Errorf("Add() failed. Expected 2 counters, got %d", len(h.counters))
	}

	// Check if maxKey is still "Bucket" because "key1" and "key2" are shorter than "Bucket"
	if h.maxKey != "Bucket" {
		t.Errorf("Add() failed. Expected maxKey 'Bucket', got %s", h.maxKey)
	}

	// Add a longer key to test maxKey update
	longKey := "longerKeyThanBucket"
	h.Add(longKey)

	if h.maxKey != longKey {
		t.Errorf("Add() failed. Expected maxKey '%s', got %s", longKey, h.maxKey)
	}
}

// TestAddWithMaxCount tests the Add function and covers the maxCount update
func TestHistogram_AddWithMaxCount(t *testing.T) {
	h := NewHistogram()

	// Add "key1" multiple times to exceed the initial maxCount (defaultWidth)
	for i := 0; i < defaultWidth+1; i++ {
		h.Add("key1")
	}

	// Check if maxCount has been updated to the new value
	if h.maxCount != defaultWidth+1 {
		t.Errorf("Add() failed. Expected maxCount to be %d, got %d", defaultWidth+1, h.maxCount)
	}

	// Add another key and check that maxCount doesn't change unless exceeded
	h.Add("key2")
	if h.maxCount != defaultWidth+1 {
		t.Errorf("Add() failed. Expected maxCount to remain %d, got %d", defaultWidth+1, h.maxCount)
	}
}

// TestAddMultiplier tests the addMultiplier function
func TestHistogram_AddMultiplier(t *testing.T) {
	h := NewHistogram()

	h.Add("key1")
	h.Add("key2")

	h.addMultiplier(2)

	if h.counters[0].count != 2 || h.counters[1].count != 2 {
		t.Errorf("addMultiplier() failed. Expected count of 2, got %d and %d",
			h.counters[0].count, h.counters[1].count)
	}
}

// TestPrint tests the Print function with multiplier
func TestHistogram_Print(t *testing.T) {
	h := NewHistogram()

	// Add sample keys to the histogram
	h.Add("key1")
	h.Add("key2")

	// Use a buffer to capture the output
	var buf bytes.Buffer

	// Call the Print function with the buffer as the output writer and a multiplier
	err := h.Print(&buf, 2)
	if err != nil {
		t.Errorf("Print() failed. Expected no error, got %v", err)
	}

	output := buf.String()

	// Check if the output contains the expected headers and keys
	if !strings.Contains(output, "Bucket") {
		t.Errorf("Print() failed. Expected header 'Bucket' in output")
	}

	if !strings.Contains(output, "key1") || !strings.Contains(output, "key2") {
		t.Errorf("Print() failed. Expected 'key1' and 'key2' in output")
	}
}

// TestSortFunctions tests Len, Less, and Swap methods
func TestHistogram_SortFunctions(t *testing.T) {
	h := NewHistogram()

	h.Add("key2")
	h.Add("key1")
	h.Add("key3")

	if !h.Less(1, 0) {
		t.Errorf("Less() failed. Expected key1 < key2")
	}

	h.Swap(0, 2)
	if h.counters[0].key != "key3" {
		t.Errorf("Swap() failed. Expected key3 at index 0, got %s", h.counters[0].key)
	}

	if h.Len() != 3 {
		t.Errorf("Len() failed. Expected length 3, got %d", h.Len())
	}
}
