package release

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
)

// Helper function to create a test manager with generated mocks
func createTestManager(t *testing.T, config *Config) (*Manager, *MockGit, *MockFS) {
	ctrl := gomock.NewController(t)
	mockGit := NewMockGit(ctrl)
	mockFS := NewMockFS(ctrl)
	logger := zaptest.NewLogger(t)

	manager := NewReleaseManager(config, mockGit, mockFS, logger)
	return manager, mockGit, mockFS
}

// Test NormalizeVersion function
func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:     "version with v prefix",
			input:    "v1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "version without v prefix",
			input:    "1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "prerelease version",
			input:    "v1.2.3-alpha.1",
			expected: "v1.2.3-alpha.1",
		},
		{
			name:        "invalid version",
			input:       "invalid",
			expectError: true,
		},
		{
			name:        "empty version",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeVersion(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Test IncrementVersion function
func TestIncrementVersion(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		versionType string
		expected    string
		expectError bool
	}{
		{
			name:        "increment patch",
			current:     "v1.2.3",
			versionType: "patch",
			expected:    "v1.2.4",
		},
		{
			name:        "increment minor",
			current:     "v1.2.3",
			versionType: "minor",
			expected:    "v1.3.0",
		},
		{
			name:        "increment major",
			current:     "v1.2.3",
			versionType: "major",
			expected:    "v2.0.0",
		},
		{
			name:        "invalid version type",
			current:     "v1.2.3",
			versionType: "invalid",
			expectError: true,
		},
		{
			name:        "invalid current version",
			current:     "invalid",
			versionType: "patch",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := IncrementVersion(tt.current, tt.versionType)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// Test FindModules method
func TestManager_FindModules(t *testing.T) {
	tests := []struct {
		name          string
		goModFiles    []string
		excludedDirs  []string
		expectedCount int
		expectError   bool
		expectedPaths []string
	}{
		{
			name:          "single module at root",
			goModFiles:    []string{"/repo/go.mod"},
			expectedCount: 1,
			expectedPaths: []string{"go.mod"},
		},
		{
			name:          "multiple modules",
			goModFiles:    []string{"/repo/go.mod", "/repo/service1/go.mod", "/repo/service2/go.mod"},
			expectedCount: 3,
			expectedPaths: []string{"go.mod", "service1/go.mod", "service2/go.mod"},
		},
		{
			name:          "modules with exclusions",
			goModFiles:    []string{"/repo/go.mod", "/repo/service1/go.mod", "/repo/test/go.mod"},
			excludedDirs:  []string{"test"},
			expectedCount: 2,
			expectedPaths: []string{"go.mod", "service1/go.mod"},
		},
		{
			name:        "fs error",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				RepoRoot:     "/repo",
				ExcludedDirs: tt.excludedDirs,
			}

			manager, _, mockFS := createTestManager(t, config)

			if tt.expectError {
				mockFS.EXPECT().FindGoModFiles("/repo").Return(nil, fmt.Errorf("fs error"))
			} else {
				mockFS.EXPECT().FindGoModFiles("/repo").Return(tt.goModFiles, nil)
			}

			modules, err := manager.FindModules()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, modules, tt.expectedCount)

				// Check relative paths
				var actualPaths []string
				for _, module := range modules {
					actualPaths = append(actualPaths, module.RelativePath)
				}
				sort.Strings(actualPaths)
				sort.Strings(tt.expectedPaths)
				assert.Equal(t, tt.expectedPaths, actualPaths)
			}
		})
	}
}

// Test GetCurrentGlobalVersion method
func TestManager_GetCurrentGlobalVersion(t *testing.T) {
	tests := []struct {
		name        string
		tags        []string
		expected    string
		expectError bool
	}{
		{
			name:     "no existing tags",
			tags:     []string{},
			expected: "v0.0.0",
		},
		{
			name:     "single version tag",
			tags:     []string{"v1.2.3"},
			expected: "v1.2.3",
		},
		{
			name:     "multiple version tags",
			tags:     []string{"v1.2.3", "v1.2.4", "v1.1.0"},
			expected: "v1.2.4",
		},
		{
			name:     "mixed tags with module prefixes",
			tags:     []string{"v1.2.3", "service1/v1.2.4", "v1.2.2"},
			expected: "v1.2.4",
		},
		{
			name:     "prerelease versions",
			tags:     []string{"v1.2.3", "v1.2.4-prerelease01", "v1.2.5-prerelease02"},
			expected: "v1.2.5-prerelease02",
		},
		{
			name:     "non-version tags ignored",
			tags:     []string{"v1.2.3", "latest", "production", "v1.2.4"},
			expected: "v1.2.4",
		},
		{
			name:        "git error",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{RepoRoot: "/repo"}
			manager, mockGit, _ := createTestManager(t, config)

			if tt.expectError {
				mockGit.EXPECT().GetTags().Return(nil, fmt.Errorf("git error"))
			} else {
				mockGit.EXPECT().GetTags().Return(tt.tags, nil)
			}

			version, err := manager.GetCurrentGlobalVersion()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, version)
			}
		})
	}
}

// Test GetNextPrereleaseVersion method
func TestManager_GetNextPrereleaseVersion(t *testing.T) {
	tests := []struct {
		name         string
		baseVersion  string
		existingTags []string
		expected     string
		expectError  bool
	}{
		{
			name:         "first prerelease",
			baseVersion:  "v1.2.3",
			existingTags: []string{},
			expected:     "v1.2.3-prerelease01",
		},
		{
			name:         "increment existing prerelease",
			baseVersion:  "v1.2.3",
			existingTags: []string{"v1.2.3-prerelease01"},
			expected:     "v1.2.3-prerelease02",
		},
		{
			name:         "multiple existing prereleases",
			baseVersion:  "v1.2.3",
			existingTags: []string{"v1.2.3-prerelease01", "v1.2.3-prerelease02", "v1.2.3-prerelease03"},
			expected:     "v1.2.3-prerelease04",
		},
		{
			name:         "with module prefixes",
			baseVersion:  "v1.2.3",
			existingTags: []string{"service1/v1.2.3-prerelease01", "v1.2.3-prerelease02"},
			expected:     "v1.2.3-prerelease03",
		},
		{
			name:         "error on single digit prerelease",
			baseVersion:  "v1.2.3",
			existingTags: []string{"v1.2.3-prerelease1"},
			expectError:  true,
		},
		{
			name:         "increment from double digit",
			baseVersion:  "v1.2.3",
			existingTags: []string{"v1.2.3-prerelease10"},
			expected:     "v1.2.3-prerelease11",
		},
		{
			name:        "invalid base version",
			baseVersion: "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{RepoRoot: "/repo"}
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			logger := zaptest.NewLogger(t)

			manager := NewReleaseManager(config, mockGit, mockFS, logger)

			mockGit.EXPECT().GetTags().Return(tt.existingTags, nil).AnyTimes()

			version, err := manager.GetNextPrereleaseVersion(tt.baseVersion)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, version)
			}
		})
	}
}

// Test CalculateNewVersion method
func TestManager_CalculateNewVersion(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		current      string
		existingTags []string
		expected     string
		expectError  bool
	}{
		{
			name: "explicit version",
			config: &Config{
				Version:  "v2.0.0",
				RepoRoot: "/repo",
			},
			current:  "v1.2.3",
			expected: "v2.0.0",
		},
		{
			name: "explicit version with prerelease",
			config: &Config{
				Version:    "v2.0.0",
				Prerelease: true,
				RepoRoot:   "/repo",
			},
			current:      "v1.2.3",
			existingTags: []string{},
			expected:     "v2.0.0-prerelease01",
		},
		{
			name: "version bump patch",
			config: &Config{
				VersionType: "patch",
				RepoRoot:    "/repo",
			},
			current:  "v1.2.3",
			expected: "v1.2.4",
		},
		{
			name: "version bump minor with prerelease",
			config: &Config{
				VersionType: "minor",
				Prerelease:  true,
				RepoRoot:    "/repo",
			},
			current:      "v1.2.3",
			existingTags: []string{},
			expected:     "v1.3.0-prerelease01",
		},
		{
			name: "prerelease-only from stable",
			config: &Config{
				VersionType: "prerelease-only",
				RepoRoot:    "/repo",
			},
			current:      "v1.2.3",
			existingTags: []string{},
			expected:     "v1.2.3-prerelease01",
		},
		{
			name: "prerelease-only increment",
			config: &Config{
				VersionType: "prerelease-only",
				RepoRoot:    "/repo",
			},
			current:      "v1.2.3-prerelease01",
			existingTags: []string{"v1.2.3-prerelease01"},
			expected:     "v1.2.3-prerelease02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			logger := zaptest.NewLogger(t)

			manager := NewReleaseManager(tt.config, mockGit, mockFS, logger)

			if tt.config.Prerelease || tt.config.VersionType == "prerelease-only" {
				mockGit.EXPECT().GetTags().Return(tt.existingTags, nil).AnyTimes()
			}

			version, err := manager.CalculateNewVersion(tt.current)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, version)
			}
		})
	}
}

// Test ValidateEnvironment method
func TestManager_ValidateEnvironment(t *testing.T) {
	tests := []struct {
		name            string
		config          *Config
		currentBranch   string
		branchError     error
		workingDirError error
		expectError     bool
	}{
		{
			name: "valid environment",
			config: &Config{
				RequiredBranch: "main",
				DryRun:         false,
			},
			currentBranch: "main",
		},
		{
			name: "wrong branch",
			config: &Config{
				RequiredBranch: "main",
			},
			currentBranch: "feature",
			expectError:   true,
		},
		{
			name: "no required branch",
			config: &Config{
				DryRun: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, mockGit, _ := createTestManager(t, tt.config)

			if tt.config.RequiredBranch != "" {
				mockGit.EXPECT().GetCurrentBranch().Return(tt.currentBranch, tt.branchError)
			}

			err := manager.ValidateEnvironment()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test CheckVersionExists method
func TestManager_CheckVersionExists(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		modules      []Module
		existingTags []string
		expectError  bool
	}{
		{
			name:    "version does not exist",
			version: "v1.2.4",
			modules: []Module{
				{RelativePath: ""},
				{RelativePath: "service1"},
			},
			existingTags: []string{"v1.2.3", "service1/v1.2.3"},
		},
		{
			name:    "version already exists for root",
			version: "v1.2.3",
			modules: []Module{
				{RelativePath: ""},
			},
			existingTags: []string{"v1.2.3"},
			expectError:  true,
		},
		{
			name:    "version already exists for module",
			version: "v1.2.3",
			modules: []Module{
				{RelativePath: "service1"},
			},
			existingTags: []string{"service1/v1.2.3"},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{RepoRoot: "/repo"}
			manager, mockGit, _ := createTestManager(t, config)

			mockGit.EXPECT().GetTags().Return(tt.existingTags, nil)

			err := manager.CheckVersionExists(tt.version, tt.modules)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test CreateRelease method
func TestManager_CreateRelease(t *testing.T) {
	tests := []struct {
		name        string
		modules     []Module
		version     string
		expectError bool
		modTidyErr  bool
		buildErr    bool
		tagErr      bool
		pushErr     bool
	}{
		{
			name: "successful release",
			modules: []Module{
				{Path: "/repo", RelativePath: ""},
				{Path: "/repo/service1", RelativePath: "service1"},
			},
			version: "v1.2.4",
		},
		{
			name: "mod tidy failure",
			modules: []Module{
				{Path: "/repo", RelativePath: ""},
			},
			version:     "v1.2.4",
			modTidyErr:  true,
			expectError: true,
		},
		{
			name: "build failure",
			modules: []Module{
				{Path: "/repo", RelativePath: ""},
			},
			version:     "v1.2.4",
			buildErr:    true,
			expectError: true,
		},
		{
			name: "tag creation failure",
			modules: []Module{
				{Path: "/repo", RelativePath: ""},
			},
			version:     "v1.2.4",
			tagErr:      true,
			expectError: true,
		},
		{
			name: "tag push failure",
			modules: []Module{
				{Path: "/repo", RelativePath: ""},
			},
			version:     "v1.2.4",
			pushErr:     true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{RepoRoot: "/repo"}
			manager, mockGit, mockFS := createTestManager(t, config)

			for _, module := range tt.modules {
				expectedTag := tt.version
				if module.RelativePath != "" {
					expectedTag = module.RelativePath + "/" + tt.version
				}

				if tt.modTidyErr {
					mockFS.EXPECT().ModTidy(module.Path).Return(fmt.Errorf("mod tidy error"))
				} else {
					mockFS.EXPECT().ModTidy(module.Path).Return(nil)

					if tt.buildErr {
						mockFS.EXPECT().Build(module.Path).Return(fmt.Errorf("build error"))
					} else {
						mockFS.EXPECT().Build(module.Path).Return(nil)

						if tt.tagErr {
							mockGit.EXPECT().CreateTag(expectedTag).Return(fmt.Errorf("tag error"))
						} else {
							mockGit.EXPECT().CreateTag(expectedTag).Return(nil)

							if tt.pushErr {
								mockGit.EXPECT().PushTag(expectedTag).Return(fmt.Errorf("push error"))
							} else {
								mockGit.EXPECT().PushTag(expectedTag).Return(nil)
							}
						}
					}
				}
			}

			err := manager.CreateRelease(tt.modules, tt.version)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Integration test simulating a typical Cadence workflow development cycle
func TestCadenceWorkflowCycle(t *testing.T) {
	config := &Config{
		RepoRoot:       "/repo",
		VersionType:    "minor",
		Prerelease:     true,
		RequiredBranch: "main",
	}

	manager, mockGit, mockFS := createTestManager(t, config)

	// Setup mock expectations for the workflow cycle
	goModFiles := []string{"/repo/go.mod", "/repo/workflows/go.mod"}
	mockFS.EXPECT().FindGoModFiles("/repo").Return(goModFiles, nil)

	// Step 1: Start with v1.2.0, create first prerelease
	mockGit.EXPECT().GetTags().Return([]string{"v1.2.0", "workflows/v1.2.0"}, nil).AnyTimes()
	mockGit.EXPECT().GetCurrentBranch().Return("main", nil)

	// Find modules
	foundModules, err := manager.FindModules()
	require.NoError(t, err)
	assert.Len(t, foundModules, 2)

	// Get current version
	currentVersion, err := manager.GetCurrentGlobalVersion()
	require.NoError(t, err)
	assert.Equal(t, "v1.2.0", currentVersion)

	// Calculate new version (v1.3.0-prerelease01)
	newVersion, err := manager.CalculateNewVersion(currentVersion)
	require.NoError(t, err)
	assert.Equal(t, "v1.3.0-prerelease01", newVersion)

	// Validate environment
	err = manager.ValidateEnvironment()
	require.NoError(t, err)

	// Check version doesn't exist
	err = manager.CheckVersionExists(newVersion, foundModules)
	require.NoError(t, err)

	t.Logf("Cadence workflow cycle test completed: %s -> %s", currentVersion, newVersion)
}

// Benchmark tests for performance-critical functions
func BenchmarkNormalizeVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NormalizeVersion("1.2.3")
	}
}

func BenchmarkIncrementVersion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = IncrementVersion("v1.2.3", "patch")
	}
}

// Test Run method with dry run
func TestManager_Run_DryRun(t *testing.T) {
	config := &Config{
		RepoRoot:       "/repo",
		VersionType:    "patch",
		DryRun:         true,
		RequiredBranch: "main",
	}

	manager, mockGit, mockFS := createTestManager(t, config)

	// Setup expectations
	mockGit.EXPECT().GetCurrentBranch().Return("main", nil)
	// No working dir check in dry run

	goModFiles := []string{"/repo/go.mod"}
	mockFS.EXPECT().FindGoModFiles("/repo").Return(goModFiles, nil)

	mockGit.EXPECT().GetTags().Return([]string{"v1.2.3"}, nil).AnyTimes()

	err := manager.Run()
	assert.NoError(t, err)
}

// Test Run method with successful release
func TestManager_Run_Success(t *testing.T) {
	config := &Config{
		RepoRoot:       "/repo",
		VersionType:    "patch",
		DryRun:         false,
		RequiredBranch: "main",
	}

	manager, mockGit, mockFS := createTestManager(t, config)

	// Setup expectations for full run
	mockGit.EXPECT().GetCurrentBranch().Return("main", nil)

	goModFiles := []string{"/repo/go.mod"}
	mockFS.EXPECT().FindGoModFiles("/repo").Return(goModFiles, nil)

	mockGit.EXPECT().GetTags().Return([]string{"v1.2.3"}, nil)
	mockGit.EXPECT().GetTags().Return([]string{"v1.2.3"}, nil)

	// Release creation expectations
	mockFS.EXPECT().ModTidy("/repo/go.mod").Return(nil)
	mockFS.EXPECT().Build("/repo/go.mod").Return(nil)
	mockGit.EXPECT().CreateTag("go.mod/v1.2.4").Return(nil)
	mockGit.EXPECT().PushTag("go.mod/v1.2.4").Return(nil)

	err := manager.Run()
	assert.NoError(t, err)
}

// Test edge case: maximum prerelease number
func TestManager_GetNextPrereleaseVersion_MaxNumber(t *testing.T) {
	config := &Config{RepoRoot: "/repo"}
	manager, mockGit, _ := createTestManager(t, config)

	// Setup tags with prerelease99
	existingTags := []string{"v1.2.3-prerelease99"}
	mockGit.EXPECT().GetTags().Return(existingTags, nil)

	version, err := manager.GetNextPrereleaseVersion("v1.2.3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum prerelease number (99) exceeded")
	assert.Empty(t, version)
}

// Test shouldExcludeModule method
func TestManager_shouldExcludeModule(t *testing.T) {
	config := &Config{
		RepoRoot:     "/repo",
		ExcludedDirs: []string{"test", "docs", "examples"},
	}

	manager, _, _ := createTestManager(t, config)

	tests := []struct {
		relPath  string
		excluded bool
	}{
		{"", false},
		{"service1", false},
		{"test", true},
		{"test/unit", true},
		{"docs", true},
		{"docs/api", true},
		{"examples", true},
		{"examples/basic", true},
		{"workflows", false},
		{"testdata", false}, // Not in excluded list
	}

	for _, tt := range tests {
		t.Run(tt.relPath, func(t *testing.T) {
			result := manager.shouldExcludeModule(tt.relPath)
			assert.Equal(t, tt.excluded, result)
		})
	}
}
