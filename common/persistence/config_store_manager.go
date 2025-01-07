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

package persistence

import (
	"context"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
)

type (

	// configStoreManagerImpl implements ConfigStoreManager based on ConfigStore and PayloadSerializer
	configStoreManagerImpl struct {
		serializer  PayloadSerializer
		persistence ConfigStore
		logger      log.Logger
	}
)

var _ ConfigStoreManager = (*configStoreManagerImpl)(nil)

// NewConfigStoreManagerImpl returns new ConfigStoreManager
func NewConfigStoreManagerImpl(persistence ConfigStore, logger log.Logger) ConfigStoreManager {
	return &configStoreManagerImpl{
		serializer:  NewPayloadSerializer(),
		persistence: persistence,
		logger:      logger,
	}
}

func (m *configStoreManagerImpl) Close() {
	m.persistence.Close()
}

func (m *configStoreManagerImpl) FetchDynamicConfig(ctx context.Context, cfgType ConfigType) (*FetchDynamicConfigResponse, error) {
	values, err := m.persistence.FetchConfig(ctx, cfgType)
	if err != nil || values == nil {
		return nil, err
	}

	config, err := m.serializer.DeserializeDynamicConfigBlob(values.Values)
	if err != nil {
		return nil, err
	}

	return &FetchDynamicConfigResponse{Snapshot: &DynamicConfigSnapshot{
		Version: values.Version,
		Values:  config,
	}}, nil
}

func (m *configStoreManagerImpl) UpdateDynamicConfig(ctx context.Context, request *UpdateDynamicConfigRequest, cfgType ConfigType) error {
	blob, err := m.serializer.SerializeDynamicConfigBlob(request.Snapshot.Values, common.EncodingTypeThriftRW)
	if err != nil {
		return err
	}

	entry := &InternalConfigStoreEntry{
		RowType:   int(cfgType),
		Version:   request.Snapshot.Version,
		Timestamp: time.Now(),
		Values:    blob,
	}

	return m.persistence.UpdateConfig(ctx, entry)
}
