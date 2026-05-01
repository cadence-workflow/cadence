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

package clitest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func TestNewCLIContext(t *testing.T) {
	app := cli.NewApp()
	ctx := NewCLIContext(
		t,
		app,
		StringArgument("region", "dca"),
		IntArgument("shards", 1024),
		BoolArgument("verbose", true),
		BoolArgument("exit-if-error", false),
		Int64Argument("bytes-per-minute", 999876543210),
		StringSliceArgument("tags", "tag1", "tag2"),
	)

	assert.True(t, ctx.IsSet("region"))
	assert.Equal(t, "dca", ctx.String("region"))

	assert.True(t, ctx.IsSet("shards"))
	assert.Equal(t, 1024, ctx.Int("shards"))

	assert.True(t, ctx.IsSet("verbose"))
	assert.True(t, ctx.Bool("verbose"))

	assert.True(t, ctx.IsSet("exit-if-error"))
	assert.False(t, ctx.Bool("exit-if-error"))

	assert.True(t, ctx.IsSet("bytes-per-minute"))
	assert.Equal(t, int64(999876543210), ctx.Int64("bytes-per-minute"))

	assert.True(t, ctx.IsSet("tags"))
	assert.Equal(t, []string{"tag1", "tag2"}, ctx.StringSlice("tags"))

	assert.False(t, ctx.IsSet("should-not-exist"))
}
