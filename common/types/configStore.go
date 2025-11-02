// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

type DynamicConfigBlob struct {
	SchemaVersion int64                 `json:"schemaVersion,omitempty"`
	Entries       []*DynamicConfigEntry `json:"entries,omitempty"`
}

type DynamicConfigEntry struct {
	Name   string                `json:"name,omitempty"`
	Values []*DynamicConfigValue `json:"values,omitempty"`
}

type DynamicConfigValue struct {
	Value   *DataBlob              `json:"value,omitempty"`
	Filters []*DynamicConfigFilter `json:"filters,omitempty"`
}

type DynamicConfigFilter struct {
	Name  string    `json:"name,omitempty"`
	Value *DataBlob `json:"value,omitempty"`
}

func (dcf *DynamicConfigFilter) Copy() *DynamicConfigFilter {
	if dcf == nil {
		return nil
	}
	var valueCopy *DataBlob
	if dcf.Value != nil {
		v := dcf.Value.DeepCopy()
		valueCopy = &v
	}
	return &DynamicConfigFilter{
		Name:  dcf.Name,
		Value: valueCopy,
	}
}

func (dcv *DynamicConfigValue) Copy() *DynamicConfigValue {
	if dcv == nil {
		return nil
	}

	var newFilters []*DynamicConfigFilter
	if dcv.Filters != nil {
		newFilters = make([]*DynamicConfigFilter, 0, len(dcv.Filters))
		for _, filter := range dcv.Filters {
			newFilters = append(newFilters, filter.Copy())
		}
	}

	var valueCopy *DataBlob
	if dcv.Value != nil {
		v := dcv.Value.DeepCopy()
		valueCopy = &v
	}
	return &DynamicConfigValue{
		Value:   valueCopy,
		Filters: newFilters,
	}
}

func (dce *DynamicConfigEntry) Copy() *DynamicConfigEntry {
	if dce == nil {
		return nil
	}

	var newValues []*DynamicConfigValue
	if dce.Values != nil {
		newValues = make([]*DynamicConfigValue, 0, len(dce.Values))
		for _, value := range dce.Values {
			newValues = append(newValues, value.Copy())
		}
	}

	return &DynamicConfigEntry{
		Name:   dce.Name,
		Values: newValues,
	}
}
