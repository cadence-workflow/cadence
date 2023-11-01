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

package persistence

import (
	"errors"
	"fmt"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/types"
	"github.com/xwb1989/sqlparser"
	"strings"
)

// VisibilityQueryFilter for sql query validation
type VisibilityQueryFilter struct {
}

// NewVisibilityQueryFilter create VisibilityQueryValidator
func NewVisibilityQueryFilter() *VisibilityQueryFilter {
	return &VisibilityQueryFilter{}
}

// FilterQuery validates that search attributes in the query and returns modified query.
func (qv *VisibilityQueryFilter) FilterQuery(whereClause string) (string, error) {
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
			return "", &types.BadRequestError{Message: "Invalid query."}
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

func (qv *VisibilityQueryFilter) validateWhereExpr(expr sqlparser.Expr) (string, error) {
	if expr == nil {
		return "", nil
	}
	switch expr := expr.(type) {
	case *sqlparser.AndExpr, *sqlparser.OrExpr:
		return qv.filterAndOrExpr(expr)
	case *sqlparser.ComparisonExpr:
		return qv.filterComparisonExpr(expr)
	case *sqlparser.RangeCond:
		return qv.filterRangeExpr(expr)
	case *sqlparser.ParenExpr:
		return qv.validateWhereExpr(expr.Expr)
	default:
		return "", errors.New("invalid where clause")
	}
}

// for "between...and..." only
// <, >, >=, <= are included in validateComparisonExpr()
func (qv *VisibilityQueryFilter) filterRangeExpr(expr sqlparser.Expr) (string, error) {
	buf := sqlparser.NewTrackedBuffer(nil)
	rangeCond := expr.(*sqlparser.RangeCond)
	colName, ok := rangeCond.Left.(*sqlparser.ColName)
	if !ok {
		return "", errors.New("invalid range expression: fail to get colname")
	}
	colNameStr := colName.Name.String()

	if colNameStr == definition.StartTime {
		return "", nil
	}
	expr.Format(buf)
	return buf.String(), nil
}

func (qv *VisibilityQueryFilter) filterAndOrExpr(expr sqlparser.Expr) (string, error) {
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

	if leftRes == "" || rightRes == "" {
		return leftRes + rightRes, nil
	}

	if isAnd {
		return fmt.Sprintf("%s and %s", leftRes, rightRes), nil
	}

	return fmt.Sprintf("%s or %s", leftRes, rightRes), nil
}

func (qv *VisibilityQueryFilter) filterComparisonExpr(expr sqlparser.Expr) (string, error) {
	comparisonExpr := expr.(*sqlparser.ComparisonExpr)
	buf := sqlparser.NewTrackedBuffer(nil)

	colName, ok := comparisonExpr.Left.(*sqlparser.ColName)
	if !ok {
		return "", errors.New("invalid comparison expression, left")
	}

	colNameStr := colName.Name.String()

	if colNameStr == definition.StartTime {
		return "", nil
	}
	expr.Format(buf)
	return buf.String(), nil
}
