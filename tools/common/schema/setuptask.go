// Copyright (c) 2017 Uber Technologies, Inc.
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

package schema

import (
	"fmt"
	"os"

	"github.com/uber/cadence/common/log"
)

// SetupTask represents a task
// that sets up cassandra schema on
// a specified keyspace
type SetupTask struct {
	db     SchemaClient
	config *SetupConfig
	logger log.Logger
}

func newSetupSchemaTask(db SchemaClient, logger log.Logger, config *SetupConfig) *SetupTask {
	return &SetupTask{
		db:     db,
		config: config,
		logger: logger,
	}
}

// Run executes the task
func (task *SetupTask) Run() error {
	config := task.config
	task.logger.Info(fmt.Sprintf("Starting schema setup, config=%+v\n", config))

	if config.Overwrite {
		err := task.db.DropAllTables()
		if err != nil {
			return err
		}
	}

	if !config.DisableVersioning {
		task.logger.Info(fmt.Sprintf("Setting up version tables\n"))
		if err := task.db.CreateSchemaVersionTables(); err != nil {
			return err
		}
	}

	if len(config.SchemaFilePath) > 0 {
		file, err := os.Open(config.SchemaFilePath)
		if err != nil {
			return err
		}
		stmts, err := ParseFile(file)
		if err != nil {
			return err
		}

		task.logger.Info("----- Creating types and tables -----")
		for _, stmt := range stmts {
			task.logger.Info(rmspaceRegex.ReplaceAllString(stmt, " "))
			if err := task.db.ExecDDLQuery(stmt); err != nil {
				return err
			}
		}
		task.logger.Info("----- Done -----")
	}

	if !config.DisableVersioning {
		task.logger.Info(fmt.Sprintf("Setting initial schema version to %v\n", config.InitialVersion))
		err := task.db.UpdateSchemaVersion(config.InitialVersion, config.InitialVersion)
		if err != nil {
			return err
		}
		task.logger.Info("Updating schema update log\n")
		err = task.db.WriteSchemaUpdateLog("0", config.InitialVersion, "", "initial version")
		if err != nil {
			return err
		}
	}

	task.logger.Info("Schema setup complete")

	return nil
}
