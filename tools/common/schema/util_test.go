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

package schema

import (
	"io"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testdata = `
ALTER TYPE domain_config ADD isolation_groups blob;
ALTER TYPE domain_config ADD isolation_groups_encoding text;
-- a comment
ALTER TYPE domain_config2 ADD isolation_groups_encoding text;
`

type mockfile struct {
	read bool
}

func (m *mockfile) Stat() (fs.FileInfo, error) {
	return nil, nil
}
func (m *mockfile) Read(in []byte) (int, error) {
	if m.read {
		return 0, io.EOF
	}
	for i := 0; i < len(in) && i < len([]byte(testdata)); i++ {
		in[i] = []byte(testdata)[i]
	}
	m.read = true
	return len(in), nil
}
func (m *mockfile) Close() error {
	return nil
}

func TestParseFile(t *testing.T) {
	res, err := ParseFile(&mockfile{})
	assert.NoError(t, err)
	expectedOutput := []string{
		"ALTER TYPE domain_config ADD isolation_groups blob;",
		"ALTER TYPE domain_config ADD isolation_groups_encoding text;",
		"ALTER TYPE domain_config2 ADD isolation_groups_encoding text;",
	}
	assert.Equal(t, expectedOutput, res)
}
