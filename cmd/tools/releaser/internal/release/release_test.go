package release

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestManager_RunRelease(t *testing.T) {
	tests := []struct {
		name           string
		currentTags    []string
		expectedTarget string
		shouldError    bool
		errorMsg       string
	}{
		{
			name:           "successful release from prerelease",
			currentTags:    []string{"v1.2.3-prerelease01"},
			expectedTarget: "v1.2.3",
			shouldError:    false,
		},
		{
			name:        "error when no prerelease exists",
			currentTags: []string{"v1.2.3"},
			shouldError: true,
			errorMsg:    "release command requires existing prerelease version",
		},
		{
			name:           "release from higher prerelease number",
			currentTags:    []string{"v1.2.3-prerelease05"},
			expectedTarget: "v1.2.3",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "release",
				SkipConfirmations: true, // Skip confirmations for testing
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)

			if !tt.shouldError {
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
				mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedTarget).Return(nil)
				mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedTarget).Return(nil)
			}

			err := manager.RunRelease(context.Background())

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManager_RunMinor(t *testing.T) {
	tests := []struct {
		name           string
		currentTags    []string
		expectedTarget string
		shouldError    bool
		errorMsg       string
	}{
		{
			name:           "successful minor from stable",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v1.3.0-prerelease01",
			shouldError:    false,
		},
		{
			name:        "error when current is prerelease",
			currentTags: []string{"v1.2.3-prerelease01"},
			shouldError: true,
			errorMsg:    "minor/major commands should be run from stable versions",
		},
		{
			name:           "minor from initial version",
			currentTags:    []string{},
			expectedTarget: "v0.1.0-prerelease01",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "minor",
				SkipConfirmations: true,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)

			if !tt.shouldError {
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
				mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedTarget).Return(nil)
				mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedTarget).Return(nil)
			}

			err := manager.RunMinor(context.Background())

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManager_RunMajor(t *testing.T) {
	tests := []struct {
		name           string
		currentTags    []string
		expectedTarget string
		shouldError    bool
	}{
		{
			name:           "successful major from stable",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v2.0.0-prerelease01",
			shouldError:    false,
		},
		{
			name:           "major from initial version",
			currentTags:    []string{},
			expectedTarget: "v1.0.0-prerelease01",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "major",
				SkipConfirmations: true,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
			mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedTarget).Return(nil)
			mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedTarget).Return(nil)

			err := manager.RunMajor(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestManager_RunPrerelease(t *testing.T) {
	tests := []struct {
		name           string
		currentTags    []string
		expectedTarget string
	}{
		{
			name:           "increment prerelease number",
			currentTags:    []string{"v1.2.3-prerelease01"},
			expectedTarget: "v1.2.3-prerelease02",
		},
		{
			name:           "create first prerelease from stable",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v1.2.3-prerelease01",
		},
		{
			name:           "increment from higher prerelease",
			currentTags:    []string{"v1.2.3-prerelease05"},
			expectedTarget: "v1.2.3-prerelease06",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "prerelease",
				SkipConfirmations: true,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
			mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedTarget).Return(nil)
			mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedTarget).Return(nil)

			err := manager.RunPrerelease(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestManager_ManualVersionOverride(t *testing.T) {
	tests := []struct {
		name           string
		manualVersion  string
		currentTags    []string
		expectedTarget string
		shouldError    bool
		errorMsg       string
	}{
		{
			name:           "valid manual version",
			manualVersion:  "v2.5.0",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v2.5.0",
			shouldError:    false,
		},
		{
			name:           "manual version without v prefix",
			manualVersion:  "2.5.0",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v2.5.0",
			shouldError:    false,
		},
		{
			name:          "invalid manual version",
			manualVersion: "invalid",
			currentTags:   []string{"v1.2.3"},
			shouldError:   true,
			errorMsg:      "invalid manual version format",
		},
		{
			name:           "manual prerelease version",
			manualVersion:  "v2.5.0-prerelease03",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v2.5.0-prerelease03",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "release",
				SkipConfirmations: true,
				ManualVersion:     tt.manualVersion,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)

			if !tt.shouldError {
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
				mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedTarget).Return(nil)
				mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedTarget).Return(nil)
			}

			err := manager.RunRelease(context.Background())

			if tt.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestManager_ConflictResolution(t *testing.T) {
	tests := []struct {
		name           string
		targetVersion  string
		existingTags   []string
		modules        []Module
		expectedCreate []string
		expectedSkip   []string
		shouldError    bool
	}{
		{
			name:          "partial conflict - some modules have version",
			targetVersion: "v1.3.0",
			existingTags:  []string{"moduleA/v1.3.0"},
			modules: []Module{
				{Path: "moduleA", Version: "v1.2.0"},
				{Path: "moduleB", Version: "v1.2.0"},
				{Path: "", Version: "v1.2.0"}, // root module
			},
			expectedCreate: []string{"moduleB/v1.3.0", "v1.3.0"},
			expectedSkip:   []string{"moduleA/v1.3.0"},
			shouldError:    false,
		},
		{
			name:          "complete conflict - all modules have version",
			targetVersion: "v1.3.0",
			existingTags:  []string{"moduleA/v1.3.0", "moduleB/v1.3.0", "v1.3.0"},
			modules: []Module{
				{Path: "moduleA", Version: "v1.2.0"},
				{Path: "moduleB", Version: "v1.2.0"},
				{Path: "", Version: "v1.2.0"},
			},
			expectedCreate: []string{},
			expectedSkip:   []string{"moduleA/v1.3.0", "moduleB/v1.3.0", "v1.3.0"},
			shouldError:    true,
		},
		{
			name:          "no conflict",
			targetVersion: "v1.3.0",
			existingTags:  []string{},
			modules: []Module{
				{Path: "moduleA", Version: "v1.2.0"},
				{Path: "moduleB", Version: "v1.2.0"},
			},
			expectedCreate: []string{"moduleA/v1.3.0", "moduleB/v1.3.0"},
			expectedSkip:   []string{},
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "release",
				SkipConfirmations: true,
				ManualVersion:     tt.targetVersion,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup tag cache
			manager.tagCache = &TagCache{
				AllTags: make([]ParsedTag, 0),
			}

			// Parse existing tags
			for _, tag := range tt.existingTags {
				manager.tagCache.AllTags = append(manager.tagCache.AllTags, ParsedTag{Raw: tag})
			}

			conflictInfo, err := manager.CheckVersionExists(tt.targetVersion, tt.modules)

			if tt.shouldError && len(tt.expectedCreate) == 0 {
				require.Error(t, err)
				assert.ElementsMatch(t, tt.expectedSkip, conflictInfo.ExistingTags)
				assert.Empty(t, conflictInfo.MissingTags)
			} else {
				if len(tt.expectedSkip) > 0 {
					require.Error(t, err) // Should have conflict error
					assert.ElementsMatch(t, tt.expectedSkip, conflictInfo.ExistingTags)
				} else {
					require.NoError(t, err) // No conflict
				}
				assert.ElementsMatch(t, tt.expectedCreate, conflictInfo.MissingTags)
			}
		})
	}
}

func TestManager_MultiModuleOperations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockGit := NewMockGit(ctrl)
	mockFS := NewMockFS(ctrl)
	mockInteraction := NewMockUserInteraction(ctrl)

	config := Config{
		RepoRoot:          "/test",
		ExcludedDirs:      []string{"cmd", "internal/tools"},
		RequiredBranch:    "master",
		Verbose:           false,
		Command:           "minor",
		SkipConfirmations: true,
	}

	manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

	// Setup mocks for multi-module repo
	currentTags := []string{
		"v1.2.3",
		"service1/v1.2.3",
		"service2/v1.2.3",
	}
	goModPaths := []string{".", "service1", "service2", "cmd/tool", "internal/tools/helper"}

	expectedTags := []string{
		"v1.3.0-prerelease01",
		"service1/v1.3.0-prerelease01",
		"service2/v1.3.0-prerelease01",
	}

	mockGit.EXPECT().GetTags(gomock.Any()).Return(currentTags, nil)
	mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
	mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return(goModPaths, nil)

	// Expect tag creation for non-excluded modules only
	for _, tag := range expectedTags {
		mockGit.EXPECT().CreateTag(gomock.Any(), tag).Return(nil)
		mockGit.EXPECT().PushTag(gomock.Any(), tag).Return(nil)
	}

	err := manager.RunMinor(context.Background())
	require.NoError(t, err)
}

func TestManager_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*MockGit, *MockFS, *MockUserInteraction)
		expectedErr string
	}{
		{
			name: "git tags fetch error",
			setupMock: func(mockGit *MockGit, mockFS *MockFS, mockInteraction *MockUserInteraction) {
				mockGit.EXPECT().GetTags(gomock.Any()).Return(nil, errors.New("git error"))
			},
			expectedErr: "failed to fetch tags",
		},
		{
			name: "git branch fetch error",
			setupMock: func(mockGit *MockGit, mockFS *MockFS, mockInteraction *MockUserInteraction) {
				mockGit.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.2.3"}, nil)
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("", errors.New("branch error"))
			},
			expectedErr: "get current branch",
		},
		{
			name: "fs find modules error",
			setupMock: func(mockGit *MockGit, mockFS *MockFS, mockInteraction *MockUserInteraction) {
				mockGit.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.2.3"}, nil)
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return(nil, errors.New("fs error"))
			},
			expectedErr: "find modules",
		},
		{
			name: "tag creation error",
			setupMock: func(mockGit *MockGit, mockFS *MockFS, mockInteraction *MockUserInteraction) {
				mockGit.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.2.3"}, nil)
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
				mockGit.EXPECT().CreateTag(gomock.Any(), "v1.3.0-prerelease01").Return(errors.New("tag error"))
			},
			expectedErr: "failed to create tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "minor",
				SkipConfirmations: true,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			tt.setupMock(mockGit, mockFS, mockInteraction)

			err := manager.RunMinor(context.Background())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestManager_RunPatch(t *testing.T) {
	tests := []struct {
		name           string
		currentTags    []string
		expectedTarget string
		shouldError    bool
	}{
		{
			name:           "successful patch from stable",
			currentTags:    []string{"v1.2.3"},
			expectedTarget: "v1.2.4-prerelease01",
			shouldError:    false,
		},
		{
			name:           "patch from initial version",
			currentTags:    []string{},
			expectedTarget: "v0.0.1-prerelease01",
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGit(ctrl)
			mockFS := NewMockFS(ctrl)
			mockInteraction := NewMockUserInteraction(ctrl)

			config := Config{
				RepoRoot:          "/test",
				ExcludedDirs:      []string{},
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           "patch",
				SkipConfirmations: true,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
			mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedTarget).Return(nil)
			mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedTarget).Return(nil)

			err := manager.RunPatch(context.Background())
			require.NoError(t, err)
		})
	}
}
