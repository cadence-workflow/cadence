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

package testflags

import (
	"fmt"
	"os"
	"testing"
)

// Pass these testflags in as environment variables in order to run tests marked as needing an
// external dependency.
// Tests can be marked as having an external dependency by calling the Require* functions below.
// In order to start external dependencies, see documentation at https://github.com/uber/cadence

var (
	cassandra = "CASSANDRA"
	mongodb   = "MONGODB"
	mysql     = "MYSQL"
	postgres  = "POSTGRES"
)

// NOTE: We are using environment variables instead of go testflags or go build directives, because we:
// 1) Want tests to be built all the time (go build directives don't give us that)
// 2) Want tests to be marked as skipped (go build directives don't give us that)
// 3) Want to be able to run individual tests (go testflags errors if the test doesn't import
// something that defines the testflags.)

func RequireMySQL(t *testing.T) {
	require(t, mysql)
}

func RequirePostgres(t *testing.T) {
	require(t, postgres)
}

func RequireMongoDB(t *testing.T) {
	require(t, mongodb)
}

func RequireCassandra(t *testing.T) {
	require(t, cassandra)
}

func require(t *testing.T, name string) {
	if !checkEnv(name) {
		t.Skip(fmt.Sprintf("Skipping test that requires %s to run - start %s and set '%s=1' environment variable to run this test.",
			name, name, name))
	}
}

func checkEnv(name string) bool {
	return os.Getenv(name) != ""
}
