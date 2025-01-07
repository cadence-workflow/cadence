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

package pinot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xwb1989/sqlparser"

	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig"
)

func TestValidateQuery(t *testing.T) {
	tests := map[string]struct {
		query     string
		validated string
		err       string
	}{
		"Case1: empty query": {
			query:     "",
			validated: "",
		},
		"Case2-1: simple query": {
			query:     "WorkflowID = 'wid'",
			validated: "WorkflowID = 'wid'",
		},
		"Case2-2: simple query with partial match": {
			query:     "WorkflowID like 'wid'",
			validated: "TEXT_MATCH(WorkflowID, '/.*wid.*/')",
		},
		"Case2-3: invalid simple query with partial match": {
			query: "WorkflowID like wid",
			err:   "right comparison is invalid: &{<nil> wid { }}"},
		"Case3-1: query with custom field": {
			query:     "CustomStringField = 'custom'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*custom.*'')')`,
		},
		"Case3-2: query with custom field not equal": {
			query:     "CustomStringField != 'custom'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*custom.*'')')`,
		},
		"Case3-3: query with custom field value is empty": {
			query:     "CustomStringField = ''",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''^$'')')`,
		},
		"Case3-4: query with custom field not equal to empty": {
			query:     "CustomStringField != ''",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''^$'')')`,
		},
		"Case4: custom field query with or in string": {
			query:     "CustomStringField='Or'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*Or.*'')')`,
		},
		"Case5: custom keyword field query": {
			query:     "CustomKeywordField = 'custom'",
			validated: `(JSON_MATCH(Attr, '"$.CustomKeywordField"=''custom''') or JSON_MATCH(Attr, '"$.CustomKeywordField[*]"=''custom'''))`,
		},
		"Case6-1: complex query I: with parenthesis": {
			query:     "(CustomStringField = 'custom and custom2 or custom3 order by') or CustomIntField between 1 and 10",
			validated: `(JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*custom and custom2 or custom3 order by.*'')') or (JSON_MATCH(Attr, '"$.CustomIntField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) >= 1 AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) <= 10))`,
		},
		"Case6-2: complex query II: with only system keys": {
			query:     "DomainID = 'd-id' and (RunID = 'run-id' or WorkflowID = 'wid')",
			validated: "DomainID = 'd-id' and (RunID = 'run-id' or WorkflowID = 'wid')",
		},
		"Case6-3: complex query III: operation priorities": {
			query:     "DomainID = 'd-id' or RunID = 'run-id' and WorkflowID = 'wid'",
			validated: "(DomainID = 'd-id' or RunID = 'run-id' and WorkflowID = 'wid')",
		},
		"Case6-4: complex query IV": {
			query:     "WorkflowID = 'wid' and (CustomStringField = 'custom and custom2 or custom3 order by' or CustomIntField between 1 and 10)",
			validated: `WorkflowID = 'wid' and (JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*custom and custom2 or custom3 order by.*'')') or (JSON_MATCH(Attr, '"$.CustomIntField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) >= 1 AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) <= 10))`,
		},
		"Case6-5: complex query with partial match": {
			query:     "RunID like '123' or WorkflowID like '123'",
			validated: "(TEXT_MATCH(RunID, '/.*123.*/') or TEXT_MATCH(WorkflowID, '/.*123.*/'))",
		},
		"Case7: invalid sql query": {
			query: "Invalid SQL",
			err:   "Invalid query: syntax error at position 38 near 'sql'",
		},
		"Case8-1: query with missing val": {
			query:     "CloseTime = missing",
			validated: "CloseTime = -1",
		},
		"Case8-2: query with not missing case": {
			query:     "CloseTime != missing",
			validated: "CloseTime != -1",
		},
		"Case8-3: query with custom attr": {
			query: "CustomKeywordField = missing",
			err:   "invalid comparison expression, right",
		},
		"Case8-4: query with custom keyword field not equal": {
			query:     "CustomKeywordField != 0",
			validated: `(JSON_MATCH(Attr, '"$.CustomKeywordField"!=''0''') and JSON_MATCH(Attr, '"$.CustomKeywordField[*]"!=''0'''))`,
		},
		"Case9: invalid where expression": {
			query: "InvalidWhereExpr",
			err:   "invalid where clause",
		},
		"Case10: invalid search attribute": {
			query: "Invalid = 'a' and 1 < 2",
			err:   `invalid search attribute "Invalid"`,
		},
		"Case11-1: order by clause": {
			query:     "order by CloseTime desc",
			validated: " order by CloseTime desc",
		},
		"Case11-2: only order by clause with custom field": {
			query:     "order by CustomIntField desc",
			validated: " order by CustomIntField desc",
		},
		"Case11-3: order by clause with custom field": {
			query:     "WorkflowID = 'wid' order by CloseTime desc",
			validated: "WorkflowID = 'wid' order by CloseTime desc",
		},
		"Case12-1: security SQL injection - with another statement": {
			query: "WorkflowID = 'wid'; SELECT * FROM important_table;",
			err:   "Invalid query: syntax error at position 53 near 'select'",
		},
		"Case12-2: security SQL injection - with union": {
			query: "WorkflowID = 'wid' union select * from dummy",
			err:   "Invalid select query.",
		},
		"Case13: or clause": {
			query:     "CustomIntField = 1 or CustomIntField = 2",
			validated: `(JSON_MATCH(Attr, '"$.CustomIntField"=''1''') or JSON_MATCH(Attr, '"$.CustomIntField"=''2'''))`,
		},
		"Case14-1: range query: custom filed": {
			query:     "CustomIntField BETWEEN 1 AND 2",
			validated: `(JSON_MATCH(Attr, '"$.CustomIntField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) >= 1 AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) <= 2)`,
		},
		"Case14-2: range query: system filed": {
			query:     "NumClusters BETWEEN 1 AND 2",
			validated: "NumClusters between 1 and 2",
		},
		"Case15-1: custom date attribute less than": {
			query:     "CustomDatetimeField < 1697754674",
			validated: `(JSON_MATCH(Attr, '"$.CustomDatetimeField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomDatetimeField') AS BIGINT) < 1697754674)`,
		},
		"Case15-2: custom date attribute greater than or equal to": {
			query:     "CustomDatetimeField >= 1697754674",
			validated: `(JSON_MATCH(Attr, '"$.CustomDatetimeField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomDatetimeField') AS BIGINT) >= 1697754674)`,
		},
		"Case15-3: system date attribute greater than or equal to": {
			query:     "StartTime >= 1697754674",
			validated: "StartTime >= 1697754674",
		},
		"Case15-4: unix nano converts to milli seconds for equal statements": {
			query:     "StartTime = 1707319950934000128",
			validated: "StartTime = 1707319950934",
		},
		"Case15-5: unix nano converts to milli seconds for unequal statements query": {
			query:     "StartTime > 1707319950934000128",
			validated: "StartTime > 1707319950934",
		},
		"Case15-6: open workflows": {
			query:     "CloseTime = -1",
			validated: "CloseTime = -1",
		},
		"Case15-7: startTime for range query": {
			query:     "StartTime BETWEEN 1707319950934000128 AND 1707319950935000128",
			validated: "StartTime between 1707319950934 and 1707319950935",
		},
		"Case15-8: invalid string for trim": {
			query:     "CloseTime = abc",
			validated: "",
			err:       "right comparison is invalid string value: abc",
		},
		"Case15-9: invalid value for trim": {
			query:     "CloseTime = 123.45",
			validated: "",
			err:       "trim time field CloseTime got error: error: failed to parse int from SQLVal 123.45",
		},
		"Case15-10: invalid from time for range query": {
			query:     "StartTime BETWEEN 17.50 AND 1707319950935000128",
			validated: "",
			err:       "trim time field StartTime got error: error: failed to parse int from SQLVal 17.50",
		},
		"Case15-11: invalid to time for range query": {
			query:     "StartTime BETWEEN 1707319950934000128 AND 1707319950935000128.1",
			validated: "",
			err:       "trim time field StartTime got error: error: failed to parse int from SQLVal 1707319950935000128.1",
		},
		"Case15-12: value already in milliseconds": {
			query:     "StartTime = 170731995093",
			validated: "StartTime = 170731995093",
		},
		"Case15-13: value in raw string for equal statement": {
			query:     "StartTime = '2024-02-07T15:32:30Z'",
			validated: "StartTime = 1707319950000",
		},
		"Case15-14: value in raw string for not equal statement": {
			query:     "StartTime > '2024-02-07T15:32:30Z'",
			validated: "StartTime > 1707319950000",
		},
		"Case15-15: value in raw string for range statement": {
			query:     "StartTime between '2024-02-07T15:32:30Z' and '2024-02-07T15:33:30Z'",
			validated: "StartTime between 1707319950000 and 1707320010000",
		},
		"Case15-16: combined time and missing case": {
			query:     "CloseTime != missing and StartTime >= 1707662555754408145",
			validated: "CloseTime != -1 and StartTime >= 1707662555754",
		},
		"Case15-17: CustomDatetimeField with big int type case": {
			query:     "CustomDatetimeField = 1707319950000",
			validated: `JSON_MATCH(Attr, '"$.CustomDatetimeField"=''1707319950000''')`,
		},
		"Case15-18: CustomDatetimeField with time.Time() type case": {
			query:     "CustomDatetimeField = '2024-02-07T15:32:30Z'",
			validated: `JSON_MATCH(Attr, '"$.CustomDatetimeField"=''1707319950000''')`,
		},
		"Case15-19: CustomDatetimeField with error case": {
			query:     "CustomDatetimeField = 'test'",
			validated: "",
			err:       "trim time field CustomDatetimeField got error: error: failed to parse int from SQLVal test",
		},
		"Case16-1: custom int attribute greater than or equal to": {
			query:     "CustomIntField >= 0",
			validated: `(JSON_MATCH(Attr, '"$.CustomIntField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomIntField') AS INT) >= 0)`,
		},
		"Case16-2: custom double attribute greater than or equal to": {
			query:     "CustomDoubleField >= 0",
			validated: `(JSON_MATCH(Attr, '"$.CustomDoubleField" is not null') AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.CustomDoubleField') AS DOUBLE) >= 0)`,
		},
		"Case17: custom keyword attribute greater than or equal to. Will return error run time": {
			query:     "CustomKeywordField < 0",
			validated: `(JSON_MATCH(Attr, '"$.CustomKeywordField"<''0''') or JSON_MATCH(Attr, '"$.CustomKeywordField[*]"<''0'''))`,
		},
		"Case18: custom int order by. Will have errors at run time. Doesn't support for now": {
			query:     "CustomIntField = 0 order by CustomIntField desc",
			validated: `JSON_MATCH(Attr, '"$.CustomIntField"=''0''') order by CustomIntField desc`,
		},
		"case19-1: close status parse string": {
			query:     "CloseStatus = 'CONTINUED_AS_NEW'",
			validated: "CloseStatus = 4",
		},
		"case19-2: close status parse number": {
			query:     "CloseStatus = '1'",
			validated: "CloseStatus = 1",
		},
		"case19-3: close status parse normal case": {
			query:     "CloseStatus = 1",
			validated: "CloseStatus = 1",
		},
		"case20-1: in clause in Attr": {
			query:     "CustomKeywordField in (123)",
			validated: `JSON_MATCH(Attr, '"$.CustomKeywordField" IN (''123'')') or JSON_MATCH(Attr, '"$.CustomKeywordField[*]" IN (''123'')')`,
		},
		"case20-2: in clause in Attr with multiple values": {
			query:     "CustomKeywordField in (123, 456)",
			validated: `JSON_MATCH(Attr, '"$.CustomKeywordField" IN (''123'',''456'')') or JSON_MATCH(Attr, '"$.CustomKeywordField[*]" IN (''123'',''456'')')`,
		},
		"case20-3-1: in clause in Attr with a string value, double quote": {
			query:     `CustomKeywordField in ("abc")`,
			validated: `JSON_MATCH(Attr, '"$.CustomKeywordField" IN (''abc'')') or JSON_MATCH(Attr, '"$.CustomKeywordField[*]" IN (''abc'')')`,
		},
		"case20-3-2: in clause in Attr with a string value, single quote": {
			query:     "CustomKeywordField in ('abc')",
			validated: `JSON_MATCH(Attr, '"$.CustomKeywordField" IN (''abc'')') or JSON_MATCH(Attr, '"$.CustomKeywordField[*]" IN (''abc'')')`,
		},
		"case20-4: in clause in Attr with invalid IN expression, value": {
			query:     "CustomKeywordField in (abc)",
			validated: "",
			err:       "invalid IN expression, value",
		},
		"case21-1: test bool value- system key- no quotes": {
			query:     "IsCron = true",
			validated: "IsCron = true",
		},
		"case21-2: test bool value- system key- single quotes": {
			query:     "IsCron = 'true'",
			validated: "IsCron = true",
		},
		"case21-3: test bool value- system key- double quotes": {
			query:     `IsCron = "true"`,
			validated: "IsCron = true",
		},
		"case21-4: test bool value- system key- invalid value": {
			query:     "IsCron = 1",
			validated: "",
			err:       "invalid bool value in pinot_query_validator: 1",
		},
		"case21-5: test bool value- when it is not SQLBool and SQLVAl": {
			query:     "IsCron = abc",
			validated: "",
			err:       "failed to process a bool key to SQLVal: &{<nil> abc { }}",
		},
		"case22-1: test not equal to a string field": {
			query:     "CustomStringField != 'abc'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')')`,
		},
		"case22-2: test not equal to an empty string": {
			query:     "CustomStringField != ''",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''^$'')')`,
		},
		// ES also doesn't support this kind of query
		"case22-3: custom string is missing": {
			query: "CustomStringField is missing",
			err:   "Invalid query: syntax error at position 55 near 'missing'",
		},
		// ES also doesn't support this kind of query
		"case22-4: custom string is not missing": {
			query: "CustomStringField is not missing",
			err:   "Invalid query: syntax error at position 59 near 'missing'",
		},
		"case22-5: 2 custom string not equal with and clause": {
			query:     "CustomStringField != 'abc' AND CustomStringField != 'def'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') and JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')')`,
		},
		"case22-6: 2 custom string, equal and not equal with and clause": {
			query:     "CustomStringField = 'abc' AND CustomStringField != 'def'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') and JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')')`,
		},
		"case22-7: 2 custom string, not equal and equal with and clause": {
			query:     "CustomStringField != 'abc' AND CustomStringField = 'def'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') and JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')')`,
		},
		"case22-8: 2 custom string equal with and clause": {
			query:     "CustomStringField = 'abc' AND CustomStringField = 'def'",
			validated: `JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') and JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')')`,
		},
		"case22-9: 2 custom string not equal with or clause": {
			query:     "CustomStringField != 'abc' OR CustomStringField != 'def'",
			validated: `(JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') or JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')'))`,
		},
		"case22-10: 2 custom string, equal and not equal with or clause": {
			query:     "CustomStringField = 'abc' OR CustomStringField != 'def'",
			validated: `(JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') or JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')'))`,
		},
		"case22-11: 2 custom string, not equal and equal with or clause": {
			query:     "CustomStringField != 'abc' OR CustomStringField = 'def'",
			validated: `(JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') or JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')'))`,
		},
		"case22-12: 2 custom string equal with or clause": {
			query:     "CustomStringField = 'abc' OR CustomStringField = 'def'",
			validated: `(JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*abc.*'')') or JSON_MATCH(Attr, '"$.CustomStringField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomStringField", ''.*def.*'')'))`,
		},
		"case23-1: custom keyword field is empty case": {
			query:     "CustomKeywordField = ''",
			validated: `JSON_MATCH(Attr, '"$.CustomKeywordField" is not null') AND JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomKeywordField", ''^$'')')`,
		},
		"case23-2: custom keyword field is not empty case": {
			query:     "CustomKeywordField != ''",
			validated: `JSON_MATCH(Attr, '"$.CustomKeywordField" is not null') AND NOT JSON_MATCH(Attr, 'REGEXP_LIKE("$.CustomKeywordField", ''^$'')')`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			validSearchAttr := dynamicconfig.GetMapPropertyFn(definition.GetDefaultIndexedKeys())
			qv := NewPinotQueryValidator(validSearchAttr)
			validated, err := qv.ValidateQuery(test.query)
			if err != nil {
				assert.Equal(t, test.err, err.Error())
			} else {
				assert.Equal(t, test.validated, validated)
			}
		})
	}
}

func TestProcessInClause_FailedInputExprCases(t *testing.T) {
	// Define test cases
	tests := map[string]struct {
		inputExpr     sqlparser.Expr
		expectedError string
	}{
		"case1: 'In' clause in Attr with invalid expr": {
			inputExpr:     &sqlparser.SQLVal{Type: sqlparser.StrVal, Val: []byte("invalid")},
			expectedError: "invalid IN expression",
		},
		"case2: 'In' clause in Attr with invalid expr, left": {
			inputExpr:     &sqlparser.ComparisonExpr{Operator: sqlparser.InStr},
			expectedError: "invalid IN expression, left",
		},
		"case3: 'In' clause in Attr with invalid expr, right": {
			inputExpr:     &sqlparser.ComparisonExpr{Operator: sqlparser.InStr, Left: &sqlparser.ColName{Name: sqlparser.NewColIdent("CustomKeywordField")}},
			expectedError: "invalid IN expression, right",
		},
	}

	// Create a new VisibilityQueryValidator
	validSearchAttr := dynamicconfig.GetMapPropertyFn(definition.GetDefaultIndexedKeys())
	qv := NewPinotQueryValidator(validSearchAttr)

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Call processInClause with the input expression
			_, err := qv.processInClause(test.inputExpr)

			// Check that an error was returned and that the error message matches the expected error message
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), test.expectedError)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	var tests = []struct {
		name     string
		timeStr  string
		expected int64
		hasErr   bool
	}{
		{"empty string", "", 0, true},
		{"valid RFC3339", "2024-02-07T15:32:30Z", 1707319950000, false},
		{"valid unix milli string", "1707319950000", 1707319950000, false},
		{"valid unix nano string", "1707319950000000000", 1707319950000, false},
		{"invalid string", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseTime(tt.timeStr)
			assert.Equal(t, parsed, tt.expected)
			if tt.hasErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestParseCloseStatus(t *testing.T) {
	tests := []struct {
		input       string
		expected    *sqlparser.SQLVal
		expectedErr bool
	}{
		{
			input:       "4",
			expected:    &sqlparser.SQLVal{Type: sqlparser.IntVal, Val: []byte("4")},
			expectedErr: false,
		},
		{
			input:       "CANCELED",
			expected:    &sqlparser.SQLVal{Type: sqlparser.IntVal, Val: []byte("2")},
			expectedErr: false,
		},
		{
			input:       "invalid",
			expected:    nil,
			expectedErr: true, // expected error,
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			original := &sqlparser.SQLVal{Type: sqlparser.IntVal, Val: []byte(test.input)}
			result, err := parseCloseStatus(original)

			assert.Equal(t, test.expected, result)
			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
