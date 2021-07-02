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

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	templateSelectLatestConfig = `SELECT * FROM clusterConfig WHERE row_type = 'dynamic_config' LIMIT 1;`

	templateInsertConfig = `INSERT INTO clusterConfig (row_type, version, timestamp, values, encoding) VALUES (?, ?, ?, ?, ?) IF NOT EXISTS;`
	//for version value, put x + 1 where x is the cached copy version.
)

func (db *cdb) InsertConfig(ctx context.Context, row *nosqlplugin.ConfigStoreRow) error {
	return nil
}

func (db *cdb) SelectLatestConfig(ctx context.Context) (*nosqlplugin.ConfigStoreRow, error) {
	return nil, nil
}
