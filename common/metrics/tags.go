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

package metrics

const (
	instanceTag    = "instance"
	domainTag      = "domain"
	domainAllValue = "all"
)

// Tag is an interface to define metrics tags
type Tag interface {
	Key() string
	Value() string
}

type _domainTag struct {
	value string
}

// DomainTag returns a new domain tag
func DomainTag(value string) Tag {
	return _domainTag{value}
}

// Key returns the key of the domain tag
func (d _domainTag) Key() string {
	return domainTag
}

// Value returns the value of a domain tag
func (d _domainTag) Value() string {
	return d.value
}

type _domainAllTag struct{}

// DomainAllTag returns a new domain all tag-value
func DomainAllTag() Tag {
	return _domainAllTag{}
}

// Key returns the key of the domain all tag
func (d _domainAllTag) Key() string {
	return domainTag
}

// Value returns the value of the domain all tag
func (d _domainAllTag) Value() string {
	return domainAllValue
}
