// Copyright (c) 2020 Uber Technologies, Inc.
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

package cassandra

import (
	"context"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

func (db *cdb) InsertConfig(ctx context.Context, row *persistence.InternalConfigStoreEntry) error {
	query := db.session.Query(templateInsertConfig, row.RowType, row.Version, row.Timestamp, row.Values.Data, row.Values.Encoding).WithContext(ctx)
	applied, err := query.MapScanCAS(make(map[string]interface{}))
	if err != nil {
		return err
	}
	if !applied {
		return nosqlplugin.NewConditionFailure("InsertConfig operation failed because of version collision")
	}
	return nil
}

func (db *cdb) SelectLatestConfig(ctx context.Context, rowType int) (*persistence.InternalConfigStoreEntry, error) {
	var version int64
	var timestamp time.Time
	var data []byte
	var encoding common.EncodingType

	query := db.session.Query(templateSelectLatestConfig, rowType).WithContext(ctx)
	err := query.Scan(&rowType, &version, &timestamp, &data, &encoding)
	if err != nil {
		return nil, err
	}

	return &persistence.InternalConfigStoreEntry{
		RowType:   rowType,
		Version:   version,
		Timestamp: timestamp,
		Values: &persistence.DataBlob{
			Data:     data,
			Encoding: encoding,
		},
	}, err
}
