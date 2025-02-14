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
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
)

// cdb represents a logical connection to Cassandra database
type cdb struct {
	logger  log.Logger
	client  gocql.Client
	session gocql.Session
	cfg     *config.NoSQL
	dc      *persistence.DynamicConfiguration
}

var _ nosqlplugin.DB = (*cdb)(nil)

// cassandraDBOption is used to provide optional settings for cdb object
type cassandraDBOption func(*cdb)

// dbWithClient returns a cdb option to set the gocql client.
// If this is not used then the globally registered client is used.
func dbWithClient(client gocql.Client) cassandraDBOption {
	return func(db *cdb) {
		db.client = client
	}
}

// newCassandraDBFromSession returns a DB from a session
func newCassandraDBFromSession(
	cfg *config.NoSQL,
	session gocql.Session,
	logger log.Logger,
	dc *persistence.DynamicConfiguration,
	opts ...cassandraDBOption,
) *cdb {
	res := &cdb{
		session: session,
		logger:  logger,
		cfg:     cfg,
		dc:      dc,
	}

	for _, opt := range opts {
		opt(res)
	}

	if res.client == nil {
		res.client = gocql.GetRegisteredClient()
	}

	return res
}

func (db *cdb) Close() {
	if db.session != nil {
		db.session.Close()
	}
}

func (db *cdb) PluginName() string {
	return PluginName
}

func (db *cdb) IsNotFoundError(err error) bool {
	return db.client.IsNotFoundError(err)
}

func (db *cdb) IsTimeoutError(err error) bool {
	return db.client.IsTimeoutError(err)
}

func (db *cdb) IsThrottlingError(err error) bool {
	return db.client.IsThrottlingError(err)
}

func (db *cdb) IsDBUnavailableError(err error) bool {
	return db.client.IsDBUnavailableError(err)
}

func (db *cdb) isCassandraConsistencyError(err error) bool {
	return db.client.IsCassandraConsistencyError(err)
}

func (db *cdb) executeWithConsistencyAll(q gocql.Query) error {
	if db.dc != nil && db.dc.EnableCassandraAllConsistencyLevelDelete() {
		if err := q.Consistency(cassandraAllConslevel).Exec(); err != nil {
			if db.isCassandraConsistencyError(err) {
				db.logger.Warn("unable to complete the delete operation due to consistency issue", tag.Error(err))
				return q.Consistency(cassandraDefaultConsLevel).Exec()
			}
			return err
		}
		return nil
	}
	return q.Exec()
}

func (db *cdb) executeBatchWithConsistencyAll(b gocql.Batch) error {
	if db.dc != nil && db.dc.EnableCassandraAllConsistencyLevelDelete() {
		if err := db.session.ExecuteBatch(b.Consistency(cassandraAllConslevel)); err != nil {
			if db.isCassandraConsistencyError(err) {
				db.logger.Warn("unable to complete the delete operation due to consistency issue", tag.Error(err))
				return db.session.ExecuteBatch(b.Consistency(cassandraDefaultConsLevel))
			}
			return err
		}
		return nil
	}
	return db.session.ExecuteBatch(b)
}
