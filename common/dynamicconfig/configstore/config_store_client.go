// Copyright (c) 2021 Uber Technologies, Inc.
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

package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	dc "github.com/uber/cadence/common/dynamicconfig"
	csc "github.com/uber/cadence/common/dynamicconfig/configstore/config"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql"
	"github.com/uber/cadence/common/persistence/sql"
	"github.com/uber/cadence/common/persistence/sql/sqlplugin"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination configstore_mock.go -self_package github.com/uber/cadence/common/dynamicconfig/configstore

var _ dc.Client = (*configStoreClient)(nil)

// Client is a stateful config store
type Client interface {
	common.Daemon
	dc.Client
}

const (
	configStoreMinPollInterval = time.Second * 2
)

var defaultConfigValues = &csc.ClientConfig{
	PollInterval:        time.Second * 10,
	UpdateRetryAttempts: 1,
	FetchTimeout:        2,
	UpdateTimeout:       2,
}

type configStoreClient struct {
	status             int32
	configStoreType    persistence.ConfigType
	values             atomic.Value
	lastUpdatedTime    time.Time
	config             *csc.ClientConfig
	configStoreManager persistence.ConfigStoreManager
	doneCh             chan struct{}
	logger             log.Logger
}

type cacheEntry struct {
	cacheVersion  int64
	schemaVersion int64
	dcEntries     map[string]*types.DynamicConfigEntry
}

// NewConfigStoreClient creates a config store client
func NewConfigStoreClient(
	clientCfg *csc.ClientConfig,
	persistenceCfg *config.Persistence,
	logger log.Logger,
	metricsClient metrics.Client,
	configType persistence.ConfigType,
) (Client, error) {
	if persistenceCfg == nil {
		return nil, errors.New("persistence cfg is nil")
	}

	ds, ok := persistenceCfg.DataStores[persistenceCfg.DefaultStore]
	if !ok {
		return nil, errors.New("default persistence config missing")
	}

	if err := validateClientConfig(clientCfg); err != nil {
		logger.Warn("invalid ClientConfig values, using default values")
		clientCfg = defaultConfigValues
	}

	client, err := newConfigStoreClient(clientCfg, &ds, logger, metricsClient, configType)
	if err != nil {
		return nil, err
	}
	err = client.startUpdate()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newConfigStoreClient(
	clientCfg *csc.ClientConfig,
	ds *config.DataStore,
	logger log.Logger,
	metricsClient metrics.Client,
	configType persistence.ConfigType,
) (*configStoreClient, error) {
	var store persistence.ConfigStore
	var err error
	switch {
	case ds.ShardedNoSQL != nil:
		store, err = nosql.NewNoSQLConfigStore(*ds.ShardedNoSQL, logger, metricsClient, nil)
	case ds.NoSQL != nil:
		store, err = nosql.NewNoSQLConfigStore(*ds.NoSQL.ConvertToShardedNoSQLConfig(), logger, metricsClient, nil)
	case ds.SQL != nil:
		var db sqlplugin.DB
		db, err = sql.NewSQLDB(ds.SQL)
		if err != nil {
			return nil, err
		}
		store, err = sql.NewSQLConfigStore(db, logger, nil)
	default:
		return nil, errors.New("both NoSQL and SQL store are not provided")
	}
	if err != nil {
		return nil, err
	}

	doneCh := make(chan struct{})
	client := &configStoreClient{
		status:             common.DaemonStatusStarted,
		config:             clientCfg,
		doneCh:             doneCh,
		configStoreManager: persistence.NewConfigStoreManagerImpl(store, logger),
		logger:             logger,
		configStoreType:    configType,
	}

	return client, nil
}

func (csc *configStoreClient) startUpdate() error {
	if err := csc.update(); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(csc.config.PollInterval)
		for {
			select {
			case <-ticker.C:
				err := csc.update()
				if err != nil {
					csc.logger.Error("Failed to update dynamic config", tag.Error(err))
				}
			case <-csc.doneCh:
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (csc *configStoreClient) GetValue(name dynamicproperties.Key) (interface{}, error) {
	return csc.getValueWithFilters(name, nil, name.DefaultValue())
}

func (csc *configStoreClient) GetValueWithFilters(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) (interface{}, error) {
	return csc.getValueWithFilters(name, filters, name.DefaultValue())
}

func (csc *configStoreClient) GetIntValue(name dynamicproperties.IntKey, filters map[dynamicproperties.Filter]interface{}) (int, error) {
	defaultValue := name.DefaultInt()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	floatVal, ok := val.(float64)
	if !ok {
		return defaultValue, errors.New("value type is not int")
	}

	if floatVal != math.Trunc(floatVal) {
		return defaultValue, errors.New("value type is not int")
	}

	return int(floatVal), nil
}

func (csc *configStoreClient) GetFloatValue(name dynamicproperties.FloatKey, filters map[dynamicproperties.Filter]interface{}) (float64, error) {
	defaultValue := name.DefaultFloat()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if floatVal, ok := val.(float64); ok {
		return floatVal, nil
	}
	return defaultValue, errors.New("value type is not float64")
}

func (csc *configStoreClient) GetBoolValue(name dynamicproperties.BoolKey, filters map[dynamicproperties.Filter]interface{}) (bool, error) {
	defaultValue := name.DefaultBool()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if boolVal, ok := val.(bool); ok {
		return boolVal, nil
	}
	return defaultValue, errors.New("value type is not bool")
}

func (csc *configStoreClient) GetStringValue(name dynamicproperties.StringKey, filters map[dynamicproperties.Filter]interface{}) (string, error) {
	defaultValue := name.DefaultString()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if stringVal, ok := val.(string); ok {
		return stringVal, nil
	}
	return defaultValue, errors.New("value type is not string")
}

// Note that all number types (ex: ints) will be returned as float64.
// It is the caller's responsibility to convert based on their context for value type.
func (csc *configStoreClient) GetMapValue(name dynamicproperties.MapKey, filters map[dynamicproperties.Filter]interface{}) (map[string]interface{}, error) {
	defaultValue := name.DefaultMap()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}
	if mapVal, ok := val.(map[string]interface{}); ok {
		return mapVal, nil
	}
	return defaultValue, errors.New("value type is not map")
}

func (csc *configStoreClient) GetDurationValue(name dynamicproperties.DurationKey, filters map[dynamicproperties.Filter]interface{}) (time.Duration, error) {
	defaultValue := name.DefaultDuration()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	var durVal time.Duration
	switch v := val.(type) {
	case string:
		durVal, err = time.ParseDuration(v)
		if err != nil {
			return defaultValue, errors.New("value string encoding cannot be parsed into duration")
		}
	case time.Duration:
		durVal = v
	default:
		return defaultValue, errors.New("value type is not duration")
	}

	return durVal, nil
}
func (csc *configStoreClient) GetListValue(name dynamicproperties.ListKey, filters map[dynamicproperties.Filter]interface{}) ([]interface{}, error) {
	defaultValue := name.DefaultList()
	val, err := csc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}
	if listVal, ok := val.([]interface{}); ok {
		return listVal, nil
	}
	return defaultValue, errors.New("value type is not list")
}

func (csc *configStoreClient) UpdateValue(name dynamicproperties.Key, value interface{}) error {
	dcValues, ok := value.([]*types.DynamicConfigValue)
	if !ok && value != nil {
		return errors.New("invalid value")
	}
	return csc.updateValue(name, dcValues, csc.config.UpdateRetryAttempts)
}

func (csc *configStoreClient) RestoreValue(name dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}) error {
	// if empty filter provided, update fallback value.
	// if u want to remove entire entry, just do update value with empty
	loaded := csc.values.Load()
	if loaded == nil {
		return dc.NotFoundError
	}
	currentCached := loaded.(cacheEntry)

	if currentCached.dcEntries == nil {
		return dc.NotFoundError
	}

	val, ok := currentCached.dcEntries[name.String()]
	if !ok {
		return dc.NotFoundError
	}

	newValues := make([]*types.DynamicConfigValue, 0, len(val.Values))
	if filters == nil {
		for _, dcValue := range val.Values {
			if dcValue.Filters != nil || len(dcValue.Filters) != 0 {
				newValues = append(newValues, dcValue.Copy())
			}
		}
	} else {
		for _, dcValue := range val.Values {
			if !matchFilters(dcValue, filters) || dcValue.Filters == nil || len(dcValue.Filters) == 0 {
				newValues = append(newValues, dcValue.Copy())
			}
		}
	}

	return csc.updateValue(name, newValues, csc.config.UpdateRetryAttempts)
}

func (csc *configStoreClient) ListValue(name dynamicproperties.Key) ([]*types.DynamicConfigEntry, error) {
	var resList []*types.DynamicConfigEntry

	loaded := csc.values.Load()
	if loaded == nil {
		return nil, nil
	}
	currentCached := loaded.(cacheEntry)

	if currentCached.dcEntries == nil {
		return nil, nil
	}
	listAll := false
	if name == nil {
		// if key is not specified, return all entries
		listAll = true
	} else if _, ok := currentCached.dcEntries[name.String()]; !ok {
		// if key is not known, return all entries
		listAll = true
	}
	if listAll {
		// if key is not known/specified, return all entries
		resList = make([]*types.DynamicConfigEntry, 0, len(currentCached.dcEntries))
		for _, entry := range currentCached.dcEntries {
			resList = append(resList, entry.Copy())
		}
	} else {
		// if key is known, return just that specific entry
		resList = make([]*types.DynamicConfigEntry, 0, 1)
		resList = append(resList, currentCached.dcEntries[name.String()])
	}

	return resList, nil
}

func (csc *configStoreClient) Stop() {
	if !atomic.CompareAndSwapInt32(&csc.status, common.DaemonStatusStarted, common.DaemonStatusStopped) {
		return
	}
	close(csc.doneCh)
	csc.configStoreManager.Close()
}

func (csc *configStoreClient) Start() {
	err := csc.startUpdate()
	if err != nil {
		csc.logger.Fatal("could not start config store", tag.Error(err))
	}
	if !atomic.CompareAndSwapInt32(&csc.status, common.DaemonStatusInitialized, common.DaemonStatusStarted) {
		return
	}
}

func (csc *configStoreClient) updateValue(name dynamicproperties.Key, dcValues []*types.DynamicConfigValue, retryAttempts int) error {
	// since values are not unique, no way to know if you are trying to update a specific value
	// or if you want to add another of the same value with different filters.
	// UpdateValue will replace everything associated with dc key.
	for _, dcValue := range dcValues {
		if err := validateKeyDataBlobPair(name, dcValue.Value); err != nil {
			return err
		}
	}
	loaded := csc.values.Load()
	var currentCached cacheEntry
	if loaded == nil {
		currentCached = cacheEntry{
			cacheVersion:  0,
			schemaVersion: 0,
			dcEntries:     map[string]*types.DynamicConfigEntry{},
		}
	} else {
		currentCached = loaded.(cacheEntry)
	}

	keyName := name.String()
	var newEntries []*types.DynamicConfigEntry

	existingEntry, entryExists := currentCached.dcEntries[keyName]

	if len(dcValues) == 0 {
		newEntries = make([]*types.DynamicConfigEntry, 0, len(currentCached.dcEntries))

		for _, entry := range currentCached.dcEntries {
			if entryExists && entry == existingEntry {
				continue
			} else {
				newEntries = append(newEntries, entry.Copy())
			}
		}
	} else {
		if entryExists {
			newEntries = make([]*types.DynamicConfigEntry, 0, len(currentCached.dcEntries))
		} else {
			newEntries = make([]*types.DynamicConfigEntry, 0, len(currentCached.dcEntries)+1)
			newEntries = append(newEntries,
				&types.DynamicConfigEntry{
					Name:   keyName,
					Values: dcValues,
				})
		}

		for _, entry := range currentCached.dcEntries {
			if entryExists && entry.Name == keyName {
				newEntries = append(newEntries,
					&types.DynamicConfigEntry{
						Name:   keyName,
						Values: dcValues,
					})
			} else {
				newEntries = append(newEntries, entry.Copy())
			}
		}
	}

	newSnapshot := &persistence.DynamicConfigSnapshot{
		Version: currentCached.cacheVersion + 1,
		Values: &types.DynamicConfigBlob{
			SchemaVersion: currentCached.schemaVersion,
			Entries:       newEntries,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), csc.config.UpdateTimeout)
	defer cancel()

	err := csc.configStoreManager.UpdateDynamicConfig(
		ctx,
		&persistence.UpdateDynamicConfigRequest{
			Snapshot: newSnapshot,
		}, csc.configStoreType,
	)

	select {
	case <-ctx.Done():
		// potentially we can retry on timeout
		return errors.New("timeout error on update")
	default:
		if err != nil {
			if _, ok := err.(*persistence.ConditionFailedError); ok && retryAttempts > 0 {
				// fetch new config and retry
				err := csc.update()
				if err != nil {
					return err
				}
				return csc.updateValue(name, dcValues, retryAttempts-1)
			}

			if retryAttempts == 0 {
				return errors.New("ran out of retry attempts on update")
			}
			return err
		}
		return nil
	}
}

func (csc *configStoreClient) update() error {
	ctx, cancel := context.WithTimeout(context.Background(), csc.config.FetchTimeout)
	defer cancel()

	res, err := csc.configStoreManager.FetchDynamicConfig(ctx, csc.configStoreType)

	select {
	case <-ctx.Done():
		return errors.New("timeout error on fetch")
	default:
		if err != nil {
			return fmt.Errorf("failed to fetch dynamic config snapshot %v", err)
		}

		if res != nil && res.Snapshot != nil {
			defer func() {
				csc.lastUpdatedTime = time.Now()
			}()

			return csc.storeValues(res.Snapshot)
		}
	}
	return nil
}

func (csc *configStoreClient) storeValues(snapshot *persistence.DynamicConfigSnapshot) error {
	// Converting the list of dynamic config entries into a map for better lookup performance
	var dcEntryMap map[string]*types.DynamicConfigEntry
	if snapshot.Values.Entries == nil {
		dcEntryMap = nil
	} else {
		dcEntryMap = make(map[string]*types.DynamicConfigEntry)
		for _, entry := range snapshot.Values.Entries {
			dcEntryMap[entry.Name] = entry
		}
	}

	csc.values.Store(cacheEntry{
		cacheVersion:  snapshot.Version,
		schemaVersion: snapshot.Values.SchemaVersion,
		dcEntries:     dcEntryMap,
	})
	csc.logger.Debug("Updated dynamic config")
	return nil
}

func (csc *configStoreClient) getValueWithFilters(key dynamicproperties.Key, filters map[dynamicproperties.Filter]interface{}, defaultValue interface{}) (interface{}, error) {
	keyName := key.String()
	loaded := csc.values.Load()
	if loaded == nil {
		return defaultValue, nil
	}
	cached := loaded.(cacheEntry)
	found := false

	if entry, ok := cached.dcEntries[keyName]; ok && entry != nil {
		for _, dcValue := range entry.Values {
			if len(dcValue.Filters) == 0 {
				parsedVal, err := convertFromDataBlob(dcValue.Value)

				if err == nil {
					defaultValue = parsedVal
					found = true
				}
				continue
			}

			if matchFilters(dcValue, filters) {
				return convertFromDataBlob(dcValue.Value)
			}
		}
	}
	if found {
		return defaultValue, nil
	}
	return defaultValue, dc.NotFoundError
}

func matchFilters(dcValue *types.DynamicConfigValue, filters map[dynamicproperties.Filter]interface{}) bool {
	if len(dcValue.Filters) > len(filters) {
		return false
	}

	for _, valueFilter := range dcValue.Filters {
		filterKey := dynamicproperties.ParseFilter(valueFilter.Name)
		if filters[filterKey] == nil {
			return false
		}

		requestValue, err := convertFromDataBlob(valueFilter.Value)
		if err != nil || filters[filterKey] != requestValue {
			return false
		}
	}
	return true
}

func validateClientConfig(config *csc.ClientConfig) error {
	if config == nil {
		return errors.New("no config found for config store based dynamic config client")
	}
	if config.PollInterval < configStoreMinPollInterval {
		return fmt.Errorf("poll interval should be at least %v", configStoreMinPollInterval)
	}
	if config.UpdateRetryAttempts < 0 {
		return errors.New("UpdateRetryAttempts must be non-negative")
	}
	if config.FetchTimeout <= 0 {
		return errors.New("FetchTimeout must be positive")
	}
	if config.UpdateTimeout <= 0 {
		return errors.New("UpdateTimeout must be positive")
	}
	return nil
}

func convertFromDataBlob(blob *types.DataBlob) (interface{}, error) {
	switch *blob.EncodingType {
	case types.EncodingTypeJSON:
		var v interface{}
		err := json.Unmarshal(blob.Data, &v)
		return v, err
	default:
		return nil, errors.New("unsupported blob encoding")
	}
}

func validateKeyDataBlobPair(key dynamicproperties.Key, blob *types.DataBlob) error {
	value, err := convertFromDataBlob(blob)
	if err != nil {
		return err
	}
	err = fmt.Errorf("key value pair mismatch, key type: %T, value type: %T", key, value)
	switch key.(type) {
	case dynamicproperties.IntKey:
		if _, ok := value.(int); !ok {
			floatVal, ok := value.(float64)
			if !ok { // int can be decoded as float64
				return err
			}
			if floatVal != math.Trunc(floatVal) {
				return errors.New("value type is not int")
			}
		}
	case dynamicproperties.BoolKey:
		if _, ok := value.(bool); !ok {
			return err
		}
	case dynamicproperties.FloatKey:
		if _, ok := value.(float64); !ok {
			return err
		}
	case dynamicproperties.StringKey:
		if _, ok := value.(string); !ok {
			return err
		}
	case dynamicproperties.DurationKey:
		if _, ok := value.(time.Duration); !ok {
			durationStr, ok := value.(string)
			if !ok {
				return err
			}
			if _, err = time.ParseDuration(durationStr); err != nil {
				return errors.New("value string encoding cannot be parsed into duration")
			}
		}
	case dynamicproperties.MapKey:
		if _, ok := value.(map[string]interface{}); !ok {
			return err
		}
	case dynamicproperties.ListKey:
		if _, ok := value.([]interface{}); !ok {
			return err
		}
	default:
		return fmt.Errorf("unknown key type: %T", key)
	}
	return nil
}
