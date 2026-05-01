// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to qvom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, qvETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
)

func TestValidateQuery(t *testing.T) {
	tests := []struct {
		msg       string
		query     string
		validated string
		err       string
		dcValid   map[string]interface{}
	}{
		{
			msg:       "empty query",
			query:     "",
			validated: "",
		},
		{
			msg:       "simple query",
			query:     "WorkflowID = 'wid'",
			validated: "WorkflowID = 'wid'",
		},
		{
			msg:       "custom field",
			query:     "CustomStringField = 'custom'",
			validated: "`Attr.CustomStringField` = 'custom'",
		},
		{
			msg:       "complex query",
			query:     "WorkflowID = 'wid' and ((CustomStringField = 'custom') or CustomIntField between 1 and 10)",
			validated: "WorkflowID = 'wid' and ((`Attr.CustomStringField` = 'custom') or `Attr.CustomIntField` between 1 and 10)",
		},
		{
			msg:   "invalid SQL",
			query: "Invalid SQL",
			err:   "Invalid query.",
		},
		{
			msg:   "invalid where expression",
			query: "InvalidWhereExpr",
			err:   "invalid where clause",
		},
		{
			msg:   "invalid comparison",
			query: "WorkflowID = 'wid' and 1 < 2",
			err:   "invalid comparison expression",
		},
		{
			msg:   "invalid range",
			query: "1 between 1 and 2 or WorkflowID = 'wid'",
			err:   "invalid range expression",
		},
		{
			msg:   "invalid search attribute in comparison",
			query: "Invalid = 'a' and 1 < 2",
			err:   "invalid search attribute \"Invalid\"",
		},
		{
			msg:   "invalid search attribute in range",
			query: "Invalid between 1 and 2 or WorkflowID = 'wid'",
			err:   "invalid search attribute \"Invalid\"",
		},
		{
			msg:       "only order by",
			query:     "order by CloseTime desc",
			validated: " order by CloseTime desc",
		},
		{
			msg:       "only order by search attribute",
			query:     "order by CustomIntField desc",
			validated: " order by `Attr.CustomIntField` desc",
		},
		{
			msg:       "condition + order by",
			query:     "WorkflowID = 'wid' order by CloseTime desc",
			validated: "WorkflowID = 'wid' order by CloseTime desc",
		},
		{
			msg:   "invalid order by attribute",
			query: "order by InvalidField desc",
			err:   "invalid order by attribute",
		},
		{
			msg:   "invalid order by attribute expr",
			query: "order by 123",
			err:   "invalid order by expression",
		},
		{
			msg:   "security SQL injection - with another statement",
			query: "WorkflowID = 'wid'; SELECT * FROM important_table;",
			err:   "Invalid query.",
		},
		{
			msg:   "security SQL injection - with always true expression",
			query: "WorkflowID = 'wid' and (RunID = 'rid' or 1 = 1)",
			err:   "invalid comparison expression",
		},
		{
			msg:   "security SQL injection - with union",
			query: "WorkflowID = 'wid' union select * from dummy",
			err:   "Invalid select query.",
		},
		{
			msg:       "valid custom search attribute",
			query:     "CustomStringField = 'value'",
			validated: "`Attr.CustomStringField` = 'value'",
		},
		{
			msg:       "custom search attribute can contain underscore",
			query:     "Custom_Field = 'value'",
			validated: "`Attr.Custom_Field` = 'value'",
			dcValid: map[string]interface{}{
				"Custom_Field": types.IndexedValueTypeString,
			},
		},
		{
			msg:       "custom search attribute can contain number",
			query:     "Custom_0 = 'value'",
			validated: "`Attr.Custom_0` = 'value'",
			dcValid: map[string]interface{}{
				"Custom_0": types.IndexedValueTypeString,
			},
		},
		{
			msg:   "customg search attribute cannot contain dot",
			query: "Custom.Field = 'value'",
			err:   "invalid search attribute \"Field\"",
			dcValid: map[string]interface{}{
				"Custom.Field": types.IndexedValueTypeString,
			},
		},
		{
			msg:   "custom search attribute cannot contain dash",
			query: "Custom-Field = 'value'",
			err:   "invalid comparison expression",
			dcValid: map[string]interface{}{
				"Custom-Field": types.IndexedValueTypeString,
			},
		},
		{
			msg:       "custom string search attribute support not like",
			query:     "CustomStringField not like 'value'",
			validated: "`Attr.CustomStringField` not like 'value'",
			dcValid: map[string]interface{}{
				"CustomStringField": types.IndexedValueTypeString,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			validSearchAttr := func(opts ...dynamicproperties.FilterOption) map[string]interface{} {
				valid := definition.GetDefaultIndexedKeys()
				for k, v := range tt.dcValid {
					valid[k] = v
				}
				return valid
			}
			validateSearchAttr := dynamicproperties.GetBoolPropertyFn(true)
			qv := NewQueryValidator(validSearchAttr, validateSearchAttr)
			validated, err := qv.ValidateQuery(tt.query)
			if err != nil {
				assert.Equal(t, tt.err, err.Error())
			} else {
				assert.Equal(t, tt.validated, validated)
			}
		})
	}
}
