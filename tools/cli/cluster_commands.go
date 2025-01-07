// Copyright (c) 2017 Uber Technologies, Inc.
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

package cli

import (
	"os"
	"sort"

	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/tools/common/commoncli"
)

type (
	SearchAttributesRow struct {
		Key       string `header:"Key"`
		ValueType string `header:"Value type"`
	}
	SearchAttributesTable []SearchAttributesRow
)

func (s SearchAttributesTable) Len() int {
	return len(s)
}
func (s SearchAttributesTable) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SearchAttributesTable) Less(i, j int) bool {
	return s[i].Key < s[j].Key
}

// GetSearchAttributes get valid search attributes
func GetSearchAttributes(c *cli.Context) error {
	wfClient, err := getWorkflowClient(c)
	if err != nil {
		return err
	}
	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context:", err)
	}

	resp, err := wfClient.GetSearchAttributes(ctx)
	if err != nil {
		return commoncli.Problem("Failed to get search attributes.", err)
	}

	table := SearchAttributesTable{}
	for k, v := range resp.Keys {
		table = append(table, SearchAttributesRow{Key: k, ValueType: v.String()})
	}
	sort.Sort(table)
	return RenderTable(os.Stdout, table, RenderOptions{Color: true, Border: true})
}
