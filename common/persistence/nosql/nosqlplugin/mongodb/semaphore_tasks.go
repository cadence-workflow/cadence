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

package mongodb

import (
	"context"
	"fmt"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func (db *mdb) SelectSemaphoreTaskControlRow(ctx context.Context, filter *nosqlplugin.SemaphoreTaskControlFilter) (*nosqlplugin.SemaphoreTaskControlRow, error) {
	return nil, fmt.Errorf("SelectSemaphoreTaskControlRow is not implemented")
}

func (db *mdb) InsertSemaphoreTaskControlRow(ctx context.Context, row *nosqlplugin.SemaphoreTaskControlRow) error {
	return fmt.Errorf("InsertSemaphoreTaskControlRow is not implemented")
}

func (db *mdb) UpdateSemaphoreTaskControlRow(ctx context.Context, row *nosqlplugin.SemaphoreTaskControlRow, previousRangeID int64) error {
	return fmt.Errorf("UpdateSemaphoreTaskControlRow is not implemented")
}

func (db *mdb) InsertSemaphoreTasks(ctx context.Context, tasks []*nosqlplugin.SemaphoreTaskRow, controlCondition *nosqlplugin.SemaphoreTaskControlRow) error {
	return fmt.Errorf("InsertSemaphoreTasks is not implemented")
}

func (db *mdb) SelectSemaphoreTasks(ctx context.Context, filter *nosqlplugin.SemaphoreTasksFilter) ([]*nosqlplugin.SemaphoreTaskRow, error) {
	return nil, fmt.Errorf("SelectSemaphoreTasks is not implemented")
}

func (db *mdb) RangeDeleteSemaphoreTasks(ctx context.Context, filter *nosqlplugin.SemaphoreTasksFilter) (int, error) {
	return 0, fmt.Errorf("RangeDeleteSemaphoreTasks is not implemented")
}

func (db *mdb) GetSemaphoreTasksCount(ctx context.Context, filter *nosqlplugin.SemaphoreTasksFilter) (int64, error) {
	return 0, fmt.Errorf("GetSemaphoreTasksCount is not implemented")
}
