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
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xwb1989/sqlparser"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/types"
)

// VisibilityQueryValidator for sql query validation
type VisibilityQueryValidator struct {
	validSearchAttributes dynamicconfig.MapPropertyFn
}

var timeSystemKeys = map[string]bool{
	"StartTime":     true,
	"CloseTime":     true,
	"ExecutionTime": true,
	"UpdateTime":    true,
}

// NewPinotQueryValidator create VisibilityQueryValidator
func NewPinotQueryValidator(validSearchAttributes dynamicconfig.MapPropertyFn) *VisibilityQueryValidator {
	return &VisibilityQueryValidator{
		validSearchAttributes: validSearchAttributes,
	}
}

// ValidateQuery validates that search attributes in the query and returns modified query.
func (qv *VisibilityQueryValidator) ValidateQuery(whereClause string) (string, error) {
	if len(whereClause) != 0 {
		// Build a placeholder query that allows us to easily parse the contents of the where clause.
		// IMPORTANT: This query is never executed, it is just used to parse and validate whereClause
		var placeholderQuery string
		whereClause := strings.TrimSpace(whereClause)
		if common.IsJustOrderByClause(whereClause) { // just order by
			placeholderQuery = fmt.Sprintf("SELECT * FROM dummy %s", whereClause)
		} else {
			placeholderQuery = fmt.Sprintf("SELECT * FROM dummy WHERE %s", whereClause)
		}

		stmt, err := sqlparser.Parse(placeholderQuery)
		if err != nil {
			return "", &types.BadRequestError{Message: "Invalid query: " + err.Error()}
		}

		sel, ok := stmt.(*sqlparser.Select)
		if !ok {
			return "", &types.BadRequestError{Message: "Invalid select query."}
		}
		buf := sqlparser.NewTrackedBuffer(nil)
		res := ""
		// validate where expr
		if sel.Where != nil {
			res, err = qv.validateWhereExpr(sel.Where.Expr)
			if err != nil {
				return "", &types.BadRequestError{Message: err.Error()}
			}
		}

		sel.OrderBy.Format(buf)
		res += buf.String()
		return res, nil
	}
	return whereClause, nil
}

func (qv *VisibilityQueryValidator) validateWhereExpr(expr sqlparser.Expr) (string, error) {
	if expr == nil {
		return "", nil
	}
	switch expr := expr.(type) {
	case *sqlparser.AndExpr, *sqlparser.OrExpr:
		return qv.validateAndOrExpr(expr)
	case *sqlparser.ComparisonExpr:
		return qv.validateComparisonExpr(expr)
	case *sqlparser.RangeCond:
		return qv.validateRangeExpr(expr)
	case *sqlparser.ParenExpr:
		return qv.validateWhereExpr(expr.Expr)
	default:
		return "", errors.New("invalid where clause")
	}
}

// for "between...and..." only
// <, >, >=, <= are included in validateComparisonExpr()
func (qv *VisibilityQueryValidator) validateRangeExpr(expr sqlparser.Expr) (string, error) {
	buf := sqlparser.NewTrackedBuffer(nil)
	rangeCond := expr.(*sqlparser.RangeCond)
	colName, ok := rangeCond.Left.(*sqlparser.ColName)
	if !ok {
		return "", errors.New("invalid range expression: fail to get colname")
	}
	colNameStr := colName.Name.String()

	if !qv.IsValidSearchAttributes(colNameStr) {
		return "", fmt.Errorf("invalid search attribute %q", colNameStr)
	}

	if definition.IsSystemIndexedKey(colNameStr) {
		if _, ok = timeSystemKeys[colNameStr]; ok {
			if lowerBound, ok := rangeCond.From.(*sqlparser.SQLVal); ok {
				trimmed, err := trimTimeFieldValueFromNanoToMilliSeconds(lowerBound)
				if err != nil {
					return "", fmt.Errorf("trim time field %s got error: %w", colNameStr, err)
				}
				rangeCond.From = trimmed
			}
			if upperBound, ok := rangeCond.To.(*sqlparser.SQLVal); ok {
				trimmed, err := trimTimeFieldValueFromNanoToMilliSeconds(upperBound)
				if err != nil {
					return "", fmt.Errorf("trim time field %s got error: %w", colNameStr, err)
				}
				rangeCond.To = trimmed
			}
		}
		expr.Format(buf)
		return buf.String(), nil
	}

	// lowerBound, ok := rangeCond.From.(*sqlparser.ColName)
	lowerBound, ok := rangeCond.From.(*sqlparser.SQLVal)
	if !ok {
		return "", errors.New("invalid range expression: fail to get lowerbound")
	}
	lowerBoundString := string(lowerBound.Val)

	upperBound, ok := rangeCond.To.(*sqlparser.SQLVal)
	if !ok {
		return "", errors.New("invalid range expression: fail to get upperbound")
	}
	upperBoundString := string(upperBound.Val)

	return fmt.Sprintf("(JSON_MATCH(Attr, '\"$.%s\" is not null') "+
		"AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.%s') AS INT) >= %s "+
		"AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.%s') AS INT) <= %s)", colNameStr, colNameStr, lowerBoundString, colNameStr, upperBoundString), nil
}

func (qv *VisibilityQueryValidator) validateAndOrExpr(expr sqlparser.Expr) (string, error) {
	var leftExpr sqlparser.Expr
	var rightExpr sqlparser.Expr
	isAnd := false

	switch expr := expr.(type) {
	case *sqlparser.AndExpr:
		leftExpr = expr.Left
		rightExpr = expr.Right
		isAnd = true
	case *sqlparser.OrExpr:
		leftExpr = expr.Left
		rightExpr = expr.Right
	}

	leftRes, err := qv.validateWhereExpr(leftExpr)
	if err != nil {
		return "", err
	}

	rightRes, err := qv.validateWhereExpr(rightExpr)
	if err != nil {
		return "", err
	}

	if isAnd {
		return fmt.Sprintf("%s and %s", leftRes, rightRes), nil
	}

	return fmt.Sprintf("(%s or %s)", leftRes, rightRes), nil
}

func (qv *VisibilityQueryValidator) validateComparisonExpr(expr sqlparser.Expr) (string, error) {
	comparisonExpr := expr.(*sqlparser.ComparisonExpr)

	colName, ok := comparisonExpr.Left.(*sqlparser.ColName)
	if !ok {
		return "", errors.New("invalid comparison expression, left")
	}

	colNameStr := colName.Name.String()

	if !qv.IsValidSearchAttributes(colNameStr) {
		return "", fmt.Errorf("invalid search attribute %q", colNameStr)
	}

	// Case1: it is system key
	// this means that we don't need to change the structure of the query,
	// just need to check if a value == "missing"
	if definition.IsSystemIndexedKey(colNameStr) {
		return qv.processSystemKey(expr)
	}
	// Case2: when a value is not system key
	// This means, the value is from Attr so that we need to change the query to be a Json index format
	return qv.processCustomKey(expr)
}

// IsValidSearchAttributes return true if key is registered
func (qv *VisibilityQueryValidator) IsValidSearchAttributes(key string) bool {
	validAttr := qv.validSearchAttributes()
	_, isValidKey := validAttr[key]
	return isValidKey
}

func (qv *VisibilityQueryValidator) processSystemBoolKey(colNameStr string, comparisonExpr sqlparser.ComparisonExpr) (string, error) {
	// case1: isCron = false
	colVal, ok := comparisonExpr.Right.(sqlparser.BoolVal)
	if !ok {
		// case2: isCron = "false" or isCron = 'false'
		sqlVal, ok := comparisonExpr.Right.(*sqlparser.SQLVal)
		if !ok {
			return "", fmt.Errorf("failed to process a bool key to SQLVal: %v", comparisonExpr.Right)
		}
		colValStr := string(sqlVal.Val)
		if strings.ToLower(colValStr) != "false" && strings.ToLower(colValStr) != "true" {
			return "", fmt.Errorf("invalid bool value in pinot_query_validator: %s", colValStr)
		}
		return fmt.Sprintf("%s = %s", colNameStr, colValStr), nil
	}
	return fmt.Sprintf("%s = %v", colNameStr, colVal), nil
}

func (qv *VisibilityQueryValidator) processSystemKey(expr sqlparser.Expr) (string, error) {
	comparisonExpr := expr.(*sqlparser.ComparisonExpr)
	buf := sqlparser.NewTrackedBuffer(nil)

	colName, ok := comparisonExpr.Left.(*sqlparser.ColName)
	if !ok {
		return "", fmt.Errorf("left comparison is invalid: %v", comparisonExpr.Left)
	}
	colNameStr := colName.Name.String()

	// handle system bool key
	if definition.IsSystemBoolKey(colNameStr) {
		return qv.processSystemBoolKey(colNameStr, *comparisonExpr)
	}

	if comparisonExpr.Operator == sqlparser.LikeStr {
		colVal, ok := comparisonExpr.Right.(*sqlparser.SQLVal)
		if !ok {
			return "", fmt.Errorf("right comparison is invalid: %v", comparisonExpr.Right)
		}
		colValStr := string(colVal.Val)
		return fmt.Sprintf("TEXT_MATCH(%s, '/.*%s.*/')", colNameStr, colValStr), nil
	}

	if comparisonExpr.Operator != sqlparser.EqualStr && comparisonExpr.Operator != sqlparser.NotEqualStr {
		if _, ok := timeSystemKeys[colNameStr]; ok {
			sqlVal, ok := comparisonExpr.Right.(*sqlparser.SQLVal)
			if !ok {
				return "", fmt.Errorf("right comparison is invalid: %v", comparisonExpr.Right)
			}
			trimmed, err := trimTimeFieldValueFromNanoToMilliSeconds(sqlVal)
			if err != nil {
				return "", fmt.Errorf("trim time field %s got error: %w", colNameStr, err)
			}
			comparisonExpr.Right = trimmed
		}

		expr.Format(buf)
		return buf.String(), nil
	}
	// need to deal with missing value e.g. CloseTime = missing
	// Question: why is the right side is sometimes a type of "colName", and sometimes a type of "SQLVal"?
	// Answer: for any value, sqlParser will treat any string that doesn't surrounded by single quote as ColName;
	// any string that surrounded by single quote as SQLVal
	_, ok = comparisonExpr.Right.(*sqlparser.SQLVal)
	if !ok { // this means, the value is a string, and not surrounded by single qoute, which means, val = missing
		colVal, ok := comparisonExpr.Right.(*sqlparser.ColName)
		if !ok {
			return "", fmt.Errorf("right comparison is invalid: %v", comparisonExpr.Right)
		}
		colValStr := colVal.Name.String()

		// double check if val is not missing
		if colValStr != "missing" {
			return "", fmt.Errorf("right comparison is invalid string value: %s", colValStr)
		}

		var newColVal string
		if strings.ToLower(colNameStr) == "historylength" {
			newColVal = "0"
		} else {
			newColVal = "-1" // -1 is the default value for all Closed workflows related fields
		}
		comparisonExpr.Right = &sqlparser.SQLVal{
			Type: sqlparser.IntVal, // or sqlparser.StrVal if you need to assign a string
			Val:  []byte(newColVal),
		}
	} else {
		if _, ok := timeSystemKeys[colNameStr]; ok {
			sqlVal, ok := comparisonExpr.Right.(*sqlparser.SQLVal)
			if !ok {
				return "", fmt.Errorf("right comparison is invalid/missing. key %s, right expr %v", colNameStr, comparisonExpr.Right)
			}
			trimmed, err := trimTimeFieldValueFromNanoToMilliSeconds(sqlVal)
			if err != nil {
				return "", fmt.Errorf("trim time field %s got error: %w", colNameStr, err)
			}
			comparisonExpr.Right = trimmed
		} else if colNameStr == "CloseStatus" {
			sqlVal, ok := comparisonExpr.Right.(*sqlparser.SQLVal)
			if !ok {
				return "", fmt.Errorf("right comparison is invalid: %v", comparisonExpr.Right)
			}
			closeStatus, err := parseCloseStatus(sqlVal)
			if err != nil {
				return "", fmt.Errorf("parse CloseStatus field got error: %w", err)
			}
			comparisonExpr.Right = closeStatus
		}
	}

	// For this branch, we still have a sqlExpr type. So need to use a buf to return the string
	comparisonExpr.Format(buf)
	return buf.String(), nil
}

func (qv *VisibilityQueryValidator) processInClause(expr sqlparser.Expr) (string, error) {
	comparisonExpr, ok := expr.(*sqlparser.ComparisonExpr)
	if !ok {
		return "", errors.New("invalid IN expression")
	}

	colName, ok := comparisonExpr.Left.(*sqlparser.ColName)
	if !ok {
		return "", errors.New("invalid IN expression, left")
	}

	colNameStr := colName.Name.String()
	valTuple, ok := comparisonExpr.Right.(sqlparser.ValTuple)
	if !ok {
		return "", errors.New("invalid IN expression, right")
	}

	values := make([]string, len(valTuple))
	for i, val := range valTuple {
		sqlVal, ok := val.(*sqlparser.SQLVal)
		if !ok {
			return "", errors.New("invalid IN expression, value")
		}
		values[i] = "''" + string(sqlVal.Val) + "''"
	}

	return fmt.Sprintf("JSON_MATCH(Attr, '\"$.%s\" IN (%s)') or JSON_MATCH(Attr, '\"$.%s[*]\" IN (%s)')",
		colNameStr, strings.Join(values, ","), colNameStr, strings.Join(values, ",")), nil
}

func (qv *VisibilityQueryValidator) processCustomKey(expr sqlparser.Expr) (string, error) {
	comparisonExpr := expr.(*sqlparser.ComparisonExpr)

	colName, ok := comparisonExpr.Left.(*sqlparser.ColName)
	if !ok {
		return "", errors.New("invalid comparison expression, left")
	}

	colNameStr := colName.Name.String()

	// check type: if is IndexedValueTypeString, change to like statement for partial match
	valType, ok := qv.validSearchAttributes()[colNameStr]
	if !ok {
		return "", fmt.Errorf("invalid search attribute")
	}

	// process IN clause in json indexed col: Attr
	operator := strings.ToLower(comparisonExpr.Operator)
	if operator == sqlparser.InStr {
		return qv.processInClause(expr)
	}

	// get the column value
	colVal, ok := comparisonExpr.Right.(*sqlparser.SQLVal)
	if !ok {
		return "", errors.New("invalid comparison expression, right")
	}

	// get the value type
	indexValType := common.ConvertIndexedValueTypeToInternalType(valType, log.NewNoop())
	colValStr := string(colVal.Val)

	switch indexValType {
	case types.IndexedValueTypeString:
		return processCustomString(operator, colNameStr, colValStr), nil
	case types.IndexedValueTypeKeyword:
		return processCustomKeyword(operator, colNameStr, colValStr), nil
	case types.IndexedValueTypeDatetime:
		var err error
		colVal, err = trimTimeFieldValueFromNanoToMilliSeconds(colVal)
		if err != nil {
			return "", fmt.Errorf("trim time field %s got error: %w", colNameStr, err)
		}
		colValStr := string(colVal.Val)
		return processCustomNum(operator, colNameStr, colValStr, "BIGINT"), nil
	case types.IndexedValueTypeDouble:
		return processCustomNum(operator, colNameStr, colValStr, "DOUBLE"), nil
	case types.IndexedValueTypeInt:
		return processCustomNum(operator, colNameStr, colValStr, "INT"), nil
	default:
		return processEqual(colNameStr, colValStr), nil
	}
}

func processCustomNum(operator string, colNameStr string, colValStr string, valType string) string {
	if operator == sqlparser.EqualStr {
		return processEqual(colNameStr, colValStr)
	}
	return fmt.Sprintf("(JSON_MATCH(Attr, '\"$.%s\" is not null') "+
		"AND CAST(JSON_EXTRACT_SCALAR(Attr, '$.%s') AS %s) %s %s)", colNameStr, colNameStr, valType, operator, colValStr)
}

func processEqual(colNameStr string, colValStr string) string {
	return fmt.Sprintf("JSON_MATCH(Attr, '\"$.%s\"=''%s''')", colNameStr, colValStr)
}

func processCustomKeyword(operator string, colNameStr string, colValStr string) string {
	// edge case
	if operator == "!=" {
		return createKeywordQuery(operator, colNameStr, colValStr, "and", "NOT ")
	}

	return createKeywordQuery(operator, colNameStr, colValStr, "or", "")
}

func createKeywordQuery(operator string, colNameStr string, colValStr string, connector string, notEqual string) string {
	if colValStr == "" {
		// partial match for an empty string (still it will only match empty string)
		// so it equals to exact match for an empty string
		return createCustomStringQuery(colNameStr, colValStr, notEqual)
	}
	return fmt.Sprintf("(JSON_MATCH(Attr, '\"$.%s\"%s''%s''') %s JSON_MATCH(Attr, '\"$.%s[*]\"%s''%s'''))",
		colNameStr, operator, colValStr, connector, colNameStr, operator, colValStr)
}

func processCustomString(operator string, colNameStr string, colValStr string) string {
	if operator == "!=" {
		return createCustomStringQuery(colNameStr, colValStr, "NOT ")
	}

	return createCustomStringQuery(colNameStr, colValStr, "")
}

func createCustomStringQuery(colNameStr string, colValStr string, notEqual string) string {
	// handle edge case
	if colValStr == "" {
		return fmt.Sprintf("JSON_MATCH(Attr, '\"$.%s\" is not null') "+
			"AND %sJSON_MATCH(Attr, 'REGEXP_LIKE(\"$.%s\", ''^$'')')", colNameStr, notEqual, colNameStr)
	}
	return fmt.Sprintf("JSON_MATCH(Attr, '\"$.%s\" is not null') "+
		"AND %sJSON_MATCH(Attr, 'REGEXP_LIKE(\"$.%s\", ''.*%s.*'')')", colNameStr, notEqual, colNameStr, colValStr)
}

func trimTimeFieldValueFromNanoToMilliSeconds(original *sqlparser.SQLVal) (*sqlparser.SQLVal, error) {
	// Convert the SQLVal to a string
	valStr := string(original.Val)
	newVal, err := parseTime(valStr)
	if err != nil {
		return original, fmt.Errorf("error: failed to parse int from SQLVal %s", valStr)
	}

	// Convert the new value back to SQLVal
	return &sqlparser.SQLVal{
		Type: sqlparser.IntVal,
		Val:  []byte(strconv.FormatInt(newVal, 10)),
	}, nil
}

func parseTime(timeStr string) (int64, error) {
	if len(timeStr) == 0 {
		return 0, errors.New("invalid time string")
	}

	// try to parse
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return parsedTime.UnixMilli(), nil
	}

	// treat as raw time
	valInt, err := strconv.ParseInt(timeStr, 10, 64)
	if err == nil {
		var newVal int64
		if valInt < 0 { // exclude open workflow which time field will be -1
			newVal = valInt
		} else if len(timeStr) > 13 { // Assuming nanoseconds if more than 13 digits
			newVal = valInt / 1000000 // Convert time to milliseconds
		} else {
			newVal = valInt
		}
		return newVal, nil
	}

	return 0, errors.New("invalid time string")
}

func parseCloseStatus(original *sqlparser.SQLVal) (*sqlparser.SQLVal, error) {
	statusStr := string(original.Val)

	// first check if already in int64 format
	if status, err := strconv.ParseInt(statusStr, 10, 64); err == nil {
		// Instead of returning the original value, return a new SQLVal that holds the integer value
		// Or it will fail the case CloseStatus = '1'
		return &sqlparser.SQLVal{
			Type: sqlparser.IntVal,
			Val:  []byte(strconv.FormatInt(status, 10)),
		}, nil
	}

	// try to parse close status string
	var parsedStatus types.WorkflowExecutionCloseStatus
	err := parsedStatus.UnmarshalText([]byte(statusStr))
	if err != nil {
		return nil, err
	}

	return &sqlparser.SQLVal{
		Type: sqlparser.IntVal,
		Val:  []byte(strconv.FormatInt(int64(parsedStatus), 10)),
	}, nil
}
