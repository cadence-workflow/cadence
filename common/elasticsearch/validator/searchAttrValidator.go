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
	"fmt"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/definition"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

// SearchAttributesValidator is used to validate search attributes
type SearchAttributesValidator struct {
	logger log.Logger

	enableQueryAttributeValidation    dynamicproperties.BoolPropertyFn
	validSearchAttributes             dynamicproperties.MapPropertyFn
	searchAttributesNumberOfKeysLimit dynamicproperties.IntPropertyFnWithDomainFilter
	searchAttributesSizeOfValueLimit  dynamicproperties.IntPropertyFnWithDomainFilter
	searchAttributesTotalSizeLimit    dynamicproperties.IntPropertyFnWithDomainFilter
}

// NewSearchAttributesValidator create SearchAttributesValidator
func NewSearchAttributesValidator(
	logger log.Logger,
	enableQueryAttributeValidation dynamicproperties.BoolPropertyFn,
	validSearchAttributes dynamicproperties.MapPropertyFn,
	searchAttributesNumberOfKeysLimit dynamicproperties.IntPropertyFnWithDomainFilter,
	searchAttributesSizeOfValueLimit dynamicproperties.IntPropertyFnWithDomainFilter,
	searchAttributesTotalSizeLimit dynamicproperties.IntPropertyFnWithDomainFilter,
) *SearchAttributesValidator {
	return &SearchAttributesValidator{
		logger:                            logger,
		enableQueryAttributeValidation:    enableQueryAttributeValidation,
		validSearchAttributes:             validSearchAttributes,
		searchAttributesNumberOfKeysLimit: searchAttributesNumberOfKeysLimit,
		searchAttributesSizeOfValueLimit:  searchAttributesSizeOfValueLimit,
		searchAttributesTotalSizeLimit:    searchAttributesTotalSizeLimit,
	}
}

// ValidateSearchAttributes validate search attributes are valid for writing and not exceed limits
func (sv *SearchAttributesValidator) ValidateSearchAttributes(input *types.SearchAttributes, domain string) error {
	if input == nil {
		return nil
	}

	// verify: number of keys <= limit
	fields := input.GetIndexedFields()
	lengthOfFields := len(fields)
	if lengthOfFields > sv.searchAttributesNumberOfKeysLimit(domain) {
		sv.logger.WithTags(tag.Number(int64(lengthOfFields)), tag.WorkflowDomainName(domain)).
			Error("number of keys in search attributes exceed limit")
		return &types.BadRequestError{Message: fmt.Sprintf("number of keys %d exceed limit", lengthOfFields)}
	}

	totalSize := 0

	validateAttrFn := sv.enableQueryAttributeValidation
	validateAttr := true
	if validateAttrFn != nil {
		validateAttr = validateAttrFn()
	}
	validAttr := sv.validSearchAttributes()
	for key, val := range fields {
		if validateAttr {
			// verify: key is whitelisted
			if !sv.isValidSearchAttributesKey(validAttr, key) {
				sv.logger.WithTags(tag.ESKey(key), tag.WorkflowDomainName(domain)).
					Error("invalid search attribute key")
				return &types.BadRequestError{Message: fmt.Sprintf("%s is not a valid search attribute key", key)}
			}
			// verify: value has the correct type
			if !sv.isValidSearchAttributesValue(validAttr, key, val) {
				sv.logger.WithTags(tag.ESKey(key), tag.ESValue(val), tag.WorkflowDomainName(domain)).
					Error("invalid search attribute value")
				return &types.BadRequestError{Message: fmt.Sprintf("%s is not a valid search attribute value for key %s", val, key)}
			}
		}
		// verify: key is not system reserved
		if definition.IsSystemIndexedKey(key) {
			sv.logger.WithTags(tag.ESKey(key), tag.WorkflowDomainName(domain)).
				Error("illegal update of system reserved attribute")
			return &types.BadRequestError{Message: fmt.Sprintf("%s is read-only Cadence reservered attribute", key)}
		}
		// verify: size of single value <= limit
		if len(val) > sv.searchAttributesSizeOfValueLimit(domain) {
			sv.logger.WithTags(tag.ESKey(key), tag.Number(int64(len(val))), tag.WorkflowDomainName(domain)).
				Error("value size of search attribute exceed limit")
			return &types.BadRequestError{Message: fmt.Sprintf("size limit exceed for key %s", key)}
		}
		totalSize += len(key) + len(val)
	}

	// verify: total size <= limit
	if totalSize > sv.searchAttributesTotalSizeLimit(domain) {
		sv.logger.WithTags(tag.Number(int64(totalSize)), tag.WorkflowDomainName(domain)).
			Error("total size of search attributes exceed limit")
		return &types.BadRequestError{Message: fmt.Sprintf("total size %d exceed limit", totalSize)}
	}

	return nil
}

// isValidSearchAttributesKey return true if key is registered
func (sv *SearchAttributesValidator) isValidSearchAttributesKey(
	validAttr map[string]interface{},
	key string,
) bool {
	_, isValidKey := validAttr[key]
	return isValidKey
}

// isValidSearchAttributesValue return true if value has the correct representation for the attribute key
func (sv *SearchAttributesValidator) isValidSearchAttributesValue(
	validAttr map[string]interface{},
	key string,
	value []byte,
) bool {
	valueType := common.ConvertIndexedValueTypeToInternalType(validAttr[key], sv.logger)
	_, err := common.DeserializeSearchAttributeValue(value, valueType)
	return err == nil
}
