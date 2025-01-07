// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/persistence"
)

func TestNewDBLoadCloser(t *testing.T) {
	tests := []struct {
		name          string
		contextSetup  func(td *cliTestData) *cli.Context
		mockSetup     func(td *cliTestData)
		expectedError string
	}{
		{
			name: "Success",
			contextSetup: func(td *cliTestData) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.Int(FlagShardID, 1, "shard ID flag")
				require.NoError(t, set.Set(FlagShardID, "1"))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				mockExecManager := persistence.NewMockExecutionManager(gomock.NewController(t))
				td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), 1).Return(mockExecManager, nil).Times(1)
			},
			expectedError: "",
		},
		{
			name: "Missing ShardID Error",
			contextSetup: func(td *cliTestData) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				return cli.NewContext(td.app, set, nil)
			},
			mockSetup:     func(td *cliTestData) {},
			expectedError: "error in NewDBLoadCloser: failed to get shard ID",
		},
		{
			name: "Execution Manager Initialization Error",
			contextSetup: func(td *cliTestData) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.Int(FlagShardID, 1, "shard ID flag")
				require.NoError(t, set.Set(FlagShardID, "1"))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), 1).Return(nil, fmt.Errorf("failed to initialize execution store")).Times(1)
			},
			expectedError: "error in NewDBLoadCloser: failed to initialize execution store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			c := tt.contextSetup(td)
			tt.mockSetup(td)

			_, err := NewDBLoadCloser(c)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewFileLoadCloser(t *testing.T) {
	tests := []struct {
		name          string
		contextSetup  func(td *cliTestData) (*cli.Context, string) // Return context and temp file path
		expectedError string
	}{
		{
			name: "Success",
			// Setup context with a temporary file to simulate successful file opening
			contextSetup: func(td *cliTestData) (*cli.Context, string) {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagInputFile, "", "Input file flag")

				// Create a temporary file for testing
				tempFile, err := os.CreateTemp("", "testfile_*.txt")
				require.NoError(t, err)

				// Close the file but do not remove it yet, so it remains available during the test
				require.NoError(t, tempFile.Close())

				// Set FlagInputFile to the path of the temporary file
				require.NoError(t, set.Set(FlagInputFile, tempFile.Name()))

				return cli.NewContext(td.app, set, nil), tempFile.Name() // Return context and file path
			},
			expectedError: "",
		},
		{
			name: "File Open Error",
			// Setup context with a non-existent file path to simulate file open failure
			contextSetup: func(td *cliTestData) (*cli.Context, string) {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagInputFile, "non_existent_file.txt", "input file flag")
				require.NoError(t, set.Set(FlagInputFile, "non_existent_file.txt"))

				return cli.NewContext(td.app, set, nil), "" // No file to delete after test
			},
			expectedError: "error in NewFileLoadCloser: cannot open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize test data
			td := newCLITestData(t)
			// Setup context and get temp file path
			c, tempFilePath := tt.contextSetup(td)

			// Defer file deletion after the test completes
			if tempFilePath != "" {
				defer os.Remove(tempFilePath) // Clean up the temporary file
			}

			// Call NewFileLoadCloser and capture result
			closer, err := NewFileLoadCloser(c)

			if tt.expectedError != "" {
				// Verify error message when expected
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, closer)
			} else {
				// Verify no error and a non-nil LoadCloser instance on success
				require.NoError(t, err)
				assert.NotNil(t, closer)
				closer.Close() // Close the file to prevent resource leakage
			}
		})
	}
}

func TestNewReporter(t *testing.T) {
	// Mock dependencies
	mockLoader := &MockLoadCloser{} // Substitute with actual mock creation
	mockPrinter := &MockPrinter{}   // Substitute with actual mock creation

	// Define test inputs
	domain := "testDomain"
	timerTypes := []int{1, 2, 3}

	// Call the function under test
	reporter := NewReporter(domain, timerTypes, mockLoader, mockPrinter)

	// Validate the output
	require.NotNil(t, reporter)
	assert.Equal(t, domain, reporter.domainID)
	assert.Equal(t, timerTypes, reporter.timerTypes)
	assert.Equal(t, mockLoader, reporter.loader)
	assert.Equal(t, mockPrinter, reporter.printer)
}

func TestNewHistogramPrinter(t *testing.T) {
	// Mock CLI context
	td := newCLITestData(t) // Setup test data for CLI context
	c := cli.NewContext(td.app, flag.NewFlagSet("test", 0), nil)

	// Define test inputs
	timeFormat := "2006-01-02 15:04:05"

	// Call the function under test
	printer := NewHistogramPrinter(c, timeFormat)

	// Validate the output
	require.NotNil(t, printer)
	histPrinter, ok := printer.(*histogramPrinter)
	require.True(t, ok, "Expected Printer to be of type *histogramPrinter")
	assert.Equal(t, c, histPrinter.ctx)
	assert.Equal(t, timeFormat, histPrinter.timeFormat)
}

func TestNewJSONPrinter(t *testing.T) {
	// Mock CLI context
	td := newCLITestData(t) // Setup test data for CLI context
	c := cli.NewContext(td.app, flag.NewFlagSet("test", 0), nil)

	// Call the function under test
	printer := NewJSONPrinter(c)

	// Validate the output
	require.NotNil(t, printer)
	jsonPrinter, ok := printer.(*jsonPrinter)
	require.True(t, ok, "Expected Printer to be of type *jsonPrinter")
	assert.Equal(t, c, jsonPrinter.ctx)
}

func TestReporter_Report(t *testing.T) {
	tests := []struct {
		name           string
		domainID       string
		timerTypes     []int
		loaderOutput   []*persistence.TimerTaskInfo
		loaderError    error
		expectedOutput []*persistence.TimerTaskInfo
		expectedError  string
		mockSetup      func(mockLoader *MockLoadCloser, mockPrinter *MockPrinter)
	}{
		{
			name:       "Successful Report with Filtering",
			domainID:   "testDomain",
			timerTypes: []int{1, 2},
			loaderOutput: []*persistence.TimerTaskInfo{
				{TaskType: 1, DomainID: "testDomain"},
				{TaskType: 3, DomainID: "testDomain"},
				{TaskType: 2, DomainID: "otherDomain"},
			},
			expectedOutput: []*persistence.TimerTaskInfo{
				{TaskType: 1, DomainID: "testDomain"},
				nil, // TaskType 3 filtered out
				nil, // DomainID otherDomain filtered out
			},
			expectedError: "",
			mockSetup: func(mockLoader *MockLoadCloser, mockPrinter *MockPrinter) {
				mockLoader.EXPECT().Load().Return([]*persistence.TimerTaskInfo{
					{TaskType: 1, DomainID: "testDomain"},
					{TaskType: 3, DomainID: "testDomain"},
					{TaskType: 2, DomainID: "otherDomain"},
				}, nil).Times(1)
				mockPrinter.EXPECT().Print(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
		},
		{
			name:           "Loader Error",
			domainID:       "testDomain",
			timerTypes:     []int{1},
			loaderOutput:   nil,
			loaderError:    fmt.Errorf("failed to load tasks"),
			expectedOutput: nil,
			expectedError:  "failed to load tasks",
			mockSetup: func(mockLoader *MockLoadCloser, mockPrinter *MockPrinter) {
				mockLoader.EXPECT().Load().Return(nil, fmt.Errorf("failed to load tasks")).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			// Initialize mocks
			mockLoader := NewMockLoadCloser(ctrl)
			mockPrinter := NewMockPrinter(ctrl)

			// Set up mock expectations
			tt.mockSetup(mockLoader, mockPrinter)

			// Create reporter instance
			reporter := NewReporter(tt.domainID, tt.timerTypes, mockLoader, mockPrinter)

			// Call Report and capture error
			err := reporter.Report(nil)

			// Check results based on expected error
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAdminTimers(t *testing.T) {
	tests := []struct {
		name           string
		contextSetup   func(td *cliTestData) *cli.Context
		mockSetup      func(td *cliTestData)
		expectedError  string
		expectedOutput string
	}{
		{
			name: "Default Timer Types with DB Loader and Histogram Printer",
			contextSetup: func(td *cliTestData) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.Var(cli.NewIntSlice(-1), FlagTimerType, "timer type flag") // Default timer types
				set.Bool(FlagPrintJSON, false, "print JSON flag")              // Use histogram printer
				set.String(FlagBucketSize, "", "bucket size flag")
				set.Int(FlagShardID, 1, "shard ID flag")                 // Set shard ID for DB loader
				set.Int(FlagShardMultiplier, 1, "shard multiplier flag") // Set a non-zero multiplier
				set.String(FlagEndDate, "", "end date flag")             // Explicitly set end date format
				set.String(FlagStartDate, "", "start date flag")         // Explicitly set start date format

				require.NoError(t, set.Set(FlagTimerType, "-1"))
				require.NoError(t, set.Set(FlagPrintJSON, "false"))
				require.NoError(t, set.Set(FlagBucketSize, "hour"))
				require.NoError(t, set.Set(FlagShardID, "1"))
				require.NoError(t, set.Set(FlagShardMultiplier, "1"))
				require.NoError(t, set.Set(FlagEndDate, "2023-01-01T00:00:00Z"))
				require.NoError(t, set.Set(FlagStartDate, "2022-01-01T00:00:00Z"))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				ctrl := gomock.NewController(t)
				mockExecManager := persistence.NewMockExecutionManager(ctrl)

				td.mockManagerFactory.EXPECT().
					initializeExecutionManager(gomock.Any(), gomock.Any()).
					Return(mockExecManager, nil).Times(1)

				mockExecManager.EXPECT().
					GetTimerIndexTasks(gomock.Any(), gomock.Any()).
					Return(&persistence.GetTimerIndexTasksResponse{
						Timers: []*persistence.TimerTaskInfo{
							{
								DomainID:            "testDomain",
								WorkflowID:          "testWorkflow1",
								RunID:               "testRun1",
								VisibilityTimestamp: time.Now(),
								TaskID:              1,
								TaskType:            persistence.TaskTypeUserTimer,
							},
							{
								DomainID:            "testDomain",
								WorkflowID:          "testWorkflow2",
								RunID:               "testRun2",
								VisibilityTimestamp: time.Now().Add(time.Hour),
								TaskID:              2,
								TaskType:            persistence.TaskTypeUserTimer,
							},
						},
						NextPageToken: nil,
					}, nil).Times(1)

				mockExecManager.EXPECT().Close().Times(1)
			},
			expectedError:  "",
			expectedOutput: "Bucket        Count\n", // Customize this with expected histogram output
		},
		{
			name: "File Loader with JSON Printer",
			contextSetup: func(td *cliTestData) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.Var(cli.NewIntSlice(persistence.TaskTypeUserTimer), FlagTimerType, "timer type flag")
				set.Bool(FlagPrintJSON, true, "print JSON flag") // Use JSON printer
				set.String(FlagInputFile, "", "input file flag") // Set input file for file loader

				// Create a temporary file and write test timer data to it
				tempFile, err := os.CreateTemp("", "testfile_*.txt")
				require.NoError(t, err)

				// Write a sample TimerTaskInfo JSON object to the file
				timerTask := persistence.TimerTaskInfo{
					DomainID:            "testDomain",
					WorkflowID:          "testWorkflow1",
					RunID:               "testRun1",
					VisibilityTimestamp: time.Now(),
					TaskID:              1,
					TaskType:            persistence.TaskTypeUserTimer,
				}
				timerData, err := json.Marshal(timerTask)
				require.NoError(t, err)
				_, err = tempFile.Write(timerData)
				require.NoError(t, err)
				tempFile.Close() // Close after writing to ensure data is saved

				// Set flag values
				require.NoError(t, set.Set(FlagTimerType, fmt.Sprint(persistence.TaskTypeUserTimer)))
				require.NoError(t, set.Set(FlagPrintJSON, "true"))
				require.NoError(t, set.Set(FlagInputFile, tempFile.Name()))

				// Ensure file is removed after the test
				t.Cleanup(func() {
					os.Remove(tempFile.Name())
				})

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup:      func(td *cliTestData) {},
			expectedError:  "",
			expectedOutput: `{"DomainID":"testDomain","WorkflowID":"testWorkflow1","RunID":"testRun1"`, // Partial match to validate JSON structure
		},
		{
			name: "Error with Unknown Bucket Size",
			contextSetup: func(td *cliTestData) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagBucketSize, "", "bucket size flag")
				set.Int(FlagShardID, 1, "shard ID flag")

				require.NoError(t, set.Set(FlagBucketSize, "unknown"))
				require.NoError(t, set.Set(FlagShardID, "1"))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				ctrl := gomock.NewController(t)
				mockExecManager := persistence.NewMockExecutionManager(ctrl)
				mockExecManager.EXPECT().Close().Times(1)

				td.mockManagerFactory.EXPECT().
					initializeExecutionManager(gomock.Any(), gomock.Any()).
					Return(mockExecManager, nil).Times(1)
			},
			expectedError: "unknown bucket size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			c := tt.contextSetup(td)

			tt.mockSetup(td)

			err := AdminTimers(c)

			// Validate expected error
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				// Validate the output using testIOHandler's outputBytes
				output := td.ioHandler.outputBytes.String()
				assert.Contains(t, output, tt.expectedOutput)
			}
		})
	}
}
