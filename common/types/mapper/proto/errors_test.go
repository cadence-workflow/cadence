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

	"github.com/stretchr/testify/assert"
	"go.uber.org/yarpc/yarpcerrors"

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
