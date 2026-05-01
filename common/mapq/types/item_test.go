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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"
)

func TestNewItemToPersist(t *testing.T) {
	ctrl := gomock.NewController(t)
	item := NewMockItem(ctrl)
	itemStr := "###item###"
	item.EXPECT().String().Return(itemStr).Times(1)
	item.EXPECT().GetAttribute("attr1").Return("value1").Times(1)
	item.EXPECT().GetAttribute("attr2").Return("value2").Times(1)

	partitions := []string{"attr1", "attr2"}
	itemPartitions := NewItemPartitions(
		partitions,
		map[string]any{
			"attr1": "*",
			"attr2": "value2",
		},
	)

	itemToPersist := NewItemToPersist(item, itemPartitions)
	if itemToPersist == nil {
		t.Fatal("itemToPersist is nil")
	}

	if got := itemToPersist.GetAttribute("attr1"); got != "value1" {
		t.Errorf("itemToPersist.GetAttribute(attr1) = %v, want %v", got, "value1")
	}
	if got := itemToPersist.GetAttribute("attr2"); got != "value2" {
		t.Errorf("itemToPersist.GetAttribute(attr2) = %v, want %v", got, "value2")
	}

	gotPartitions := itemToPersist.GetPartitionKeys()
	if diff := cmp.Diff(partitions, gotPartitions); diff != "" {
		t.Fatalf("Partition keys mismatch (-want +got):\n%s", diff)
	}
	if got := itemToPersist.GetPartitionValue("attr1"); got != "*" {
		t.Errorf("itemToPersist.GetPartitionValue(attr1) = %v, want %v", got, "*")
	}
	if got := itemToPersist.GetPartitionValue("attr2"); got != "value2" {
		t.Errorf("itemToPersist.GetPartitionValue(attr2) = %v, want %v", got, "value2")
	}

	itemToPersistStr := itemToPersist.String()
	itemPartitionsStr := itemPartitions.String()
	if !strings.Contains(itemToPersistStr, itemPartitionsStr) {
		t.Errorf("itemToPersist.String() = %v, want to contain %v", itemToPersistStr, itemPartitionsStr)
	}
	if !strings.Contains(itemToPersistStr, itemStr) {
		t.Errorf("itemToPersist.String() = %v, want to contain %v", itemToPersistStr, itemStr)
	}
}
