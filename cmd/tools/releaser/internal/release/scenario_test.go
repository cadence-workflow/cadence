package release

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCompleteReleaseWorkflow(t *testing.T) {
	tests := []struct {
		name        string
		workflow    []workflowStep
		description string
	}{
		{
			name:        "feature development cycle",
			description: "Complete feature development from minor through prerelease iterations to final release",
			workflow: []workflowStep{
				{
					command:       "minor",
					currentTags:   []string{"v1.2.3"},
					expectedTag:   "v1.3.0-prerelease01",
					expectedError: false,
				},
				{
					command:       "prerelease",
					currentTags:   []string{"v1.2.3", "v1.3.0-prerelease01"},
					expectedTag:   "v1.3.0-prerelease02",
					expectedError: false,
				},
				{
					command:       "prerelease",
					currentTags:   []string{"v1.2.3", "v1.3.0-prerelease01", "v1.3.0-prerelease02"},
					expectedTag:   "v1.3.0-prerelease03",
					expectedError: false,
				},
				{
					command:       "release",
					currentTags:   []string{"v1.2.3", "v1.3.0-prerelease01", "v1.3.0-prerelease02", "v1.3.0-prerelease03"},
					expectedTag:   "v1.3.0",
					expectedError: false,
				},
			},
		},
		{
			name:        "major version cycle",
			description: "Major version development cycle with breaking changes",
			workflow: []workflowStep{
				{
					command:       "major",
					currentTags:   []string{"v1.3.0"},
					expectedTag:   "v2.0.0-prerelease01",
					expectedError: false,
				},
				{
					command:       "prerelease",
					currentTags:   []string{"v1.3.0", "v2.0.0-prerelease01"},
					expectedTag:   "v2.0.0-prerelease02",
					expectedError: false,
				},
				{
					command:       "release",
					currentTags:   []string{"v1.3.0", "v2.0.0-prerelease01", "v2.0.0-prerelease02"},
					expectedTag:   "v2.0.0",
					expectedError: false,
				},
			},
		},
		{
			name:        "initial repository setup",
			description: "Starting from empty repository to first release",
			workflow: []workflowStep{
				{
					command:       "minor",
					currentTags:   []string{},
					expectedTag:   "v0.1.0-prerelease01",
					expectedError: false,
				},
				{
					command:       "release",
					currentTags:   []string{"v0.1.0-prerelease01"},
					expectedTag:   "v0.1.0",
					expectedError: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing workflow: %s", tt.description)

			for i, step := range tt.workflow {
				t.Logf("Step %d: %s command", i+1, step.command)

				ctrl := gomock.NewController(t)

				mockGit := NewMockGit(ctrl)
				mockFS := NewMockFS(ctrl)
				mockInteraction := NewMockUserInteraction(ctrl)

				config := Config{
					RepoRoot:          "/test",
					ExcludedDirs:      []string{},
					RequiredBranch:    "master",
					Verbose:           false,
					Command:           step.command,
					SkipConfirmations: true,
				}

				manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

				// Setup mocks
				mockGit.EXPECT().GetTags(gomock.Any()).Return(step.currentTags, nil)

				if !step.expectedError {
					mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
					mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
					mockGit.EXPECT().CreateTag(gomock.Any(), step.expectedTag).Return(nil)
					mockGit.EXPECT().PushTag(gomock.Any(), step.expectedTag).Return(nil)
				}

				// Execute command
				var err error
				switch step.command {
				case "minor":
					err = manager.RunMinor(context.Background())
				case "major":
					err = manager.RunMajor(context.Background())
				case "prerelease":
					err = manager.RunPrerelease(context.Background())
				case "release":
					err = manager.RunRelease(context.Background())
				}

				if step.expectedError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}

				ctrl.Finish()
			}
		})
	}
}

type workflowStep struct {
	command       string
	currentTags   []string
	expectedTag   string
	expectedError bool
}

func TestMultiModuleComplexScenarios(t *testing.T) {
	tests := []struct {
		name            string
		currentTags     []string
		modules         []string
		command         string
		expectedTags    []string
		excludedModules []string
	}{
		{
			name: "multi-module minor release",
			currentTags: []string{
				"v1.2.3",
				"service1/v1.2.3",
				"service2/v1.2.3",
				"cmd/tool/v1.0.0", // This should be excluded
			},
			modules: []string{".", "service1", "service2", "cmd/tool"},
			command: "minor",
			expectedTags: []string{
				"v1.3.0-prerelease01",
				"service1/v1.3.0-prerelease01",
				"service2/v1.3.0-prerelease01",
			},
			excludedModules: []string{"cmd/tool"},
		},
		{
			name: "multi-module with version conflicts",
			currentTags: []string{
				"v1.2.3",
				"service1/v1.3.0", // Already has target version
				"service2/v1.2.3",
			},
			modules: []string{".", "service1", "service2"},
			command: "manual",
			expectedTags: []string{
				"v1.3.0",          // Root module
				"service2/v1.3.0", // Service2 missing
				// service1/v1.3.0 should be skipped
			},
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
				ExcludedDirs:      tt.excludedModules,
				RequiredBranch:    "master",
				Verbose:           false,
				Command:           tt.command,
				SkipConfirmations: true,
			}

			if tt.command == "manual" {
				config.ManualVersion = "v1.3.0"
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.currentTags, nil)
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return(tt.modules, nil)

			// Expect tag creation only for expected tags
			for _, tag := range tt.expectedTags {
				mockGit.EXPECT().CreateTag(gomock.Any(), tag).Return(nil)
				mockGit.EXPECT().PushTag(gomock.Any(), tag).Return(nil)
			}

			// Execute command
			var err error
			if tt.command == "manual" {
				err = manager.RunRelease(context.Background())
			} else {
				err = manager.RunMinor(context.Background())
			}

			require.NoError(t, err)
		})
	}
}

func TestErrorRecoveryScenarios(t *testing.T) {
	tests := []struct {
		name          string
		description   string
		setupMocks    func(*MockGit, *MockFS, *MockUserInteraction)
		command       string
		expectedErr   string
		shouldRecover bool
	}{
		{
			name:        "partial tag creation failure",
			description: "Some tags created successfully, others failed",
			setupMocks: func(mockGit *MockGit, mockFS *MockFS, mockInteraction *MockUserInteraction) {
				mockGit.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.2.3"}, nil)
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{".", "service1"}, nil)

				// First tag succeeds, second fails
				mockGit.EXPECT().CreateTag(gomock.Any(), "v1.3.0-prerelease01").Return(nil)
				mockGit.EXPECT().CreateTag(gomock.Any(), "service1/v1.3.0-prerelease01").
					Return(assert.AnError)
			},
			command:     "minor",
			expectedErr: "failed to create tag",
		},
		{
			name:        "network failure during push",
			description: "Tags created locally but push failed",
			setupMocks: func(mockGit *MockGit, mockFS *MockFS, mockInteraction *MockUserInteraction) {
				mockGit.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.2.3"}, nil)
				mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
				mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)

				// Tag creation succeeds
				mockGit.EXPECT().CreateTag(gomock.Any(), "v1.3.0-prerelease01").Return(nil)
				// Push fails
				mockGit.EXPECT().PushTag(gomock.Any(), "v1.3.0-prerelease01").
					Return(assert.AnError)
			},
			command:     "minor",
			expectedErr: "push tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing scenario: %s", tt.description)

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
				Command:           tt.command,
				SkipConfirmations: true,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			tt.setupMocks(mockGit, mockFS, mockInteraction)

			var err error
			switch tt.command {
			case "minor":
				err = manager.RunMinor(context.Background())
			case "release":
				err = manager.RunRelease(context.Background())
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestConcurrentPrereleaseScenarios(t *testing.T) {
	tests := []struct {
		name            string
		existingTags    []string
		expectedVersion string
		description     string
	}{
		{
			name: "gap in prerelease sequence",
			existingTags: []string{
				"v1.3.0-prerelease01",
				"v1.3.0-prerelease03", // Missing 02
				"v1.3.0-prerelease05",
			},
			expectedVersion: "v1.3.0-prerelease06", // Should continue from highest
			description:     "Should continue from highest prerelease number even with gaps",
		},
		{
			name: "mixed prerelease versions",
			existingTags: []string{
				"v1.2.0-prerelease05",
				"v1.3.0-prerelease01",
				"v1.3.0-prerelease02",
				"v1.4.0-prerelease01",
			},
			expectedVersion: "v1.4.0-prerelease02", // Should work with current global version
			description:     "Should handle mixed prerelease versions for different base versions",
		},
		{
			name: "approaching prerelease limit",
			existingTags: []string{
				"v1.3.0-prerelease98",
			},
			expectedVersion: "v1.3.0-prerelease99",
			description:     "Should handle high prerelease numbers approaching limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Testing scenario: %s", tt.description)

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
			mockGit.EXPECT().GetTags(gomock.Any()).Return(tt.existingTags, nil)
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return("master", nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)
			mockGit.EXPECT().CreateTag(gomock.Any(), tt.expectedVersion).Return(nil)
			mockGit.EXPECT().PushTag(gomock.Any(), tt.expectedVersion).Return(nil)

			err := manager.RunPrerelease(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestUserInteractionScenarios(t *testing.T) {
	tests := []struct {
		name              string
		currentBranch     string
		userResponses     []bool // Responses to confirmation prompts
		skipConfirmations bool
		shouldComplete    bool
		expectedActions   int // Number of git operations expected
	}{
		{
			name:              "user confirms all warnings and actions",
			currentBranch:     "feature-branch",         // Wrong branch
			userResponses:     []bool{true, true, true}, // Continue despite warning, create tags, push tags
			skipConfirmations: false,
			shouldComplete:    true,
			expectedActions:   2, // create + push
		},
		{
			name:              "user rejects branch warning",
			currentBranch:     "feature-branch",
			userResponses:     []bool{false}, // Reject continuing despite warning
			skipConfirmations: false,
			shouldComplete:    false,
			expectedActions:   0,
		},
		{
			name:              "user confirms warning but rejects tag creation",
			currentBranch:     "feature-branch",
			userResponses:     []bool{true, false}, // Continue despite warning, but reject tag creation
			skipConfirmations: false,
			shouldComplete:    false,
			expectedActions:   0,
		},
		{
			name:              "user creates tags but skips push",
			currentBranch:     "feature-branch",
			userResponses:     []bool{true, true, false}, // Continue, create, but don't push
			skipConfirmations: false,
			shouldComplete:    false, // Partial completion
			expectedActions:   1,     // Only create
		},
		{
			name:              "skip all confirmations",
			currentBranch:     "feature-branch",
			userResponses:     []bool{}, // No interactions expected
			skipConfirmations: true,
			shouldComplete:    true,
			expectedActions:   2, // create + push
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
				SkipConfirmations: tt.skipConfirmations,
			}

			manager := NewReleaseManager(config, mockGit, mockFS, mockInteraction)

			// Setup basic mocks
			mockGit.EXPECT().GetTags(gomock.Any()).Return([]string{"v1.2.3"}, nil)
			mockGit.EXPECT().GetCurrentBranch(gomock.Any()).Return(tt.currentBranch, nil)
			mockFS.EXPECT().FindGoModFiles(gomock.Any(), "/test").Return([]string{"."}, nil)

			// Setup user interaction mocks
			responseIndex := 0
			if !tt.skipConfirmations {
				if tt.currentBranch != "master" {
					// Branch warning confirmation
					if responseIndex < len(tt.userResponses) {
						mockInteraction.EXPECT().
							ConfirmWithDefault(gomock.Any(), "Continue?", false).
							Return(tt.userResponses[responseIndex], nil)
						responseIndex++

						if !tt.userResponses[responseIndex-1] {
							// User rejected, no more interactions
							goto executeTest
						}
					}
				}

				// Tag creation confirmation
				if responseIndex < len(tt.userResponses) {
					mockInteraction.EXPECT().
						Confirm(gomock.Any(), gomock.Any()).
						Return(tt.userResponses[responseIndex], nil)
					responseIndex++

					if !tt.userResponses[responseIndex-1] {
						// User rejected tag creation
						goto executeTest
					}
				}

				// Tag push confirmation
				if responseIndex < len(tt.userResponses) {
					mockInteraction.EXPECT().
						Confirm(gomock.Any(), gomock.Any()).
						Return(tt.userResponses[responseIndex], nil)
				}
			}

			// Setup git operation mocks based on expected actions
			if tt.expectedActions >= 1 {
				mockGit.EXPECT().CreateTag(gomock.Any(), "v1.3.0-prerelease01").Return(nil)
			}
			if tt.expectedActions >= 2 {
				mockGit.EXPECT().PushTag(gomock.Any(), "v1.3.0-prerelease01").Return(nil)
			}

		executeTest:
			err := manager.RunMinor(context.Background())

			if tt.shouldComplete {
				require.NoError(t, err)
			} else {
				// May have error or be cancelled
				if err != nil {
					t.Logf("Expected cancellation/error: %v", err)
				}
			}
		})
	}
}
