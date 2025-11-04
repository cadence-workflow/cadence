// Copyright (c) 2019 Uber Technologies, Inc.
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

//go:build tools
// +build tools

package tools

import (
	// forces one import-order pattern
	_ "github.com/daixiang0/gci"
	// enumer for generating utility methods for const enums
	_ "github.com/dmarkham/enumer"
	// protobuf stuff
	_ "github.com/gogo/protobuf/protoc-gen-gofast"
	// gowrap for generating decorators for interface
	_ "github.com/hexdigest/gowrap"
	// replaces golint - configurable and much faster
	_ "github.com/mgechev/revive"
	// mockery for generating mocks
	_ "github.com/vektra/mockery/v2"
	// mockgen for generating mocks
	_ "go.uber.org/mock/mockgen"
	// nilaway for nil pointer analysis
	_ "go.uber.org/nilaway"
	// thriftrw code gen
	_ "go.uber.org/thriftrw"
	_ "go.uber.org/yarpc/encoding/protobuf/protoc-gen-yarpc-go"
	// yarpc plugin for thriftrw code gen
	_ "go.uber.org/yarpc/encoding/thrift/thriftrw-plugin-yarpc"
	// removes unused imports and formats
	_ "golang.org/x/tools/cmd/goimports"
)
