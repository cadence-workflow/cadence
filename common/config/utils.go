package config

import (
	"reflect"
	"strings"
)

// CollectYAMLFields extracts all YAML field names from a struct.
// It recursively processes nested structs and returns a map of top-level field names.
func CollectYAMLFields(v interface{}) map[string]bool {
	fields := make(map[string]bool)
	collectFields(reflect.ValueOf(v), "", fields)
	return fields
}

// collectFields recursively processes a value to extract YAML field names.
func collectFields(val reflect.Value, prefix string, fields map[string]bool) {
	// If it's a pointer, get the element it points to
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return
		}
		val = val.Elem()
	}

	// Skip if not a struct
	if val.Kind() != reflect.Struct {
		return
	}

	typ := val.Type()

	// Process each field in the struct
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Get the YAML tag
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" {
			// If no yaml tag, use lowercase field name
			yamlTag = strings.ToLower(field.Name)
		} else {
			// Extract the name part of the tag (before any comma)
			yamlTag = strings.Split(yamlTag, ",")[0]

			// Skip fields marked with "-"
			if yamlTag == "-" {
				continue
			}
		}

		// Form the full path
		fullPath := yamlTag
		if prefix != "" {
			fullPath = prefix + "." + yamlTag
		}

		// Extract top-level key
		topLevelKey := strings.Split(fullPath, ".")[0]
		fields[topLevelKey] = true

		// Recursively process nested structs
		fieldVal := val.Field(i)
		if fieldVal.Kind() == reflect.Struct ||
			(fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() && fieldVal.Elem().Kind() == reflect.Struct) {
			collectFields(fieldVal, fullPath, fields)
		}

		// Handle slices/arrays of structs
		if (fieldVal.Kind() == reflect.Slice || fieldVal.Kind() == reflect.Array) &&
			!fieldVal.IsNil() && fieldVal.Len() > 0 {
			// Get the element type
			elemVal := fieldVal.Index(0)
			if elemVal.Kind() == reflect.Struct ||
				(elemVal.Kind() == reflect.Ptr && !elemVal.IsNil() && elemVal.Elem().Kind() == reflect.Struct) {
				collectFields(elemVal, fullPath, fields)
			}
		}

		// Handle maps with struct values
		if fieldVal.Kind() == reflect.Map && !fieldVal.IsNil() && fieldVal.Len() > 0 {
			// Get a value type from the map
			for _, key := range fieldVal.MapKeys() {
				elemVal := fieldVal.MapIndex(key)
				if elemVal.Kind() == reflect.Struct ||
					(elemVal.Kind() == reflect.Ptr && !elemVal.IsNil() && elemVal.Elem().Kind() == reflect.Struct) {
					// For map we don't append the key to the path since YAML mapping is dynamic
					collectFields(elemVal, fullPath, fields)
					break // We only need to check one value
				}
			}
		}
	}
}

// ExtractTopLevelKeys is a convenience function that takes any struct
// and returns only the top-level YAML keys it would access
func ExtractTopLevelKeys(v interface{}) []string {
	fieldMap := CollectYAMLFields(v)
	keys := make([]string, 0, len(fieldMap))
	for key := range fieldMap {
		keys = append(keys, key)
	}
	return keys
}
