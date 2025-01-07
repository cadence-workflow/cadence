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

package commoncli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintErr(t *testing.T) {
	run := func(t *testing.T, err error) string {
		t.Helper()
		buf := strings.Builder{}
		// improves t.Log display by placing it all on a common start column, and easier to read the Equal below
		buf.WriteString("\n")
		failed := printErr(err, &buf)
		assert.NoError(t, failed, "error during printing")
		t.Log(buf.String())
		return buf.String()
	}
	t.Run("printable only", func(t *testing.T) {
		str := run(t, Problem("a problem", nil))
		assert.Equal(t, `
Error: a problem
`, str)
	})
	t.Run("printable top", func(t *testing.T) {
		str := run(t,
			Problem("a problem",
				fmt.Errorf("wrapper: %w",
					errors.New("cause"))))
		// causes are nested flat, chains are cleaned up
		assert.Equal(t, `
Error: a problem
Error details:
  wrapper
  cause
`, str)
	})
	t.Run("printable top fancy middle", func(t *testing.T) {
		str := run(t,
			Problem("a problem",
				fmt.Errorf("wrapper caused by -> %w",
					errors.New("cause"))))
		// cleans up other kinds of suffixes
		assert.Equal(t, `
Error: a problem
Error details:
  wrapper caused by ->
  cause
`, str)
	})
	t.Run("printable top unfixable middle", func(t *testing.T) {
		str := run(t,
			Problem("a problem",
				fmt.Errorf("wrapper (caused by: %w)",
					errors.New("cause"))))
		// does not clean up something that isn't a suffix
		assert.Equal(t, `
Error: a problem
Error details:
  wrapper (caused by: cause)
  cause
`, str)
	})
	t.Run("printable bottom", func(t *testing.T) {
		str := run(t,
			fmt.Errorf("msg: %w",
				Problem("a problem", nil)))
		// note: the Problem is the displayed err, even though there is an error wrapper above it.
		// all contents are visible for troubleshooting purposes, it just tries to make the "problem" clearer.
		assert.Equal(t, `
Error: a problem
Error details:
  msg
  Error (above): a problem
`, str)
	})
	t.Run("printable mid", func(t *testing.T) {
		str := run(t,
			fmt.Errorf("msg: %w",
				Problem("one layer deep",
					errors.New("cause"))))
		assert.Equal(t, `
Error: one layer deep
Error details:
  msg
  Error (above): one layer deep
  Error details:
    cause
`, str)
	})
	t.Run("printable nested", func(t *testing.T) {
		str := run(t,
			Problem("top",
				fmt.Errorf("wrapper: %w",
					Problem("bottom",
						errors.New("cause")))))
		assert.Equal(t, `
Error: top
Error details:
  wrapper
  Error: bottom
  Error details:
    cause
`, str)
	})
	t.Run("multi-line error", func(t *testing.T) {
		str := run(t, fmt.Errorf(`what if
it has
multiple lines: %w`, errors.New(`even
when
nested`)))
		// maybe not ideal but it works well enough I think
		assert.Equal(t, `
Error: what if
it has
multiple lines
Error details:
  even
  when
  nested
`, str)
	})
}
