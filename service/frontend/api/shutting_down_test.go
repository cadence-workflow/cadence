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

package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShuttingDownError(t *testing.T) {
	wh, _ := setupMocksForWorkflowHandler(t)
	wh.Stop()

	// get all methods of Handler interface
	tt := reflect.TypeOf(struct{ Handler }{})
	methodNames := make(map[string]struct{})
	for i := 0; i < tt.NumMethod(); i++ {
		methodNames[tt.Method(i).Name] = struct{}{}
	}
	delete(methodNames, "GetClusterInfo")
	delete(methodNames, "Health")

	v := reflect.ValueOf(wh)
	for name := range methodNames {
		method := v.MethodByName(name)
		methodType := method.Type()
		if methodType.Kind() != reflect.Func {
			t.Fatalf("method: %s is not a function - %s", name, methodType.String())
		}
		if methodType.IsVariadic() {
			t.Fatalf("method: %s is variadic - %s", name, methodType.String())
		}
		if methodType.NumIn() < 1 || methodType.NumIn() > 2 {
			t.Fatalf("method: %s has wrong number of inputs - %s", name, methodType.String())
		}

		var results []reflect.Value
		if methodType.NumIn() == 1 {
			results = method.Call([]reflect.Value{reflect.ValueOf(context.Background())})
		} else {
			results = method.Call([]reflect.Value{reflect.ValueOf(context.Background()), reflect.Zero(methodType.In(1))})
		}
		if len(results) == 1 {
			err, ok := results[0].Interface().(error)
			if !ok {
				t.Fatalf("method: %s has wrong output type - %s", name, methodType.String())
			}
			assert.ErrorContains(t, err, "Shutting down")
		} else if len(results) == 2 {
			err, ok := results[1].Interface().(error)
			if !ok {
				t.Fatalf("method: %s has wrong output type - %s", name, methodType.String())
			}
			assert.ErrorContains(t, err, "Shutting down")
		} else {
			t.Fatalf("method: %s has wrong number of outputs - %s", name, methodType.String())
		}
	}
}
