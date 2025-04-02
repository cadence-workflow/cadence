package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Simple top-level struct
type SimpleConfig struct {
	Database string `yaml:"db"`
	Logging  string `yaml:"log"`
	Port     int    `yaml:"port"`
}

// Nested structs
type AppConfig struct {
	DB struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"db"`

	Log struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"log"`

	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
}

// Struct with slice
type ServiceConfig struct {
	Name     string `yaml:"name"`
	Services []struct {
		Name  string `yaml:"name"`
		URL   string `yaml:"url"`
		Token string `yaml:"token"`
	} `yaml:"services"`
	Cache map[string]struct {
		TTL int `yaml:"ttl"`
	} `yaml:"cache"`
}

// Struct without YAML tags
type NoTagConfig struct {
	Database struct {
		Username string
		Password string
	}
	Server struct {
		Host string
		Port int
	}
}

// Struct with pointers
type PointerConfig struct {
	Database *struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"db"`

	Log *struct {
		Level string `yaml:"level"`
		File  string `yaml:"file"`
	} `yaml:"log"`
}

// Struct with ignored fields
type IgnoredFieldsConfig struct {
	Database string `yaml:"db"`
	Internal string `yaml:"-"`
	private  string // unexported fields are ignored
}

func TestCollectYAMLFields(t *testing.T) {
	// Test simple struct
	t.Run("SimpleStruct", func(t *testing.T) {
		fields := CollectYAMLFields(SimpleConfig{})
		assert.ElementsMatch(t, []string{"db", "log", "port"}, keysFromMap(fields))
	})

	// Test nested structs
	t.Run("NestedStructs", func(t *testing.T) {
		fields := CollectYAMLFields(AppConfig{})
		assert.ElementsMatch(t, []string{"db", "log", "server"}, keysFromMap(fields))
	})

	// Test struct with slice
	t.Run("StructWithSlice", func(t *testing.T) {
		config := ServiceConfig{
			Services: []struct {
				Name  string `yaml:"name"`
				URL   string `yaml:"url"`
				Token string `yaml:"token"`
			}{
				{Name: "test", URL: "http://example.com", Token: "token"},
			},
			Cache: map[string]struct {
				TTL int `yaml:"ttl"`
			}{
				"test": {TTL: 3600},
			},
		}
		fields := CollectYAMLFields(config)
		assert.ElementsMatch(t, []string{"name", "services", "cache"}, keysFromMap(fields))
	})

	// Test struct without YAML tags
	t.Run("NoTags", func(t *testing.T) {
		fields := CollectYAMLFields(NoTagConfig{})
		assert.ElementsMatch(t, []string{"database", "server"}, keysFromMap(fields))
	})

	// Test struct with pointers
	t.Run("PointerFields", func(t *testing.T) {
		// With nil pointers
		nilConfig := PointerConfig{}
		fields := CollectYAMLFields(nilConfig)
		assert.ElementsMatch(t, []string{"db", "log"}, keysFromMap(fields))

		// With non-nil pointers
		fullConfig := PointerConfig{
			Database: &struct {
				Host     string `yaml:"host"`
				Port     int    `yaml:"port"`
				Username string `yaml:"username"`
				Password string `yaml:"password"`
			}{},
			Log: &struct {
				Level string `yaml:"level"`
				File  string `yaml:"file"`
			}{},
		}
		fields = CollectYAMLFields(fullConfig)
		assert.ElementsMatch(t, []string{"db", "log"}, keysFromMap(fields))
	})

	// Test ignored fields
	t.Run("IgnoredFields", func(t *testing.T) {
		fields := CollectYAMLFields(IgnoredFieldsConfig{})
		assert.ElementsMatch(t, []string{"db"}, keysFromMap(fields))
	})

	// Test the convenience function
	t.Run("ExtractTopLevelKeys", func(t *testing.T) {
		keys := ExtractTopLevelKeys(AppConfig{})
		assert.ElementsMatch(t, []string{"db", "log", "server"}, keys)
	})
}

// Helper function to extract keys from a map
func keysFromMap(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
