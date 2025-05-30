package release

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		name           string
		rawTag         string
		expectedModule string
		expectedVer    string
		expectedPre    bool
		expectedPreNum int
		shouldHaveVer  bool
	}{
		{
			name:           "root module stable version",
			rawTag:         "v1.2.3",
			expectedModule: "",
			expectedVer:    "1.2.3",
			expectedPre:    false,
			shouldHaveVer:  true,
		},
		{
			name:           "root module prerelease",
			rawTag:         "v1.2.3-prerelease01",
			expectedModule: "",
			expectedVer:    "1.2.3-prerelease01",
			expectedPre:    true,
			expectedPreNum: 1,
			shouldHaveVer:  true,
		},
		{
			name:           "submodule stable version",
			rawTag:         "service1/v1.2.3",
			expectedModule: "service1",
			expectedVer:    "1.2.3",
			expectedPre:    false,
			shouldHaveVer:  true,
		},
		{
			name:           "submodule prerelease",
			rawTag:         "service1/v1.2.3-prerelease05",
			expectedModule: "service1",
			expectedVer:    "1.2.3-prerelease05",
			expectedPre:    true,
			expectedPreNum: 5,
			shouldHaveVer:  true,
		},
		{
			name:           "nested module",
			rawTag:         "services/auth/v2.1.0",
			expectedModule: "services/auth",
			expectedVer:    "2.1.0",
			expectedPre:    false,
			shouldHaveVer:  true,
		},
		{
			name:           "high prerelease number",
			rawTag:         "v1.2.3-prerelease99",
			expectedModule: "",
			expectedVer:    "1.2.3-prerelease99",
			expectedPre:    true,
			expectedPreNum: 99,
			shouldHaveVer:  true,
		},
		{
			name:          "invalid version tag",
			rawTag:        "invalid-tag",
			shouldHaveVer: false,
		},
		{
			name:          "not a version tag",
			rawTag:        "refs/heads/master",
			shouldHaveVer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			parsed := manager.parseTag(tt.rawTag)

			assert.Equal(t, tt.rawTag, parsed.Raw)
			assert.Equal(t, tt.expectedModule, parsed.ModulePath)
			assert.Equal(t, tt.expectedPre, parsed.IsPrerelease)

			if tt.shouldHaveVer {
				require.NotNil(t, parsed.Version)
				assert.Equal(t, tt.expectedVer, parsed.Version.String())
				if tt.expectedPre {
					assert.Equal(t, tt.expectedPreNum, parsed.PrereleaseNum)
				}
			} else {
				assert.Nil(t, parsed.Version)
			}
		})
	}
}

func TestIncrementVersion(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		versionType string
		expected    string
		shouldError bool
	}{
		{
			name:        "increment major",
			current:     "v1.2.3",
			versionType: "major",
			expected:    "v2.0.0",
		},
		{
			name:        "increment minor",
			current:     "v1.2.3",
			versionType: "minor",
			expected:    "v1.3.0",
		},
		{
			name:        "increment patch",
			current:     "v1.2.3",
			versionType: "patch",
			expected:    "v1.2.4",
		},
		{
			name:        "increment from zero",
			current:     "v0.0.0",
			versionType: "minor",
			expected:    "v0.1.0",
		},
		{
			name:        "increment major from high version",
			current:     "v15.27.99",
			versionType: "major",
			expected:    "v16.0.0",
		},
		{
			name:        "invalid version type",
			current:     "v1.2.3",
			versionType: "invalid",
			shouldError: true,
		},
		{
			name:        "invalid current version",
			current:     "invalid",
			versionType: "major",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IncrementVersion(tt.current, tt.versionType)

			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		shouldError bool
	}{
		{
			name:     "already normalized",
			input:    "v1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "add v prefix",
			input:    "1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "prerelease with v",
			input:    "v1.2.3-prerelease01",
			expected: "v1.2.3-prerelease01",
		},
		{
			name:     "prerelease without v",
			input:    "1.2.3-prerelease01",
			expected: "v1.2.3-prerelease01",
		},
		{
			name:     "invalid semver",
			input:    "1.2",
			expected: "v1.2.0",
		},
		{
			name:        "completely invalid",
			input:       "not-a-version",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeVersion(tt.input)

			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetNextPrereleaseVersion(t *testing.T) {
	tests := []struct {
		name         string
		baseVersion  string
		existingPres []int
		expected     string
		shouldError  bool
		errorMsg     string
	}{
		{
			name:         "first prerelease",
			baseVersion:  "v1.2.3",
			existingPres: []int{},
			expected:     "v1.2.3-prerelease01",
		},
		{
			name:         "increment from existing",
			baseVersion:  "v1.2.3",
			existingPres: []int{1, 2, 3},
			expected:     "v1.2.3-prerelease04",
		},
		{
			name:         "increment from high number",
			baseVersion:  "v1.2.3",
			existingPres: []int{25},
			expected:     "v1.2.3-prerelease26",
		},
		{
			name:         "unordered existing prereleases",
			baseVersion:  "v1.2.3",
			existingPres: []int{3, 1, 5, 2},
			expected:     "v1.2.3-prerelease06",
		},
		{
			name:         "max prerelease exceeded",
			baseVersion:  "v1.2.3",
			existingPres: []int{99}, // Next would be 100
			shouldError:  true,
			errorMsg:     "maximum prerelease number (99) exceeded",
		},
		{
			name:        "invalid base version",
			baseVersion: "invalid",
			shouldError: true,
			errorMsg:    "failed to parse base version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				tagCache: &TagCache{
					PrereleaseCache: make(map[string][]int),
				},
			}

			// Setup prerelease cache
			if len(tt.existingPres) > 0 {
				// Parse base version to get the clean string
				if baseVer, err := semver.NewVersion(tt.baseVersion); err == nil {
					cleanBase := "v" + baseVer.String()
					manager.tagCache.PrereleaseCache[cleanBase] = tt.existingPres
				}
			}

			result, err := manager.GetNextPrereleaseVersion(tt.baseVersion)

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestVersionCalculationMethods(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		currentVersion string
		expected       string
		shouldError    bool
		errorMsg       string
	}{
		// Release version calculations
		{
			name:           "release from prerelease",
			method:         "release",
			currentVersion: "v1.2.3-prerelease01",
			expected:       "v1.2.3",
		},
		{
			name:           "release from high prerelease",
			currentVersion: "v2.5.10-prerelease15",
			method:         "release",
			expected:       "v2.5.10",
		},

		// Minor version calculations
		{
			name:           "minor from stable",
			method:         "minor",
			currentVersion: "v1.2.3",
			expected:       "v1.3.0-prerelease01",
		},
		{
			name:           "minor from prerelease base",
			method:         "minor",
			currentVersion: "v1.2.3-prerelease05",
			expected:       "v1.3.0-prerelease01",
		},

		// Major version calculations
		{
			name:           "major from stable",
			method:         "major",
			currentVersion: "v1.2.3",
			expected:       "v2.0.0-prerelease01",
		},
		{
			name:           "major from prerelease base",
			method:         "major",
			currentVersion: "v1.2.3-prerelease05",
			expected:       "v2.0.0-prerelease01",
		},

		// Prerelease calculations
		{
			name:           "prerelease from stable",
			method:         "prerelease",
			currentVersion: "v1.2.3",
			expected:       "v1.2.3-prerelease01",
		},
		{
			name:           "prerelease increment",
			method:         "prerelease",
			currentVersion: "v1.2.3-prerelease01",
			expected:       "v1.2.3-prerelease02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				tagCache: &TagCache{
					PrereleaseCache: make(map[string][]int),
				},
			}

			// Setup prerelease cache for increment scenarios
			if tt.method == "prerelease" && tt.currentVersion == "v1.2.3-prerelease01" {
				manager.tagCache.PrereleaseCache["v1.2.3"] = []int{1}
			}

			var result string
			var err error

			switch tt.method {
			case "release":
				result, err = manager.calculateReleaseVersion(tt.currentVersion)
			case "minor":
				result, err = manager.calculateMinorVersion(tt.currentVersion)
			case "major":
				result, err = manager.calculateMajorVersion(tt.currentVersion)
			case "prerelease":
				result, err = manager.calculatePrereleaseVersion(tt.currentVersion)
			}

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGetBaseVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "stable version",
			input:    "v1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "prerelease version",
			input:    "v1.2.3-prerelease01",
			expected: "v1.2.3",
		},
		{
			name:     "high prerelease version",
			input:    "v2.5.10-prerelease25",
			expected: "v2.5.10",
		},
		{
			name:     "invalid version",
			input:    "invalid",
			expected: "invalid", // Should return input unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			result := manager.getBaseVersion(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCurrentGlobalVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionTags []string
		expected    string
	}{
		{
			name:        "no versions",
			versionTags: []string{},
			expected:    "v0.0.0",
		},
		{
			name:        "single version",
			versionTags: []string{"v1.2.3"},
			expected:    "v1.2.3",
		},
		{
			name:        "multiple versions",
			versionTags: []string{"v1.2.3", "v1.1.0", "v2.0.0", "v1.3.0"},
			expected:    "v2.0.0",
		},
		{
			name:        "prerelease versions",
			versionTags: []string{"v1.2.3", "v1.3.0-prerelease01", "v1.2.4"},
			expected:    "v1.3.0-prerelease01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				tagCache: &TagCache{
					VersionTags: make([]ParsedTag, 0),
				},
			}

			// Parse and add version tags
			for _, tag := range tt.versionTags {
				parsed := manager.parseTag(tag)
				if parsed.Version != nil {
					manager.tagCache.VersionTags = append(manager.tagCache.VersionTags, parsed)
				}
			}

			// Sort and set highest version
			if len(manager.tagCache.VersionTags) > 0 {
				manager.sortVersionTags()
				manager.tagCache.HighestVersion = manager.tagCache.VersionTags[len(manager.tagCache.VersionTags)-1].Version
			}

			result := manager.GetCurrentGlobalVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortVersionTags(t *testing.T) {
	manager := &Manager{
		tagCache: &TagCache{
			VersionTags: []ParsedTag{
				{Raw: "v2.0.0", Version: semver.MustParse("v2.0.0")},
				{Raw: "v1.2.3", Version: semver.MustParse("v1.2.3")},
				{Raw: "v1.3.0-prerelease01", Version: semver.MustParse("v1.3.0-prerelease01")},
				{Raw: "v1.1.0", Version: semver.MustParse("v1.1.0")},
			},
		},
	}

	manager.sortVersionTags()

	expected := []string{
		"v1.1.0",
		"v1.2.3",
		"v1.3.0-prerelease01",
		"v2.0.0",
	}

	actual := make([]string, len(manager.tagCache.VersionTags))
	for i, tag := range manager.tagCache.VersionTags {
		actual[i] = tag.Raw
	}

	assert.Equal(t, expected, actual)
}
