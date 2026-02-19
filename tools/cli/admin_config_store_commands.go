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

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/tools/common/commoncli"
)

type cliEntry struct {
	Name         string
	DefaultValue interface{} `json:"defaultValue,omitempty"`
	Values       []*cliValue
}

type cliValue struct {
	Value   interface{}
	Filters []*cliFilter
}

type cliFilter struct {
	Name  string
	Value interface{}
}

// AdminGetDynamicConfig gets value of specified dynamic config parameter matching specified filter
func AdminGetDynamicConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	configName, err := getRequiredOption(c, FlagDynamicConfigName)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}

	filter := c.String(FlagDynamicConfigFilter)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	parsedFilters, err := parseInputFilter(filter)
	if err != nil {
		return commoncli.Problem("Failed to parse input filter array", err)
	}

	req := &types.GetDynamicConfigRequest{
		ConfigName: configName,
		Filters:    parsedFilters,
	}

	val, err := adminClient.GetDynamicConfig(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to get dynamic config value", err)
	}

	var umVal interface{}
	err = json.Unmarshal(val.Value.Data, &umVal)
	if err != nil {
		return commoncli.Problem("Failed to unmarshal response", err)
	}

	if umVal == nil {
		fmt.Printf("No values stored for specified dynamic config.\n")
	} else {
		prettyPrintJSONObject(getDeps(c).Output(), umVal)
	}

	return nil
}

// AdminUpdateDynamicConfig updates specified dynamic config parameter with specified values
func AdminUpdateDynamicConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	dcName, err := getRequiredOption(c, FlagDynamicConfigName)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	dcValuesRaw := c.StringSlice(FlagDynamicConfigValue)

	// WORKAROUND: urfave/cli v2 StringSliceFlag splits on commas by default
	// This breaks JSON values. Reassemble the split pieces.
	var dcValues []string
	if len(dcValuesRaw) > 0 {
		// Check if first value looks like it was split (starts with { but doesn't end with })
		if len(dcValuesRaw) > 1 && strings.HasPrefix(dcValuesRaw[0], "{") && !strings.HasSuffix(dcValuesRaw[len(dcValuesRaw)-1], "}") {
			// Likely split on commas, reassemble
			dcValues = []string{strings.Join(dcValuesRaw, ",")}
		} else if len(dcValuesRaw) > 1 && strings.HasPrefix(dcValuesRaw[0], "{") {
			// Check if it was split in the middle
			assembled := strings.Join(dcValuesRaw, ",")
			// Verify it's valid JSON now
			var test interface{}
			if json.Unmarshal([]byte(assembled), &test) == nil {
				dcValues = []string{assembled}
			} else {
				// Not split, use as-is
				dcValues = dcValuesRaw
			}
		} else {
			dcValues = dcValuesRaw
		}
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	var parsedValues []*types.DynamicConfigValue

	if dcValues != nil {
		parsedValues = make([]*types.DynamicConfigValue, 0, len(dcValues))

		for _, valueString := range dcValues {
			var parsedInputValue *cliValue
			err := json.Unmarshal([]byte(valueString), &parsedInputValue)
			if err != nil {
				return commoncli.Problem("Unable to unmarshal value to inputValue", err)
			}
			parsedValue, err := convertFromInputValue(parsedInputValue)
			if err != nil {
				return commoncli.Problem("Unable to convert from inputValue to DynamicConfigValue", err)
			}
			parsedValues = append(parsedValues, parsedValue)
		}
	} else {
		parsedValues = nil
	}

	req := &types.UpdateDynamicConfigRequest{
		ConfigName:   dcName,
		ConfigValues: parsedValues,
	}

	err = adminClient.UpdateDynamicConfig(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to update dynamic config value", err)
	}
	fmt.Printf("Dynamic Config %q updated with %s \n", dcName, dcValues)
	return nil
}

// AdminRestoreDynamicConfig removes values of specified dynamic config parameter matching specified filter
func AdminRestoreDynamicConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	dcName, err := getRequiredOption(c, FlagDynamicConfigName)
	if err != nil {
		return commoncli.Problem("Required flag not found", err)
	}
	filter := c.String(FlagDynamicConfigFilter)

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}

	parsedFilters, err := parseInputFilter(filter)
	if err != nil {
		return commoncli.Problem("Failed to parse input filter", err)
	}

	req := &types.RestoreDynamicConfigRequest{
		ConfigName: dcName,
		Filters:    parsedFilters,
	}

	err = adminClient.RestoreDynamicConfig(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to restore dynamic config value", err)
	}
	fmt.Printf("Dynamic Config %q restored\n", dcName)
	return nil
}

// AdminListDynamicConfig lists all values associated with specified dynamic config parameter or all values for all dc parameter if none is specified.
func AdminListDynamicConfig(c *cli.Context) error {
	adminClient, err := getDeps(c).ServerAdminClient(c)
	if err != nil {
		return err
	}

	ctx, cancel, err := newContext(c)
	defer cancel()
	if err != nil {
		return commoncli.Problem("Error in creating context: ", err)
	}
	req := &types.ListDynamicConfigRequest{
		ConfigName: "", // empty string means all config values
	}

	val, err := adminClient.ListDynamicConfig(ctx, req)
	if err != nil {
		return commoncli.Problem("Failed to list dynamic config value(s)", err)
	}

	if val == nil || val.Entries == nil || len(val.Entries) == 0 {
		fmt.Printf("No dynamic config values stored to list.\n")
	} else {
		cliEntries := make([]*cliEntry, 0, len(val.Entries))
		for _, dcEntry := range val.Entries {
			cliEntry, err := convertToInputEntry(dcEntry)
			if err != nil {
				fmt.Printf("Cannot parse list response.\n")
			}
			cliEntries = append(cliEntries, cliEntry)
		}
		prettyPrintJSONObject(getDeps(c).Output(), cliEntries)
	}
	return nil
}

// AdminListConfigKeys lists all available dynamic config keys with description and default value
func AdminListConfigKeys(c *cli.Context) error {

	type ConfigRow struct {
		Name        string      `header:"Name" json:"name"`
		Description string      `header:"Description" json:"description"`
		Default     interface{} `header:"Default value" json:"default"`
	}

	var rows []ConfigRow

	for name, k := range dynamicproperties.GetAllKeys() {
		rows = append(rows, ConfigRow{
			Name:        name,
			Description: k.Description(),
			Default:     k.DefaultValue(),
		})
	}
	// sorting config key names alphabetically
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})

	return Render(c, rows, RenderOptions{
		DefaultTemplate: templateTable,
		Color:           true,
		Border:          true,
		ColumnAlignment: []int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT}},
	)
}

func convertToInputEntry(dcEntry *types.DynamicConfigEntry) (*cliEntry, error) {
	newValues := make([]*cliValue, 0, len(dcEntry.Values))
	for _, value := range dcEntry.Values {
		newValue, err := convertToInputValue(value)
		if err != nil {
			return nil, err
		}
		newValues = append(newValues, newValue)
	}
	return &cliEntry{
		Name:   dcEntry.Name,
		Values: newValues,
	}, nil
}

func convertToInputValue(dcValue *types.DynamicConfigValue) (*cliValue, error) {
	newFilters := make([]*cliFilter, 0, len(dcValue.Filters))
	for _, filter := range dcValue.Filters {
		newFilter, err := convertToInputFilter(filter)
		if err != nil {
			return nil, err
		}
		newFilters = append(newFilters, newFilter)
	}

	var val interface{}
	err := json.Unmarshal(dcValue.Value.Data, &val)
	if err != nil {
		return nil, err
	}

	return &cliValue{
		Value:   val,
		Filters: newFilters,
	}, nil
}

func convertToInputFilter(dcFilter *types.DynamicConfigFilter) (*cliFilter, error) {
	var val interface{}
	err := json.Unmarshal(dcFilter.Value.Data, &val)
	if err != nil {
		return nil, err
	}

	return &cliFilter{
		Name:  dcFilter.Name,
		Value: val,
	}, nil
}

func convertFromInputValue(inputValue *cliValue) (*types.DynamicConfigValue, error) {
	encodedValue, err := json.Marshal(inputValue.Value)
	if err != nil {
		return nil, err
	}

	blob := &types.DataBlob{
		EncodingType: types.EncodingTypeJSON.Ptr(),
		Data:         encodedValue,
	}

	dcFilters := make([]*types.DynamicConfigFilter, 0, len(inputValue.Filters))
	for _, inputFilter := range inputValue.Filters {
		dcFilter, err := convertFromInputFilter(inputFilter)
		if err != nil {
			return nil, err
		}
		dcFilters = append(dcFilters, dcFilter)
	}

	return &types.DynamicConfigValue{
		Value:   blob,
		Filters: dcFilters,
	}, nil
}

func convertFromInputFilter(inputFilter *cliFilter) (*types.DynamicConfigFilter, error) {
	encodedValue, err := json.Marshal(inputFilter.Value)
	if err != nil {
		return nil, err
	}

	return &types.DynamicConfigFilter{
		Name: inputFilter.Name,
		Value: &types.DataBlob{
			EncodingType: types.EncodingTypeJSON.Ptr(),
			Data:         encodedValue,
		},
	}, nil
}

func parseInputFilter(inputFilter string) ([]*types.DynamicConfigFilter, error) {
	if inputFilter == "" || inputFilter == "{}" {
		return nil, nil
	}

	var mapFilter = make(map[string]interface{})
	if err := json.Unmarshal([]byte(inputFilter), &mapFilter); err != nil {
		return nil, err
	}

	var parsedFilters = make([]*types.DynamicConfigFilter, 0, len(mapFilter))

	for name, value := range mapFilter {
		parsedFilter, err := convertFromInputFilter(&cliFilter{
			Name:  name,
			Value: value,
		})
		if err != nil {
			return nil, err
		}

		parsedFilters = append(parsedFilters, parsedFilter)
	}

	return parsedFilters, nil
}
