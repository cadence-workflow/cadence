// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package sql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/serialization"
)

func TestNewFactory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	assert.NotNil(t, factory)
}

func TestFactoryNewTaskStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	taskStore, err := factory.NewTaskStore()
	assert.Nil(t, taskStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	taskStore, err = factory.NewTaskStore()
	assert.NotNil(t, taskStore)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewShardStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	shardStore, err := factory.NewShardStore()
	assert.Nil(t, shardStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	shardStore, err = factory.NewShardStore()
	assert.NotNil(t, shardStore)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewHistoryV2Store(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	historyV2Store, err := factory.NewHistoryStore()
	assert.Nil(t, historyV2Store)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	historyV2Store, err = factory.NewHistoryStore()
	assert.NotNil(t, historyV2Store)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewMetadataV2Store(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	metadataV2Store, err := factory.NewDomainStore()
	assert.Nil(t, metadataV2Store)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	metadataV2Store, err = factory.NewDomainStore()
	assert.NotNil(t, metadataV2Store)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewExecutionStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	executionStore, err := factory.NewExecutionStore(0)
	assert.Nil(t, executionStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	executionStore, err = factory.NewExecutionStore(0)
	assert.NotNil(t, executionStore)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewVisibilityStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	visibilityStore, err := factory.NewVisibilityStore(true)
	assert.Nil(t, visibilityStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	visibilityStore, err = factory.NewVisibilityStore(true)
	assert.NotNil(t, visibilityStore)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewQueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	queueStore, err := factory.NewQueue(persistence.DomainReplicationQueueType)
	assert.Nil(t, queueStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	queueStore, err = factory.NewQueue(persistence.DomainReplicationQueueType)
	assert.NotNil(t, queueStore)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewConfigStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	configStore, err := factory.NewConfigStore()
	assert.Nil(t, configStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	configStore, err = factory.NewConfigStore()
	assert.NotNil(t, configStore)
	assert.NoError(t, err)
	factory.Close()
}

func TestFactoryNewDomainAuditStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.SQL{}
	clusterName := "test"
	logger := testlogger.New(t)
	mockParser := serialization.NewMockParser(ctrl)
	dc := &persistence.DynamicConfiguration{}
	factory := NewFactory(cfg, clusterName, logger, mockParser, dc)
	domainAuditStore, err := factory.NewDomainAuditStore()
	assert.Nil(t, domainAuditStore)
	assert.Error(t, err)
	factory.Close()

	cfg.PluginName = "shared"
	factory = NewFactory(cfg, clusterName, logger, mockParser, dc)
	domainAuditStore, err = factory.NewDomainAuditStore()
	assert.NotNil(t, domainAuditStore)
	assert.NoError(t, err)
	factory.Close()
}
