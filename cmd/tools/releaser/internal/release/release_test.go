package release

import (
	"context"
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewReleaseManager(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGit(ctrl)
	mockFS := NewMockFS(ctrl)
	mockInteraction := NewMockUserInteraction(ctrl)

	config := &Config{
		RepoRoot:       "/test/repo",
		Prerelease:     false,
		Version:        "1.0.0",
		VersionType:    "patch",
		ExcludedDirs:   []string{"vendor", "test"},
		RequiredBranch: "main",
		Verbose:        true,
	}

	manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
	assert.Equal(t, mockGit, manager.git)
	assert.Equal(t, mockFS, manager.fs)
	assert.Equal(t, mockInteraction, manager.interaction)
}

func TestManager_GetKnownReleases(t *testing.T) {
	testCases := []struct {
		name         string
		gitTags      []string
		gitError     error
		expectError  bool
		expectedTags int
	}{
		{
			name: "successful tag parsing",
			gitTags: []string{
				"v1.0.0",
				"v1.1.0",
				"service1/v2.0.0",
				"service2/v1.5.0-prerelease01",
				"invalid-tag",
				"v2.0.0-prerelease02",
			},
			gitError:     nil,
			expectError:  false,
			expectedTags: 5,
		},
		{
			name:         "git error",
			gitTags:      nil,
			gitError:     errors.New("git command failed"),
			expectError:  true,
			expectedTags: 0,
		},
		{
			name:         "empty tags",
			gitTags:      []string{},
			gitError:     nil,
			expectError:  false,
			expectedTags: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := &Config{RepoRoot: "/test"}
			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			mockGit.EXPECT().GetTags(gomock.Any()).Return(tc.gitTags, tc.gitError)

			err := manager.GetKnownReleases(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, manager.tagCache)
			assert.Len(t, manager.tagCache.VersionTags, tc.expectedTags)

			if tc.expectedTags > 0 {
				// Verify highest version is cached
				assert.NotNil(t, manager.tagCache.HighestVersion)
			}
		})
	}
}

func TestManager_parseTag(t *testing.T) {
	testCases := []struct {
		name                  string
		rawTag                string
		expectedModulePath    string
		expectedVersion       string
		expectedPrerelease    bool
		expectedPrereleaseNum int
	}{
		{
			name:               "root module version",
			rawTag:             "v1.2.3",
			expectedModulePath: "",
			expectedVersion:    "1.2.3",
			expectedPrerelease: false,
		},
		{
			name:                  "root module prerelease",
			rawTag:                "v1.2.3-prerelease01",
			expectedModulePath:    "",
			expectedVersion:       "1.2.3-prerelease01",
			expectedPrerelease:    true,
			expectedPrereleaseNum: 1,
		},
		{
			name:               "submodule version",
			rawTag:             "service1/v2.1.0",
			expectedModulePath: "service1",
			expectedVersion:    "2.1.0",
			expectedPrerelease: false,
		},
		{
			name:                  "submodule prerelease",
			rawTag:                "service1/v2.1.0-prerelease05",
			expectedModulePath:    "service1",
			expectedVersion:       "2.1.0-prerelease05",
			expectedPrerelease:    true,
			expectedPrereleaseNum: 5,
		},
		{
			name:               "invalid tag",
			rawTag:             "not-a-version",
			expectedModulePath: "",
			expectedVersion:    "",
			expectedPrerelease: false,
		},
	}

	manager := &Manager{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := manager.parseTag(tc.rawTag)

			assert.Equal(t, tc.rawTag, parsed.Raw)
			assert.Equal(t, tc.expectedModulePath, parsed.ModulePath)
			assert.Equal(t, tc.expectedPrerelease, parsed.IsPrerelease)

			if tc.expectedVersion != "" {
				require.NotNil(t, parsed.Version)
				assert.Equal(t, tc.expectedVersion, parsed.Version.String())
			} else {
				assert.Nil(t, parsed.Version)
			}

			if tc.expectedPrerelease {
				assert.Equal(t, tc.expectedPrereleaseNum, parsed.PrereleaseNum)
			}
		})
	}
}

func TestManager_calculateNewVersion(t *testing.T) {
	testCases := []struct {
		name            string
		currentVersion  string
		configVersion   string
		configType      string
		prerelease      bool
		expectedVersion string
		expectError     bool
		setupCache      func(*Manager)
	}{
		{
			name:            "explicit version",
			currentVersion:  "v1.0.0",
			configVersion:   "2.0.0",
			expectedVersion: "v2.0.0",
			expectError:     false,
		},
		{
			name:            "explicit version with prerelease",
			currentVersion:  "v1.0.0",
			configVersion:   "2.0.0",
			prerelease:      true,
			expectedVersion: "v2.0.0-prerelease01",
			expectError:     false,
			setupCache: func(m *Manager) {
				m.tagCache = &TagCache{
					PrereleaseCache: make(map[string][]int),
				}
			},
		},
		{
			name:            "major increment",
			currentVersion:  "v1.2.3",
			configType:      "major",
			expectedVersion: "v2.0.0",
			expectError:     false,
		},
		{
			name:            "minor increment",
			currentVersion:  "v1.2.3",
			configType:      "minor",
			expectedVersion: "v1.3.0",
			expectError:     false,
		},
		{
			name:            "patch increment",
			currentVersion:  "v1.2.3",
			configType:      "patch",
			expectedVersion: "v1.2.4",
			expectError:     false,
		},
		{
			name:            "patch increment with prerelease",
			currentVersion:  "v1.2.3",
			configType:      "patch",
			prerelease:      true,
			expectedVersion: "v1.2.4-prerelease01",
			expectError:     false,
			setupCache: func(m *Manager) {
				m.tagCache = &TagCache{
					PrereleaseCache: make(map[string][]int),
				}
			},
		},
		{
			name:            "prerelease-only from stable",
			currentVersion:  "v1.2.3",
			configType:      "prerelease-only",
			expectedVersion: "v1.2.3-prerelease01",
			expectError:     false,
			setupCache: func(m *Manager) {
				m.tagCache = &TagCache{
					PrereleaseCache: make(map[string][]int),
				}
			},
		},
		{
			name:           "invalid version type",
			currentVersion: "v1.0.0",
			configType:     "invalid",
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &Manager{
				config: &Config{
					Version:     tc.configVersion,
					VersionType: tc.configType,
					Prerelease:  tc.prerelease,
				},
			}

			if tc.setupCache != nil {
				tc.setupCache(manager)
			}

			result, err := manager.calculateNewVersion(tc.currentVersion)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedVersion, result)
		})
	}
}

func TestManager_GetNextPrereleaseVersion(t *testing.T) {
	testCases := []struct {
		name            string
		baseVersion     string
		existingNumbers []int
		expectedVersion string
		expectError     bool
	}{
		{
			name:            "first prerelease",
			baseVersion:     "v1.0.0",
			existingNumbers: []int{},
			expectedVersion: "v1.0.0-prerelease01",
			expectError:     false,
		},
		{
			name:            "increment existing prerelease",
			baseVersion:     "v1.0.0",
			existingNumbers: []int{10, 11, 15},
			expectedVersion: "v1.0.0-prerelease16",
			expectError:     false,
		},
		{
			name:            "single digit error",
			baseVersion:     "v1.0.0",
			existingNumbers: []int{8},
			expectError:     true,
		},
		{
			name:            "max prerelease exceeded",
			baseVersion:     "v1.0.0",
			existingNumbers: []int{99},
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &Manager{
				tagCache: &TagCache{
					PrereleaseCache: map[string][]int{
						tc.baseVersion: tc.existingNumbers,
					},
				},
			}

			result, err := manager.GetNextPrereleaseVersion(tc.baseVersion)

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedVersion, result)
		})
	}
}

func TestManager_FindModules(t *testing.T) {
	testCases := []struct {
		name            string
		goModPaths      []string
		fsError         error
		excludedDirs    []string
		expectedModules int
		expectError     bool
	}{
		{
			name: "multiple modules found",
			goModPaths: []string{
				"test/repo",
				"test/repo/service1",
				"test/repo/service2",
			},
			expectedModules: 3,
			expectError:     false,
		},
		{
			name: "modules with exclusions",
			goModPaths: []string{
				".",
				"./service1",
				"./vendor",
				"./test",
			},
			excludedDirs:    []string{"vendor", "test"},
			expectedModules: 2,
			expectError:     false,
		},
		{
			name:            "filesystem error",
			goModPaths:      nil,
			fsError:         errors.New("filesystem error"),
			expectedModules: 0,
			expectError:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := &Config{
				RepoRoot:     "/test/repo",
				ExcludedDirs: tc.excludedDirs,
			}
			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)
			manager.tagCache = &TagCache{
				ModuleTags: make(map[string][]ParsedTag),
			}

			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test/repo").Return(tc.goModPaths, tc.fsError)

			modules, err := manager.FindModules(context.Background())

			if tc.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, modules, tc.expectedModules)

			// Verify module structure
			for _, module := range modules {
				assert.NotEmpty(t, module.Version)
			}
		})
	}
}

func TestManager_CheckVersionExists(t *testing.T) {
	modules := []Module{
		{Path: "", Version: "v1.0.0"},
		{Path: "service1", Version: "v1.0.0"},
	}

	testCases := []struct {
		name         string
		version      string
		existingTags []ParsedTag
		expectError  bool
	}{
		{
			name:    "version does not exist",
			version: "v2.0.0",
			existingTags: []ParsedTag{
				{Raw: "v1.0.0"},
				{Raw: "service1/v1.0.0"},
			},
			expectError: false,
		},
		{
			name:    "version exists for root module",
			version: "v1.0.0",
			existingTags: []ParsedTag{
				{Raw: "v1.0.0"},
				{Raw: "service1/v1.0.0"},
			},
			expectError: true,
		},
		{
			name:    "version exists for submodule",
			version: "v2.0.0",
			existingTags: []ParsedTag{
				{Raw: "v1.0.0"},
				{Raw: "service1/v2.0.0"},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := &Manager{
				tagCache: &TagCache{
					AllTags: tc.existingTags,
				},
			}

			err := manager.CheckVersionExists(tc.version, modules)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_AssessCurrentState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGit(ctrl)
	mockFS := NewMockFS(ctrl)
	mockInteraction := NewMockUserInteraction(ctrl)

	config := &Config{RepoRoot: "/test/repo"}
	manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

	// Setup mocks
	mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("main", nil)
	mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test/repo").Return(
		[]string{"/test/repo"}, nil)

	// Setup tag cache
	version, _ := semver.NewVersion("v1.0.0")
	manager.tagCache = &TagCache{
		HighestVersion: version,
		ModuleTags:     make(map[string][]ParsedTag),
	}

	state, err := manager.AssessCurrentState(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "main", state.CurrentBranch)
	assert.Equal(t, "v1.0.0", state.CurrentVersion)
	assert.Len(t, state.Modules, 1)
	assert.NotNil(t, state.TagCache)
}

func TestManager_InteractiveRelease(t *testing.T) {
	testCases := []struct {
		name              string
		skipConfirmations bool
		userConfirmsTag   bool
		userConfirmsPush  bool
		expectTagCreation bool
		expectTagPushing  bool
		expectError       bool
	}{
		{
			name:              "full interactive flow - user confirms both",
			skipConfirmations: false,
			userConfirmsTag:   true,
			userConfirmsPush:  true,
			expectTagCreation: true,
			expectTagPushing:  true,
			expectError:       false,
		},
		{
			name:              "user cancels tag creation",
			skipConfirmations: false,
			userConfirmsTag:   false,
			userConfirmsPush:  false,
			expectTagCreation: false,
			expectTagPushing:  false,
			expectError:       true,
		},
		{
			name:              "user creates tags but cancels push",
			skipConfirmations: false,
			userConfirmsTag:   true,
			userConfirmsPush:  false,
			expectTagCreation: true,
			expectTagPushing:  false,
			expectError:       false,
		},
		{
			name:              "skip all confirmations",
			skipConfirmations: true,
			expectTagCreation: true,
			expectTagPushing:  true,
			expectError:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := &Config{
				RepoRoot:          "/test/repo",
				SkipConfirmations: tc.skipConfirmations,
			}
			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks for state assessment
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("main", nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test/repo").Return(
				[]string{"."}, nil)

			// Setup tag cache
			manager.tagCache = &TagCache{
				AllTags:    []ParsedTag{},
				ModuleTags: make(map[string][]ParsedTag),
			}

			// Setup interaction expectations
			if !tc.skipConfirmations {
				mockInteraction.EXPECT().Confirm(gomock.Any(), "Create tags?").
					Return(tc.userConfirmsTag, nil)

				if tc.userConfirmsTag {
					mockInteraction.EXPECT().Confirm(gomock.Any(), "Push tags?").
						Return(tc.userConfirmsPush, nil)
				}
			}

			// Setup git operation expectations
			if tc.expectTagCreation {
				mockGit.EXPECT().CreateTag(gomock.Any(), "v1.0.0").Return(nil)
			}

			if tc.expectTagPushing {
				mockGit.EXPECT().PushTag(gomock.Any(), "v1.0.0").Return(nil)
			}

			err := manager.InteractiveRelease(context.Background(), "v1.0.0")

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_Run(t *testing.T) {
	testCases := []struct {
		name        string
		config      *Config
		expectShow  bool
		expectError bool
		setupMocks  func(*MockGit, *MockFS, *MockUserInteraction)
	}{
		{
			name: "show current state mode",
			config: &Config{
				RepoRoot: "/test/repo",
				// No version, type, or prerelease - should show state
			},
			expectShow:  true,
			expectError: false,
			setupMocks: func(git *MockGit, fs *MockFS, ui *MockUserInteraction) {
				git.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.0.0"}, nil)
				git.EXPECT().GetCurrentBranch(gomock.Any()).Return("main", nil)
				fs.EXPECT().FindGoModFiles(gomock.Any(), "/test/repo").Return(
					[]string{"/test/repo"}, nil)
			},
		},
		{
			name: "interactive release mode",
			config: &Config{
				RepoRoot:          "/test/repo",
				Version:           "2.0.0",
				SkipConfirmations: true,
			},
			expectShow:  false,
			expectError: false,
			setupMocks: func(git *MockGit, fs *MockFS, ui *MockUserInteraction) {
				git.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.0.0"}, nil)
				git.EXPECT().GetCurrentBranch(gomock.Any()).Return("main", nil)
				fs.EXPECT().FindGoModFiles(gomock.Any(), "/test/repo").Return(
					[]string{"."}, nil)
				git.EXPECT().CreateTag(gomock.Any(), "v2.0.0").Return(nil)
				git.EXPECT().PushTag(gomock.Any(), "v2.0.0").Return(nil)
			},
		},
		{
			name: "git error during setup",
			config: &Config{
				RepoRoot: "/test/repo",
				Version:  "2.0.0",
			},
			expectShow:  false,
			expectError: true,
			setupMocks: func(git *MockGit, fs *MockFS, ui *MockUserInteraction) {
				git.EXPECT().GetTags(gomock.Any()).Return(nil, errors.New("git error"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			manager := NewReleaseManager(tc.config, mockGit, mockFS, mockInteraction)

			tc.setupMocks(mockGit, mockFS, mockInteraction)

			err := manager.Run(context.Background())

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "version with v prefix",
			input:    "v1.2.3",
			expected: "v1.2.3",
			hasError: false,
		},
		{
			name:     "version without v prefix",
			input:    "1.2.3",
			expected: "v1.2.3",
			hasError: false,
		},
		{
			name:     "invalid version",
			input:    "not.a.version",
			expected: "",
			hasError: true,
		},
		{
			name:     "prerelease version",
			input:    "1.2.3-prerelease01",
			expected: "v1.2.3-prerelease01",
			hasError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeVersion(tc.input)

			if tc.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestIncrementVersion(t *testing.T) {
	testCases := []struct {
		name        string
		current     string
		versionType string
		expected    string
		hasError    bool
	}{
		{
			name:        "major increment",
			current:     "v1.2.3",
			versionType: "major",
			expected:    "v2.0.0",
			hasError:    false,
		},
		{
			name:        "minor increment",
			current:     "v1.2.3",
			versionType: "minor",
			expected:    "v1.3.0",
			hasError:    false,
		},
		{
			name:        "patch increment",
			current:     "v1.2.3",
			versionType: "patch",
			expected:    "v1.2.4",
			hasError:    false,
		},
		{
			name:        "invalid version type",
			current:     "v1.2.3",
			versionType: "invalid",
			expected:    "",
			hasError:    true,
		},
		{
			name:        "invalid current version",
			current:     "invalid",
			versionType: "patch",
			expected:    "",
			hasError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := IncrementVersion(tc.current, tc.versionType)

			if tc.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}

func TestManager_getTagName(t *testing.T) {
	manager := &Manager{}

	testCases := []struct {
		name     string
		module   Module
		version  string
		expected string
	}{
		{
			name: "root module",
			module: Module{
				Path: "",
			},
			version:  "v1.0.0",
			expected: "v1.0.0",
		},
		{
			name: "submodule",
			module: Module{
				Path: "service1",
			},
			version:  "v1.0.0",
			expected: "service1/v1.0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := manager.getTagName(tc.module, tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestManager_shouldExcludeModule(t *testing.T) {
	manager := &Manager{
		config: &Config{
			ExcludedDirs: []string{"vendor", "test", "internal/test"},
		},
	}

	testCases := []struct {
		name     string
		relPath  string
		expected bool
	}{
		{
			name:     "not excluded",
			relPath:  "service1",
			expected: false,
		},
		{
			name:     "excluded exact match",
			relPath:  "vendor",
			expected: true,
		},
		{
			name:     "excluded subdirectory",
			relPath:  "vendor/something",
			expected: true,
		},
		{
			name:     "excluded nested path",
			relPath:  "internal/test/module",
			expected: true,
		},
		{
			name:     "similar but not excluded",
			relPath:  "testing",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := manager.shouldExcludeModule(tc.relPath)
			assert.Equal(t, tc.expected, result)
		})
	}
}
