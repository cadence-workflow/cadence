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

package sqlite

// IsDupEntryError verify if the error is a duplicate entry error
func (mdb *DB) IsDupEntryError(err error) bool {
	// TODO implement me
	panic("implement me")
}

// IsNotFoundError verify if the error is a not found error
func (mdb *DB) IsNotFoundError(err error) bool {
	// TODO implement me
	panic("implement me")
}

// IsTimeoutError verify if the error is a timeout error
func (mdb *DB) IsTimeoutError(err error) bool {
	// TODO implement me
	panic("implement me")
}

// IsThrottlingError verify if the error is a throttling error
func (mdb *DB) IsThrottlingError(err error) bool {
	// TODO implement me
	panic("implement me")
}
