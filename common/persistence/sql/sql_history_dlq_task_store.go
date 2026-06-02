// Copyright (c) 2025 Uber Technologies, Inc.
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

package sql

import (
	"context"
	"errors"

	p "github.com/uber/cadence/common/persistence"
)

var errHistoryDLQNotImplemented = errors.New("history task DLQ not implemented for SQL")

type sqlHistoryDLQTaskStore struct{}

func (s *sqlHistoryDLQTaskStore) GetName() string { return "sql" }
func (s *sqlHistoryDLQTaskStore) Close()          {}

func (s *sqlHistoryDLQTaskStore) CreateHistoryDLQTask(_ context.Context, _ p.InternalCreateHistoryDLQTaskRequest) error {
	return errHistoryDLQNotImplemented
}

func (s *sqlHistoryDLQTaskStore) GetHistoryDLQTasks(_ context.Context, _ p.HistoryDLQGetTasksRequest) (p.InternalGetHistoryDLQTasksResponse, error) {
	return p.InternalGetHistoryDLQTasksResponse{}, errHistoryDLQNotImplemented
}

func (s *sqlHistoryDLQTaskStore) RangeDeleteHistoryDLQTasks(_ context.Context, _ p.HistoryDLQDeleteTasksRequest) error {
	return errHistoryDLQNotImplemented
}

func (s *sqlHistoryDLQTaskStore) GetHistoryDLQAckLevels(_ context.Context, _ p.HistoryDLQGetAckLevelsRequest) (p.InternalGetHistoryDLQAckLevelsResponse, error) {
	return p.InternalGetHistoryDLQAckLevelsResponse{}, errHistoryDLQNotImplemented
}

func (s *sqlHistoryDLQTaskStore) UpdateHistoryDLQAckLevel(_ context.Context, _ p.InternalUpdateHistoryDLQAckLevelRequest) error {
	return errHistoryDLQNotImplemented
}
