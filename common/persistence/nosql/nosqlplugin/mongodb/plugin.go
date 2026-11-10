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

package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	// PluginName is the name of the plugin
	PluginName = "mongodb"
)

type plugin struct{}

var _ nosqlplugin.Plugin = (*plugin)(nil)

func init() {
	nosql.RegisterPlugin(PluginName, &plugin{})
}

// CreateDB initialize the db object
func (p *plugin) CreateDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (nosqlplugin.DB, error) {
	return p.doCreateDB(cfg, logger)
}

func (p *plugin) SetupDB(cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (persistence.SetupDB, error) {
	// TODO implement me
	panic("implement me")
}

func (p *plugin) SchemaDB(dbType persistence.DBType, cfg *config.NoSQL, logger log.Logger, dc *persistence.DynamicConfiguration) (persistence.SchemaDB, error) {
	// TODO implement me
	panic("implement me")
}

func (p *plugin) doCreateDB(cfg *config.NoSQL, logger log.Logger) (*mdb, error) {
	uri := fmt.Sprintf("mongodb://%v:%v@%v:%v/", cfg.User, cfg.Password, cfg.Hosts, cfg.Port)
	// TODO CreateDB/CreateAdminDB don't pass in context.Context so we are using background for now
	// It's okay because this is being called during server startup or CLI.
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if cfg.Keyspace == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}
	db := client.Database(cfg.Keyspace)
	return &mdb{
		client: client,
		dbConn: db,
		cfg:    cfg,
		logger: logger,
	}, err
}
