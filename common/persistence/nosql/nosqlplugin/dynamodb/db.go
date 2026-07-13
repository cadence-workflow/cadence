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

package dynamodb

import (
	"errors"
	"fmt"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
)

const (
	// PluginName is the name of the plugin
	PluginName = "dynamodb"
)

var (
	errConditionFailed = errors.New("internal condition fail error")
)

// ddb represents a logical connection to DynamoDB database
type ddb struct {
}

var _ nosqlplugin.DB = (*ddb)(nil)

// NewDynamoDB return a new DB
func NewDynamoDB(cfg config.NoSQL, logger log.Logger) (nosqlplugin.DB, error) {
	return nil, fmt.Errorf("TODO")
}

func (db *ddb) Close() {
	panic("TODO")
}

func (db *ddb) PluginName() string {
	return PluginName
}

func (db *ddb) IsNotFoundError(err error) bool {
	panic("TODO")
}

func (db *ddb) IsTimeoutError(err error) bool {
	panic("TODO")
}

func (db *ddb) IsThrottlingError(err error) bool {
	panic("TODO")
}

func (db *ddb) IsDBUnavailableError(err error) bool {
	panic("TODO")
}

func (db *ddb) IsConditionFailedError(err error) bool {
	return err == errConditionFailed
}

func (db *ddb) GetActivityMapDeleteResetThreshold() int { return 0 }

func (db *ddb) GetTimerMapDeleteResetThreshold() int { return 0 }
