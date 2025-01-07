// Copyright (c) 2017 Uber Technologies, Inc.
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

//go:generate mockgen -package $GOPACKAGE -destination interfaces_mock.go -self_package github.com/uber/cadence/common/codec github.com/uber/cadence/common/codec BinaryEncoder

package codec

import (
	"go.uber.org/thriftrw/protocol/stream"
	"go.uber.org/thriftrw/wire"

	"github.com/uber/cadence/common/types"
)

type (
	// BinaryEncoder represent the encoder which can serialize or deserialize object
	BinaryEncoder interface {
		Encode(obj ThriftObject) ([]byte, error)
		Decode(payload []byte, val ThriftObject) error
	}

	// ThriftObject represents a thrift object
	ThriftObject interface {
		FromWire(w wire.Value) error
		ToWire() (wire.Value, error)
		Encode(stream.Writer) error
		Decode(stream.Reader) error
	}
)

const (
	// used by thriftrw binary codec
	preambleVersion0 byte = 0x59
)

var (
	// MissingBinaryEncodingVersion indicate that the encoding version is missing
	MissingBinaryEncodingVersion = &types.BadRequestError{Message: "Missing binary encoding version."}
	// InvalidBinaryEncodingVersion indicate that the encoding version is incorrect
	InvalidBinaryEncodingVersion = &types.BadRequestError{Message: "Invalid binary encoding version."}
	// MsgPayloadNotThriftEncoded indicate message is not thrift encoded
	MsgPayloadNotThriftEncoded = &types.BadRequestError{Message: "Message payload is not thrift encoded."}
)
