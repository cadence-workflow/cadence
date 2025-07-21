package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateReleaseCommand(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		shouldError    bool
		errorMsg       string
	}{
		{
			name:           "valid prerelease version",
			currentVersion: "v1.2.3-prerelease01",
			shouldError:    false,
		},
		{
			name:           "valid higher prerelease",
			currentVersion: "v2.5.0-prerelease15",
			shouldError:    false,
		},
		{
			name:           "invalid stable version",
			currentVersion: "v1.2.3",
			shouldError:    true,
			errorMsg:       "release command requires existing prerelease version",
		},
		{
			name:           "invalid initial version",
			currentVersion: "v0.0.0",
			shouldError:    true,
			errorMsg:       "release command requires existing prerelease version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			err := manager.validateReleaseCommand(tt.currentVersion)

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMinorMajorCommand(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		shouldError    bool
		errorMsg       string
	}{
		{
			name:           "valid stable version",
			currentVersion: "v1.2.3",
			shouldError:    false,
		},
		{
			name:           "valid initial version",
			currentVersion: "v0.0.0",
			shouldError:    false,
		},
		{
			name:           "invalid prerelease version",
			currentVersion: "v1.2.3-prerelease01",
			shouldError:    true,
			errorMsg:       "minor/major commands should be run from stable versions",
		},
		{
			name:           "invalid higher prerelease",
			currentVersion: "v2.5.0-prerelease15",
			shouldError:    true,
			errorMsg:       "minor/major commands should be run from stable versions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			err := manager.validateMinorMajorCommand(tt.currentVersion)

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProcessManualVersion(t *testing.T) {
	tests := []struct {
		name          string
		manualVersion string
		expected      string
		shouldError   bool
		errorMsg      string
	}{
		{
			name:          "valid version with v",
			manualVersion: "v1.2.3",
			expected:      "v1.2.3",
			shouldError:   false,
		},
		{
			name:          "valid version without v",
			manualVersion: "1.2.3",
			expected:      "v1.2.3",
			shouldError:   false,
		},
		{
			name:          "valid prerelease with v",
			manualVersion: "v2.0.0-prerelease01",
			expected:      "v2.0.0-prerelease01",
			shouldError:   false,
		},
		{
			name:          "valid prerelease without v",
			manualVersion: "2.0.0-prerelease01",
			expected:      "v2.0.0-prerelease01",
			shouldError:   false,
		},
		{
			name:          "auto-complete version format",
			manualVersion: "1.2",
			expected:      "v1.2.0",
			shouldError:   false,
		},
		{
			name:          "completely invalid",
			manualVersion: "not-a-version",
			shouldError:   true,
			errorMsg:      "invalid manual version format",
		},
		{
			name:          "empty version",
			manualVersion: "",
			shouldError:   true,
			errorMsg:      "invalid manual version format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			result, err := manager.processManualVersion(tt.manualVersion)

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

func TestShouldExcludeModule(t *testing.T) {
	tests := []struct {
		name          string
		excludedDirs  []string
		modulePath    string
		shouldExclude bool
	}{
		{
			name:          "not excluded",
			excludedDirs:  []string{"cmd", "internal/tools"},
			modulePath:    "service1",
			shouldExclude: false,
		},
		{
			name:          "exact match",
			excludedDirs:  []string{"cmd", "internal/tools"},
			modulePath:    "cmd",
			shouldExclude: true,
		},
		{
			name:          "prefix match",
			excludedDirs:  []string{"cmd", "internal/tools"},
			modulePath:    "cmd/tool1",
			shouldExclude: true,
		},
		{
			name:          "nested prefix match",
			excludedDirs:  []string{"cmd", "internal/tools"},
			modulePath:    "internal/tools/helper",
			shouldExclude: true,
		},
		{
			name:          "partial match should not exclude",
			excludedDirs:  []string{"cmd", "internal/tools"},
			modulePath:    "cmdline",
			shouldExclude: false,
		},
		{
			name:          "empty excluded dirs",
			excludedDirs:  []string{},
			modulePath:    "any/path",
			shouldExclude: false,
		},
		{
			name:          "root module",
			excludedDirs:  []string{"cmd"},
			modulePath:    "",
			shouldExclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				config: Config{
					ExcludedDirs: tt.excludedDirs,
				},
			}

			result := manager.shouldExcludeModule(tt.modulePath)
			assert.Equal(t, tt.shouldExclude, result)
		})
	}
}

func TestGetTagName(t *testing.T) {
	tests := []struct {
		name     string
		module   Module
		version  string
		expected string
	}{
		{
			name:     "root module",
			module:   Module{Path: "", Version: "v1.2.3"},
			version:  "v1.3.0",
			expected: "v1.3.0",
		},
		{
			name:     "submodule",
			module:   Module{Path: "service1", Version: "v1.2.3"},
			version:  "v1.3.0",
			expected: "service1/v1.3.0",
		},
		{
			name:     "nested module",
			module:   Module{Path: "services/auth", Version: "v1.2.3"},
			version:  "v2.0.0",
			expected: "services/auth/v2.0.0",
		},
		{
			name:     "prerelease version",
			module:   Module{Path: "service1", Version: "v1.2.3"},
			version:  "v1.3.0-prerelease01",
			expected: "service1/v1.3.0-prerelease01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			result := manager.getTagName(tt.module, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLatestVersionForModule(t *testing.T) {
	tests := []struct {
		name       string
		modulePath string
		tags       []string
		expected   string
	}{
		{
			name:       "no tags for module",
			modulePath: "service1",
			tags:       []string{},
			expected:   "v0.0.0",
		},
		{
			name:       "single tag",
			modulePath: "service1",
			tags:       []string{"service1/v1.2.3"},
			expected:   "v1.2.3",
		},
		{
			name:       "multiple tags - choose latest",
			modulePath: "service1",
			tags:       []string{"service1/v1.2.3", "service1/v1.1.0", "service1/v2.0.0"},
			expected:   "v2.0.0",
		},
		{
			name:       "mix of stable and prerelease",
			modulePath: "service1",
			tags:       []string{"service1/v1.2.3", "service1/v1.3.0-prerelease01"},
			expected:   "v1.3.0-prerelease01",
		},
		{
			name:       "root module",
			modulePath: "",
			tags:       []string{"v1.2.3", "v1.1.0", "v2.0.0"},
			expected:   "v2.0.0",
		},
		{
			name:       "no cache initialized",
			modulePath: "service1",
			tags:       nil, // Will simulate uninitialized cache
			expected:   "v0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}

			if tt.tags != nil {
				// Initialize cache
				manager.tagCache = &TagCache{
					ModuleTags: make(map[string][]ParsedTag),
				}

				// Parse and add tags
				for _, tag := range tt.tags {
					parsed := manager.parseTag(tag)
					if parsed.Version != nil {
						if manager.tagCache.ModuleTags[parsed.ModulePath] == nil {
							manager.tagCache.ModuleTags[parsed.ModulePath] = make([]ParsedTag, 0)
						}
						manager.tagCache.ModuleTags[parsed.ModulePath] = append(
							manager.tagCache.ModuleTags[parsed.ModulePath], parsed)
					}
				}
			}

			result := manager.getLatestVersionForModule(tt.modulePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterActionsForMissingTags(t *testing.T) {
	tests := []struct {
		name        string
		actions     []Action
		missingTags []string
		expected    []Action
	}{
		{
			name: "filter some actions",
			actions: []Action{
				{Type: ActionCreateTag, Target: "v1.3.0"},
				{Type: ActionPushTags, Target: "v1.3.0"},
				{Type: ActionCreateTag, Target: "service1/v1.3.0"},
				{Type: ActionPushTags, Target: "service1/v1.3.0"},
				{Type: ActionCreateTag, Target: "service2/v1.3.0"},
				{Type: ActionPushTags, Target: "service2/v1.3.0"},
			},
			missingTags: []string{"v1.3.0", "service2/v1.3.0"},
			expected: []Action{
				{Type: ActionCreateTag, Target: "v1.3.0"},
				{Type: ActionPushTags, Target: "v1.3.0"},
				{Type: ActionCreateTag, Target: "service2/v1.3.0"},
				{Type: ActionPushTags, Target: "service2/v1.3.0"},
			},
		},
		{
			name: "no missing tags",
			actions: []Action{
				{Type: ActionCreateTag, Target: "v1.3.0"},
				{Type: ActionPushTags, Target: "v1.3.0"},
			},
			missingTags: []string{},
			expected:    []Action(nil),
		},
		{
			name:        "no actions",
			actions:     []Action{},
			missingTags: []string{"v1.3.0"},
			expected:    []Action(nil),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{}
			result := manager.filterActionsForMissingTags(tt.actions, tt.missingTags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateWithWarnings(t *testing.T) {
	tests := []struct {
		name             string
		currentBranch    string
		requiredBranch   string
		expectedWarnings int
		expectedMessage  string
	}{
		{
			name:             "correct branch",
			currentBranch:    "master",
			requiredBranch:   "master",
			expectedWarnings: 0,
		},
		{
			name:             "wrong branch",
			currentBranch:    "feature-branch",
			requiredBranch:   "master",
			expectedWarnings: 1,
			expectedMessage:  "you are not on master",
		},
		{
			name:             "no required branch",
			currentBranch:    "any-branch",
			requiredBranch:   "",
			expectedWarnings: 0,
		},
		{
			name:             "different wrong branch",
			currentBranch:    "develop",
			requiredBranch:   "main",
			expectedWarnings: 1,
			expectedMessage:  "you are not on main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				config: Config{
					RequiredBranch: tt.requiredBranch,
				},
			}

			state := &State{
				CurrentBranch: tt.currentBranch,
			}

			warnings := manager.validateWithWarnings(state, "v1.3.0")

			assert.Len(t, warnings, tt.expectedWarnings)
			if tt.expectedWarnings > 0 {
				assert.Contains(t, warnings[0].Message, tt.expectedMessage)
				assert.Equal(t, WrongBranch, warnings[0].Type)
			}
		})
	}
}

func TestFindModules(t *testing.T) {
	tests := []struct {
		name         string
		goModPaths   []string
		excludedDirs []string
		expected     []Module
	}{
		{
			name:         "single root module",
			goModPaths:   []string{"."},
			excludedDirs: []string{},
			expected: []Module{
				{Path: "", Version: "v0.0.0"},
			},
		},
		{
			name:         "multiple modules",
			goModPaths:   []string{".", "service1", "service2"},
			excludedDirs: []string{},
			expected: []Module{
				{Path: "", Version: "v0.0.0"},
				{Path: "service1", Version: "v0.0.0"},
				{Path: "service2", Version: "v0.0.0"},
			},
		},
		{
			name:         "modules with exclusions",
			goModPaths:   []string{".", "service1", "cmd/tool", "internal/tools/helper"},
			excludedDirs: []string{"cmd", "internal/tools"},
			expected: []Module{
				{Path: "", Version: "v0.0.0"},
				{Path: "service1", Version: "v0.0.0"},
			},
		},
		{
			name:         "deduplicate paths",
			goModPaths:   []string{".", "./", "service1", "./service1"},
			excludedDirs: []string{},
			expected: []Module{
				{Path: "", Version: "v0.0.0"},
				{Path: "service1", Version: "v0.0.0"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFS := NewMockFS(ctrl)
			manager := &Manager{
				config: Config{
					RepoRoot:     "/test",
					ExcludedDirs: tt.excludedDirs,
				},
				fs: mockFS,
				tagCache: &TagCache{
					ModuleTags: make(map[string][]ParsedTag),
				},
			}

			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return(tt.goModPaths, nil)

			result, err := manager.FindModules(context.Background())
			require.NoError(t, err)

			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestHandleWarningsAndConfirmations(t *testing.T) {
	tests := []struct {
		name              string
		warnings          []Warning
		skipConfirmations bool
		userResponses     []bool
		shouldError       bool
		errorMsg          string
	}{
		{
			name:              "no warnings",
			warnings:          []Warning{},
			skipConfirmations: false,
			shouldError:       false,
		},
		{
			name: "skip confirmations with warnings",
			warnings: []Warning{
				{Type: WrongBranch, Message: "you are not on master"},
			},
			skipConfirmations: true,
			shouldError:       false,
		},
		{
			name: "user confirms warning",
			warnings: []Warning{
				{Type: WrongBranch, Message: "you are not on master"},
			},
			skipConfirmations: false,
			userResponses:     []bool{true},
			shouldError:       false,
		},
		{
			name: "user rejects warning",
			warnings: []Warning{
				{Type: WrongBranch, Message: "you are not on master"},
			},
			skipConfirmations: false,
			userResponses:     []bool{false},
			shouldError:       true,
			errorMsg:          "operation cancelled due to: you are not on master",
		},
		{
			name: "multiple warnings - all confirmed",
			warnings: []Warning{
				{Type: WrongBranch, Message: "you are not on master"},
				{Type: ExistingTags, Message: "some tags exist"},
			},
			skipConfirmations: false,
			userResponses:     []bool{true, true},
			shouldError:       false,
		},
		{
			name: "multiple warnings - second rejected",
			warnings: []Warning{
				{Type: WrongBranch, Message: "you are not on master"},
				{Type: ExistingTags, Message: "some tags exist"},
			},
			skipConfirmations: false,
			userResponses:     []bool{true, false},
			shouldError:       true,
			errorMsg:          "operation cancelled due to: some tags exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockInteraction := NewMockUserInteraction(ctrl)
			manager := &Manager{
				config: Config{
					SkipConfirmations: tt.skipConfirmations,
				},
				interaction: mockInteraction,
			}

			// Setup mock expectations
			if !tt.skipConfirmations && len(tt.warnings) > 0 {
				for i, response := range tt.userResponses {
					if i < len(tt.warnings) {
						mockInteraction.EXPECT().
							ConfirmWithDefault(gomock.Any(), "Continue?", false).
							Return(response, nil)

						if !response {
							break // Stop after first rejection
						}
					}
				}
			}

			err := manager.handleWarningsAndConfirmations(context.Background(), tt.warnings)

			if tt.shouldError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
