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

package filestore

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
)

var _ dynamicconfig.Client = (*fileBasedClient)(nil)

const (
	minPollInterval = time.Second * 5
	fileMode        = 0644 // used for update config file
)

type constrainedValue struct {
	Value       interface{}
	Constraints map[string]interface{}
}

// FileBasedClientConfig is the config for the file based dynamic config client.
// It specifies where the config file is stored and how often the config should be
// updated by checking the config file again.
type FileBasedClientConfig struct {
	Filepath     string        `yaml:"filepath"`
	PollInterval time.Duration `yaml:"pollInterval"`
}

type fileBasedClient struct {
	values          atomic.Value
	lastUpdatedTime time.Time
	config          *FileBasedClientConfig
	doneCh          chan struct{}
	logger          log.Logger
}

// NewFileBasedClient creates a file based client.
func NewFileBasedClient(config *FileBasedClientConfig, logger log.Logger, doneCh chan struct{}) (dynamicconfig.Client, error) {
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	client := &fileBasedClient{
		config: config,
		doneCh: doneCh,
		logger: logger,
	}
	if err := client.update(); err != nil {
		return nil, err
	}
	go func() {
		ticker := time.NewTicker(client.config.PollInterval)
		for {
			select {
			case <-ticker.C:
				err := client.update()
				if err != nil {
					client.logger.Error("Failed to update dynamic config", tag.Error(err))
				}
			case <-client.doneCh:
				ticker.Stop()
				return
			}
		}
	}()
	return client, nil
}

func (fc *fileBasedClient) GetValue(name dynamicconfig.Key) (interface{}, error) {
	return fc.getValueWithFilters(name, nil, name.DefaultValue())
}

func (fc *fileBasedClient) GetValueWithFilters(name dynamicconfig.Key, filters map[dynamicconfig.Filter]interface{}) (interface{}, error) {
	return fc.getValueWithFilters(name, filters, name.DefaultValue())
}

func (fc *fileBasedClient) GetIntValue(name dynamicconfig.IntKey, filters map[dynamicconfig.Filter]interface{}) (int, error) {
	defaultValue := name.DefaultInt()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if intVal, ok := val.(int); ok {
		return intVal, nil
	}
	return defaultValue, fmt.Errorf("value type is not int but is: %T", val)
}

func (fc *fileBasedClient) GetFloatValue(name dynamicconfig.FloatKey, filters map[dynamicconfig.Filter]interface{}) (float64, error) {
	defaultValue := name.DefaultFloat()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if floatVal, ok := val.(float64); ok {
		return floatVal, nil
	} else if intVal, ok := val.(int); ok {
		return float64(intVal), nil
	}
	return defaultValue, fmt.Errorf("value type is not float64 but is: %T", val)
}

func (fc *fileBasedClient) GetBoolValue(name dynamicconfig.BoolKey, filters map[dynamicconfig.Filter]interface{}) (bool, error) {
	defaultValue := name.DefaultBool()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if boolVal, ok := val.(bool); ok {
		return boolVal, nil
	}
	return defaultValue, fmt.Errorf("value type is not bool but is: %T", val)
}

func (fc *fileBasedClient) GetStringValue(name dynamicconfig.StringKey, filters map[dynamicconfig.Filter]interface{}) (string, error) {
	defaultValue := name.DefaultString()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	if stringVal, ok := val.(string); ok {
		return stringVal, nil
	}
	return defaultValue, fmt.Errorf("value type is not string but is: %T", val)
}

func (fc *fileBasedClient) GetMapValue(name dynamicconfig.MapKey, filters map[dynamicconfig.Filter]interface{}) (map[string]interface{}, error) {
	defaultValue := name.DefaultMap()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}
	if mapVal, ok := val.(map[string]interface{}); ok {
		return mapVal, nil
	}
	return defaultValue, fmt.Errorf("value type is not map but is: %T", val)
}

func (fc *fileBasedClient) GetDurationValue(name dynamicconfig.DurationKey, filters map[dynamicconfig.Filter]interface{}) (time.Duration, error) {
	defaultValue := name.DefaultDuration()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}

	durationString, ok := val.(string)
	if !ok {
		return defaultValue, fmt.Errorf("value type is not string but is: %T", val)
	}

	durationVal, err := time.ParseDuration(durationString)
	if err != nil {
		return defaultValue, fmt.Errorf("failed to parse duration: %v", err)
	}
	return durationVal, nil
}

func (fc *fileBasedClient) GetListValue(name dynamicconfig.ListKey, filters map[dynamicconfig.Filter]interface{}) ([]interface{}, error) {
	defaultValue := name.DefaultList()
	val, err := fc.getValueWithFilters(name, filters, defaultValue)
	if err != nil {
		return defaultValue, err
	}
	if listVal, ok := val.([]interface{}); ok {
		return listVal, nil
	}
	return defaultValue, fmt.Errorf("value type is not list but is: %T", val)
}

func (fc *fileBasedClient) UpdateValue(name dynamicconfig.Key, value interface{}) error {
	if err := dynamicconfig.ValidateKeyValuePair(name, value); err != nil {
		return err
	}
	keyName := name.String()
	currentValues := make(map[string][]*constrainedValue)

	confContent, err := ioutil.ReadFile(fc.config.Filepath)
	if err != nil {
		return fmt.Errorf("failed to read dynamic config file %v: %v", fc.config.Filepath, err)
	}

	if err = yaml.Unmarshal(confContent, currentValues); err != nil {
		return fmt.Errorf("failed to decode dynamic config %v", err)
	}

	cVal := &constrainedValue{
		Value: value,
	}
	currentValues[keyName] = []*constrainedValue{cVal}
	newBytes, _ := yaml.Marshal(currentValues)

	err = ioutil.WriteFile(fc.config.Filepath, newBytes, fileMode)
	if err != nil {
		return fmt.Errorf("failed to write config file, err: %v", err)
	}

	return fc.storeValues(currentValues)
}

func (fc *fileBasedClient) RestoreValue(name dynamicconfig.Key, filters map[dynamicconfig.Filter]interface{}) error {
	return errors.New("not supported for file based client")
}

func (fc *fileBasedClient) ListValue(name dynamicconfig.Key) ([]*types.DynamicConfigEntry, error) {
	return nil, errors.New("not supported for file based client")
}

func (fc *fileBasedClient) update() error {
	defer func() {
		fc.lastUpdatedTime = time.Now()
	}()

	newValues := make(map[string][]*constrainedValue)

	info, err := os.Stat(fc.config.Filepath)
	if err != nil {
		return fmt.Errorf("failed to get status of dynamic config file: %v", err)
	}
	if !info.ModTime().After(fc.lastUpdatedTime) {
		return nil
	}

	confContent, err := ioutil.ReadFile(fc.config.Filepath)
	if err != nil {
		return fmt.Errorf("failed to read dynamic config file %v: %v", fc.config.Filepath, err)
	}

	if err = yaml.Unmarshal(confContent, newValues); err != nil {
		return fmt.Errorf("failed to decode dynamic config %v", err)
	}

	return fc.storeValues(newValues)
}

func (fc *fileBasedClient) storeValues(newValues map[string][]*constrainedValue) error {
	// yaml will unmarshal map into map[interface{}]interface{} instead of map[string]interface{}
	// manually convert key type to string for all values here
	// We don't need to convert constraints as their type can't be map. If user does use a map as filter
	// value, it won't match anyway.
	for _, s := range newValues {
		for _, cv := range s {
			var err error
			cv.Value, err = convertKeyTypeToString(cv.Value)
			if err != nil {
				return err
			}
		}
	}

	fc.values.Store(newValues)
	fc.logger.Info("Updated dynamic config")
	return nil
}

func (fc *fileBasedClient) getValueWithFilters(key dynamicconfig.Key, filters map[dynamicconfig.Filter]interface{}, defaultValue interface{}) (interface{}, error) {
	keyName := key.String()
	values := fc.values.Load().(map[string][]*constrainedValue)
	found := false
	for _, constrainedValue := range values[keyName] {
		if len(constrainedValue.Constraints) == 0 {
			// special handling for default value (value without any constraints)
			defaultValue = constrainedValue.Value
			found = true
			continue
		}
		if match(constrainedValue, filters) {
			return constrainedValue.Value, nil
		}
	}
	if !found {
		return defaultValue, dynamicconfig.NotFoundError
	}
	return defaultValue, nil
}

// match will return true if the constraints matches the filters or any subsets
func match(v *constrainedValue, filters map[dynamicconfig.Filter]interface{}) bool {
	if len(v.Constraints) > len(filters) {
		return false
	}

	for constrain, constrainedValue := range v.Constraints {
		constrainKey := dynamicconfig.ParseFilter(constrain)
		if filters[constrainKey] == nil || filters[constrainKey] != constrainedValue {
			return false
		}
	}
	return true
}

func convertKeyTypeToString(v interface{}) (interface{}, error) {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		return convertKeyTypeToStringMap(v)
	case []interface{}:
		return convertKeyTypeToStringSlice(v)
	default:
		return v, nil
	}
}

func convertKeyTypeToStringMap(m map[interface{}]interface{}) (map[string]interface{}, error) {
	stringKeyMap := make(map[string]interface{})
	for key, value := range m {
		stringKey, ok := key.(string)
		if !ok {
			return nil, fmt.Errorf("type of key %v is not string", key)
		}
		convertedValue, err := convertKeyTypeToString(value)
		if err != nil {
			return nil, err
		}
		stringKeyMap[stringKey] = convertedValue
	}
	return stringKeyMap, nil
}

func convertKeyTypeToStringSlice(s []interface{}) ([]interface{}, error) {
	stringKeySlice := make([]interface{}, len(s))
	for idx, value := range s {
		convertedValue, err := convertKeyTypeToString(value)
		if err != nil {
			return nil, err
		}
		stringKeySlice[idx] = convertedValue
	}
	return stringKeySlice, nil
}

func validateConfig(config *FileBasedClientConfig) error {
	if config == nil {
		return errors.New("no config found for file based dynamic config client")
	}
	if _, err := os.Stat(config.Filepath); err != nil {
		return fmt.Errorf("error checking dynamic config file at path %s, error: %v", config.Filepath, err)
	}

	// check if poll interval needs to be adjusted
	if config.PollInterval < minPollInterval {
		config.PollInterval = minPollInterval
	}
	return nil
}
