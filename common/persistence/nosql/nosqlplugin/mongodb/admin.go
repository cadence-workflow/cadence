// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
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
	"io/ioutil"

	"go.mongodb.org/mongo-driver/bson"

	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

var _ nosqlplugin.AdminDB = (*mdb)(nil)

const (
	testSchemaDir = "schema/mongodb/"
)

func (db *mdb) SetupTestDatabase(schemaBaseDir string, replicas int) error {
	if schemaBaseDir == "" {
		var err error
		schemaBaseDir, err = nosqlplugin.GetDefaultTestSchemaDir(testSchemaDir)
		if err != nil {
			return err
		}
	}

	schemaFile := schemaBaseDir + "cadence/schema.json"
	byteValues, err := ioutil.ReadFile(schemaFile)
	if err != nil {
		return err
	}
	var commands []interface{}
	err = bson.UnmarshalExtJSON(byteValues, false, &commands)
	if err != nil {
		return err
	}
	for _, cmd := range commands {
		result := db.dbConn.RunCommand(context.Background(), cmd)
		if result.Err() != nil {
			return result.Err()
		}
	}
	return nil
}

func (db *mdb) TeardownTestDatabase() error {
	result := db.dbConn.RunCommand(context.Background(), bson.D{{"dropDatabase", 1}})
	err := result.Err()
	return err
}
