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

package common

import (
	"errors"

	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/service/dynamicconfig"
	"github.com/xwb1989/sqlparser"
)

// VisibilityQueryValidator for sql query validation
type VisibilityQueryValidator struct {
	validSearchAttributes dynamicconfig.MapPropertyFn
}

// NewQueryValidator create VisibilityQueryValidator
func NewQueryValidator(validSearchAttributes dynamicconfig.MapPropertyFn) *VisibilityQueryValidator {
	return &VisibilityQueryValidator{
		validSearchAttributes: validSearchAttributes,
	}
}

// ValidateListRequestForQuery validate that search attributes in listRequest query is legal,
// and add prefix for custom keys
func (qv *VisibilityQueryValidator) ValidateListRequestForQuery(listRequest *workflow.ListWorkflowExecutionsRequest) error {
	whereClause := listRequest.GetQuery()
	newQuery, err := qv.validateListOrCountRequestForQuery(whereClause)
	if err != nil {
		return err
	}
	listRequest.Query = StringPtr(newQuery)
	return nil
}

// ValidateCountRequestForQuery validate that search attributes in countRequest query is legal,
// and add prefix for custom keys
func (qv *VisibilityQueryValidator) ValidateCountRequestForQuery(countRequest *workflow.CountWorkflowExecutionsRequest) error {
	whereClause := countRequest.GetQuery()
	newQuery, err := qv.validateListOrCountRequestForQuery(whereClause)
	if err != nil {
		return err
	}
	countRequest.Query = StringPtr(newQuery)
	return nil
}

// validateListOrCountRequestForQuery valid sql for visibility API
func (qv *VisibilityQueryValidator) validateListOrCountRequestForQuery(whereClause string) (string, error) {
	if len(whereClause) != 0 {
		sqlQuery := "SELECT * FROM dummy WHERE " + whereClause
		stmt, err := sqlparser.Parse(sqlQuery)
		if err != nil {
			return "", &workflow.BadRequestError{Message: "Invalid query."}
		}

		sel := stmt.(*sqlparser.Select)
		err = qv.validateWhereExpr(sel.Where.Expr)
		if err != nil {
			return "", &workflow.BadRequestError{Message: err.Error()}
		}

		buf := sqlparser.NewTrackedBuffer(nil)
		sel.Where.Expr.Format(buf)
		return buf.String(), nil
	}
	return whereClause, nil
}

func (qv *VisibilityQueryValidator) validateWhereExpr(expr sqlparser.Expr) error {
	if expr == nil {
		return nil
	}

	switch expr.(type) {
	case *sqlparser.AndExpr, *sqlparser.OrExpr:
		return qv.validateAndOrExpr(expr)
	case *sqlparser.ComparisonExpr:
		return qv.validateComparisonExpr(expr)
	case *sqlparser.RangeCond:
		return qv.validateRangeExpr(expr)
	case *sqlparser.ParenExpr:
		parentExpr := expr.(*sqlparser.ParenExpr)
		return qv.validateWhereExpr(parentExpr.Expr)
	default:
		return errors.New("invalid where clause")
	}

	return nil
}

func (qv *VisibilityQueryValidator) validateAndOrExpr(expr sqlparser.Expr) error {
	var leftExpr sqlparser.Expr
	var rightExpr sqlparser.Expr

	switch expr.(type) {
	case *sqlparser.AndExpr:
		andExpr := expr.(*sqlparser.AndExpr)
		leftExpr = andExpr.Left
		rightExpr = andExpr.Right
	case *sqlparser.OrExpr:
		orExpr := expr.(*sqlparser.OrExpr)
		leftExpr = orExpr.Left
		rightExpr = orExpr.Right
	}

	if err := qv.validateWhereExpr(leftExpr); err != nil {
		return err
	}
	return qv.validateWhereExpr(rightExpr)
}

func (qv *VisibilityQueryValidator) validateComparisonExpr(expr sqlparser.Expr) error {
	comparisonExpr := expr.(*sqlparser.ComparisonExpr)
	colName, ok := comparisonExpr.Left.(*sqlparser.ColName)
	if !ok {
		return errors.New("invalid comparison expression")
	}
	colNameStr := colName.Name.String()
	if qv.IsValidSearchAttributes(colNameStr) {
		if !definition.IsSystemIndexedKey(colNameStr) { // add search attribute prefix
			comparisonExpr.Left = &sqlparser.ColName{
				Metadata:  colName.Metadata,
				Name:      sqlparser.NewColIdent(definition.Attr + "." + colNameStr),
				Qualifier: colName.Qualifier,
			}
		}
		return nil
	}
	return errors.New("invalid search attribute")
}

func (qv *VisibilityQueryValidator) validateRangeExpr(expr sqlparser.Expr) error {
	rangeCond := expr.(*sqlparser.RangeCond)
	colName, ok := rangeCond.Left.(*sqlparser.ColName)
	if !ok {
		return errors.New("invalid range expression")
	}
	colNameStr := colName.Name.String()
	if qv.IsValidSearchAttributes(colNameStr) {
		if !definition.IsSystemIndexedKey(colNameStr) { // add search attribute prefix
			rangeCond.Left = &sqlparser.ColName{
				Metadata:  colName.Metadata,
				Name:      sqlparser.NewColIdent(definition.Attr + "." + colNameStr),
				Qualifier: colName.Qualifier,
			}
		}
		return nil
	}
	return errors.New("invalid search attribute")
}

// IsValidSearchAttributes return true if key is registered
func (qv *VisibilityQueryValidator) IsValidSearchAttributes(key string) bool {
	validAttr := qv.validSearchAttributes()
	_, isValidKey := validAttr[key]
	return isValidKey
}
