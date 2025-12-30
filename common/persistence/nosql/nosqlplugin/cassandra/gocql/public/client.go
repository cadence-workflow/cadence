// Copyright (c) 2017-2021 Uber Technologies, Inc.
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

package public

import (
	"context"
	"errors"
	"strings"

	gogocql "github.com/gocql/gocql"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

var _ gocql.Client = client{}

type (
	client struct {
	}
)

func init() {
	gocql.RegisterClient(client{})
}

func (c client) CreateSession(
	config gocql.ClusterConfig,
) (gocql.Session, error) {
	return gocql.NewSession(config)
}

func (c client) IsTimeoutError(err error) bool {
	if err == context.DeadlineExceeded {
		return true
	}
	if err == gogocql.ErrTimeoutNoResponse {
		return true
	}
	if err == gogocql.ErrConnectionClosed {
		return true
	}
	_, ok := err.(*gogocql.RequestErrWriteTimeout)
	return ok
}

func (c client) IsNotFoundError(err error) bool {
	return err == gogocql.ErrNotFound
}

func (c client) IsThrottlingError(err error) bool {
	if req, ok := err.(gogocql.RequestError); ok {
		// gocql does not expose the constant errOverloaded = 0x1001
		return req.Code() == 0x1001
	}
	return false
}

// IsDBUnavailableError checks if the error is a database unavailable error
// relating to issues like the database being overloaded or the consistency level not being achievable
// because a node is down.
func (c client) IsDBUnavailableError(err error) bool {
	var e gogocql.RequestError
	if errors.As(err, &e) {
		// 0x1000 == UNAVAILABLE
		// if this is not the expected error code, bail out early
		if e.Code() != 0x1000 {
			return false
		}
		// emit these errors in the condition that the database is in trouble
		// and, from an actionability standpoint, the problem points to something
		// at the database level that require resolution
		// https://github.com/apache/cassandra/blob/3c69bd23673caa40b22f3500a3e3eecabc25c8e5/src/java/org/apache/cassandra/locator/ReplicaPlans.java#L329
		if strings.Contains(e.Message(), "Cannot perform LWT operation") ||
			strings.Contains(strings.ToLower(e.Message()), "cannot achieve consistency level") {
			return true
		}
	}
	return false
}

func (c client) IsCassandraConsistencyError(err error) bool {
	if req, ok := err.(gogocql.RequestError); ok {
		// 0x1000 == UNAVAILABLE
		return req.Code() == 0x1000
	}
	return false
}
