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

// Package yaml provides a lazy YAML unmarshaler that defers decoding until
// the eventual target type is known. It has no dependencies beyond yaml.v2,
// so it can be imported by leaf config packages (e.g.
// common/dynamicconfig/openfeatureclient/config) that must stay
// import-cycle-free with respect to common/config.
package yaml

import (
	"fmt"

	yamlv2 "gopkg.in/yaml.v2" // CAUTION: go.uber.org/config does not support yaml.v3
)

// Node is a lazy-unmarshaler, because *yaml.Node only exists in gopkg.in/yaml.v3, not v2,
// and go.uber.org/config currently uses only v2.
type Node struct {
	unmarshal func(out any) error
}

var _ yamlv2.Unmarshaler = (*Node)(nil)

func (n *Node) UnmarshalYAML(unmarshal func(interface{}) error) error {
	n.unmarshal = unmarshal
	return nil
}

func (n *Node) Decode(out any) error {
	if n == nil {
		return nil
	}
	return n.unmarshal(out)
}

// ToNode is a bit of a hack to get a *yaml.Node for config-parsing compatibility purposes.
// There is probably a better way to achieve this with yaml-loading compatibility, but this is at least fairly simple.
func ToNode(input any) (*Node, error) {
	data, err := yamlv2.Marshal(input)
	if err != nil {
		// should be extremely unlikely, unless yaml marshaling is customized
		return nil, fmt.Errorf("could not serialize data to yaml: %w", err)
	}
	var out *Node
	err = yamlv2.Unmarshal(data, &out)
	if err != nil {
		// should not be possible
		return nil, fmt.Errorf("could not deserialize to yaml node: %w", err)
	}
	return out, nil
}
