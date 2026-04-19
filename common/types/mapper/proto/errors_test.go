// Copyright (c) 2021 Uber Technologies Inc.
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

package proto

import (
	"errors"
	"reflect"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/assert"
	"go.uber.org/yarpc/yarpcerrors"

	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/common/types/mapper/testutils"
	"github.com/uber/cadence/common/types/testdata"
)

func TestErrors(t *testing.T) {
	for _, err := range testdata.Errors {
		name := reflect.TypeOf(err).Elem().Name()
		t.Run(name, func(t *testing.T) {
			// Test that the mappings does not lose information
			assert.Equal(t, err, ToError(FromError(err)))
		})
	}
}

func TestNilMapsToOK(t *testing.T) {
	protoNoError := FromError(nil)
	assert.Equal(t, yarpcerrors.CodeOK, yarpcerrors.FromError(protoNoError).Code())
	assert.Nil(t, ToError(protoNoError))
}

func TestFromUnknownErrorMapsToUnknownError(t *testing.T) {
	err := errors.New("unknown error")
	protobufErr := FromError(err)
	assert.True(t, yarpcerrors.IsUnknown(protobufErr))

	clientErr := ToError(protobufErr)

	assert.True(t, yarpcerrors.IsUnknown(clientErr))
	assert.ErrorContains(t, clientErr, err.Error())
}

func TestToDeadlineExceededMapsToItself(t *testing.T) {
	timeout := yarpcerrors.DeadlineExceededErrorf("timeout")
	assert.Equal(t, timeout, ToError(timeout))
}

// RetryTaskV2ErrorFuzzer ensures StartEventID/StartEventVersion and
// EndEventID/EndEventVersion are either both nil or both non-nil.
// FromEventIDVersionPair returns nil if either field is nil, which would
// drop the non-nil partner on round-trip.
func RetryTaskV2ErrorFuzzer(e *types.RetryTaskV2Error, c fuzz.Continue) {
	c.FuzzNoCustom(e)
	if e.StartEventID == nil || e.StartEventVersion == nil {
		e.StartEventID = nil
		e.StartEventVersion = nil
	}
	if e.EndEventID == nil || e.EndEventVersion == nil {
		e.EndEventID = nil
		e.EndEventVersion = nil
	}
}

func TestErrorsFuzz(t *testing.T) {
	seed := time.Now().UnixNano()
	defer func() {
		if t.Failed() {
			t.Logf("fuzz seed: %v", seed)
		}
	}()

	for _, errTemplate := range testdata.Errors {
		errType := reflect.TypeOf(errTemplate).Elem()
		t.Run(errType.Name(), func(t *testing.T) {
			var customFuncs []interface{}
			if _, ok := errTemplate.(*types.RetryTaskV2Error); ok {
				customFuncs = []interface{}{RetryTaskV2ErrorFuzzer}
			}
			f := fuzz.NewWithSeed(seed).Funcs(customFuncs...)
			for i := 0; i < testutils.DefaultIterations; i++ {
				newErr := reflect.New(errType).Interface()
				f.Fuzz(newErr)
				assert.Equal(t, newErr, ToError(FromError(newErr.(error))), "iteration %d", i)
			}
		})
	}
}
