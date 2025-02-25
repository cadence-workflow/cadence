package sqlite

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/uber/cadence/common/config"
)

// buildDSN builds the data source name for sqlite from config.SQL
// If DatabaseName is not provided, then sqlite will use in-memory database, otherwise it will use the file as the database
// All dsn attributes can be set up in the ConnectAttributes field of the config.SQL
// Available attributes can be found here: https://github.com/ncruces/go-sqlite3/blob/main/driver/driver.go
// PRAGMA attributes should start with "_pragma." prefix
// example: "_pragma.journal_mode":"wal" will be transformed to "_pragma=journal_mode(wal)"
// More about PRAGMA attributes: https://sqlite.org/pragma.html
// Default PRAGMA values if not provided:
// - journal_mode: WAL (file only) 	- https://sqlite.org/pragma.html#pragma_journal_mode
// - busy_timeout: 60 seconds 		- https://sqlite.org/pragma.html#pragma_busy_timeout
func buildDSN(cfg *config.SQL) string {

	// by default, we use in-memory database if no database name is provided
	var dsn = "file::memory:"

	// if database name is provided, then sqlite will use the file as the database
	if cfg.DatabaseName != "" {
		dsn = fmt.Sprintf("file:%s", cfg.DatabaseName)

	}

	if dsnAttrs := buildDSNAttrs(cfg); dsnAttrs != "" {
		dsn += "?" + dsnAttrs
	}

	return dsn
}

const (
	// if journal mode is not provided, we set it to WAL by default
	// WAL mode allows readers and writers from different processes
	// to access the database concurrently by default
	// https://sqlite.org/pragma.html#pragma_journal_mode
	// https://www.sqlite.org/wal.html
	pragmaJournalModeAttrName     = "_pragma.journal_mode"
	pragmaJournalModeDefaultValue = "WAL"

	// if busy_timeout is not provided, we set it to 60 seconds by default
	// https://sqlite.org/pragma.html#pragma_busy_timeout
	pragmaBusyTimeoutAttrName = "_pragma.busy_timeout"
	pragmaBusyTimeoutDefault  = "60000"
)

// buildDSNAttrs builds the data source name attributes for sqlite from config.SQL
func buildDSNAttrs(cfg *config.SQL) string {

	sanitizedAttrs := sanitizeDSNAttrs(cfg.ConnectAttributes)

	if cfg.DatabaseName != "" {
		defaultIfEmpty(sanitizedAttrs, pragmaJournalModeAttrName, pragmaJournalModeDefaultValue)
	}

	defaultIfEmpty(sanitizedAttrs, pragmaBusyTimeoutAttrName, pragmaBusyTimeoutDefault)
	return joinDSNAttrs(sanitizedAttrs)
}

// defaultIfEmpty sets the value to the key if the key is not present in the attributes
func defaultIfEmpty(attrs map[string]string, key, value string) {
	if hasAttr(attrs, key) {
		return
	}
	attrs[key] = value
}

// hasAttr checks if the attributes map has any of the keys
func hasAttr(attrs map[string]string, keys ...string) bool {
	for key := range attrs {
		for _, k := range keys {
			if key == k {
				return true
			}
		}
	}
	return false
}

// sanitizeDSNAttrs sanitizes the attributes by trimming the keys and values
func sanitizeDSNAttrs(attrs map[string]string) map[string]string {
	sanitized := make(map[string]string, len(attrs))

	for k, v := range attrs {
		k, v = sanitizeDSNAttrKey(k), sanitizeDSNAttrValue(v)
		sanitized[k] = v
	}

	return sanitized
}

const (
	// pragmaKey is the key is used to set pragma arguments in the DSN
	pragmaKey = "_pragma"

	// pragmaPrefix is the prefix used to identify pragma arguments in the config
	pragmaPrefix = "_pragma."
)

// isPragmaKey checks if the key is a pragma key
func isPragmaKey(key string) bool {
	return strings.HasPrefix(key, pragmaPrefix)
}

// transformPragmaArgument transforms the pragma argument to the format that can be used in the DSN
// example: "_pragma.journal_mode":"wal" -> "_pragma=journal_mode(wal)"
func transformPragmaArgument(key, value string) (newKey, newValue string) {
	return pragmaKey, fmt.Sprintf("%s(%s)", strings.TrimPrefix(key, pragmaPrefix), value)
}

// sanitizeDSNAttrElem trims the value, lowercases it
func sanitizeDSNAttrKey(v string) string {
	return strings.TrimSpace(strings.ToLower(v))
}

// sanitizeDSNAttrElem trims the value, lowercases it
func sanitizeDSNAttrValue(v string) string {
	return strings.TrimSpace(v)
}

// sortedKeys returns the sorted keys of the map
func sortMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

// joinDSNAttrs joins the attributes into a single string
// with key=value pairs separated by & and escaped
func joinDSNAttrs(attrs map[string]string) string {
	first := true
	var buf bytes.Buffer

	// sort the keys to make the order of the attributes deterministic
	sortedKeys := sortMapKeys(attrs)

	for _, k := range sortedKeys {
		v := attrs[k]

		// pragma arguments should be transformed to the format that can be used in the DSN
		if isPragmaKey(k) {
			k, v = transformPragmaArgument(k, v)
		}

		if !first {
			buf.WriteString("&")
		}
		first = false
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v)
	}
	return buf.String()
}
