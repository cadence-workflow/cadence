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
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"

	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/reconciliation/invariant"
)

func TestAdminDBClean_noFixExecution(t *testing.T) {
	tests := []struct {
		name           string
		setupContext   func(app *cli.App) *cli.Context
		inputFileData  string // Simulate the content of the input file
		expectedOutput string
		expectedError  string
	}{
		{
			name: "SuccessCase_emptyResult",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				// Define flags
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")
				// Use a StringSliceFlag to simulate multiple collection values
				set.Var(cli.NewStringSlice("CollectionHistory", "CollectionDomain"), FlagInvariantCollection, "invariant collection flag")

				// Set actual values for the flags
				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, "input.json"))

				return cli.NewContext(app, set, nil)
			},
			inputFileData:  `[{"Execution": {"ShardID": 1}}]`, // Simulate the content of input file
			expectedOutput: ``,
			expectedError:  "",
		},
		{
			name: "MissingRequiredFlagScanType",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagInputFile, "", "Input file flag")
				// Missing FlagScanType
				_ = set.Set(FlagInputFile, "input.json")
				return cli.NewContext(app, set, nil)
			},
			inputFileData:  ``,
			expectedOutput: "",
			expectedError:  "Required flag not found:",
		},
		{
			name: "UnknownScanType",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")
				require.NoError(t, set.Set(FlagScanType, "unknown"))
				require.NoError(t, set.Set(FlagInputFile, "input.json"))
				return cli.NewContext(app, set, nil)
			},
			inputFileData:  ``,
			expectedOutput: "",
			expectedError:  "unknown scan type",
		},
		{
			name: "InvalidInvariantCollection",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				// Define FlagScanType and FlagInputFile
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")

				// Simulate the collection slice with multiple collections (including an invalid one)
				set.Var(cli.NewStringSlice("invalid_collection", "history"), FlagInvariantCollection, "invariant collection flag")

				// Set actual values for the flags
				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, "input.json"))

				return cli.NewContext(app, set, nil)
			},
			inputFileData:  ``,
			expectedOutput: "",
			expectedError:  "unknown invariant collection",
		},
		{
			name: "NoInvariantsError",
			setupContext: func(app *cli.App) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInvariantCollection, "", "invariant collection flag")
				set.String(FlagInputFile, "", "Input file flag")

				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, "input.json"))
				require.NoError(t, set.Set(FlagInvariantCollection, "invalid_collection"))

				return cli.NewContext(app, set, nil)
			},
			inputFileData:  `[{"Execution": {"ShardID": 1}}]`,
			expectedOutput: "",
			expectedError:  "no invariants for scantype",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp input file with the test's input data
			inputFile, err := os.CreateTemp("", "test_input_*.json")
			assert.NoError(t, err)
			defer os.Remove(inputFile.Name()) // Clean up after test

			// Write input data to the temp file
			if tt.inputFileData != "" {
				_, err = inputFile.WriteString(tt.inputFileData)
				assert.NoError(t, err)
			}
			inputFile.Close()

			// Create test IO handler to capture output
			ioHandler := &testIOHandler{}

			// Set up the CLI app
			app := NewCliApp(nil, WithIOHandler(ioHandler))

			// Set up the CLI context
			c := tt.setupContext(app)

			// Overwrite the FlagInputFile with the actual temp file path
			_ = c.Set(FlagInputFile, inputFile.Name())

			// Call AdminDBClean and validate output and errors
			err = AdminDBClean(c)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Validate the captured output
			assert.Contains(t, ioHandler.outputBytes.String(), tt.expectedOutput)
		})
	}
}

func TestAdminDBClean_inFixExecution(t *testing.T) {
	tests := []struct {
		name           string
		contextSetup   func(td *cliTestData, inputFilePath string) *cli.Context
		mockSetup      func(td *cliTestData)
		inputFileData  string
		expectedError  string
		expectedOutput string
	}{
		{
			name: "Success",
			contextSetup: func(td *cliTestData, inputFilePath string) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")
				set.Var(cli.NewStringSlice("CollectionHistory", "CollectionDomain"), FlagInvariantCollection, "invariant collection flag")

				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, inputFilePath))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				ctrl := gomock.NewController(t)
				mockExecManager := persistence.NewMockExecutionManager(ctrl)
				mockHistoryManager := persistence.NewMockHistoryManager(ctrl)
				mockInvariantManager := invariant.NewMockManager(ctrl)

				mockExecManager.EXPECT().Close().Times(1)
				mockHistoryManager.EXPECT().Close().Times(1)

				mockInvariantManager.EXPECT().RunFixes(gomock.Any(), gomock.Any()).Return(invariant.ManagerFixResult{}).Times(1)

				td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), gomock.Any()).Return(mockExecManager, nil).Times(1)
				td.mockManagerFactory.EXPECT().initializeHistoryManager(gomock.Any()).Return(mockHistoryManager, nil).Times(1)
				td.mockManagerFactory.EXPECT().initializeInvariantManager(gomock.Any()).Return(mockInvariantManager, nil).Times(1)

			},
			inputFileData: `{"Execution": {"ShardID": 1}, "Result": {}}`,
			expectedOutput: `{"Execution":{"BranchToken":null,"TreeID":"","BranchID":"","ShardID":1,"DomainID":"","WorkflowID":"","RunID":"","State":0},"Input":{"Execution":{"BranchToken":null,"TreeID":"","BranchID":"","ShardID":1,"DomainID":"","WorkflowID":"","RunID":"","State":0},"Result":{"CheckResultType":"","DeterminingInvariantType":null,"CheckResults":null}},"Result":{"FixResultType":"","DeterminingInvariantName":null,"FixResults":null}}
`,
			expectedError: "",
		},
		{
			name: "init invariant manager error",
			contextSetup: func(td *cliTestData, inputFilePath string) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")
				set.Var(cli.NewStringSlice("CollectionHistory", "CollectionDomain"), FlagInvariantCollection, "invariant collection flag")

				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, inputFilePath))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				ctrl := gomock.NewController(t)
				mockExecManager := persistence.NewMockExecutionManager(ctrl)
				mockHistoryManager := persistence.NewMockHistoryManager(ctrl)

				mockExecManager.EXPECT().Close().Times(1)
				mockHistoryManager.EXPECT().Close().Times(1)

				td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), gomock.Any()).Return(mockExecManager, nil).Times(1)
				td.mockManagerFactory.EXPECT().initializeHistoryManager(gomock.Any()).Return(mockHistoryManager, nil).Times(1)
				td.mockManagerFactory.EXPECT().initializeInvariantManager(gomock.Any()).Return(nil, fmt.Errorf("init invariant manager error")).Times(1)

			},
			inputFileData:  `{"Execution": {"ShardID": 1}, "Result": {}}`,
			expectedOutput: ``,
			expectedError:  "Error in fix execution: : init invariant manager error",
		},
		{
			name: "init execution manager error",
			contextSetup: func(td *cliTestData, inputFilePath string) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")
				set.Var(cli.NewStringSlice("CollectionHistory", "CollectionDomain"), FlagInvariantCollection, "invariant collection flag")

				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, inputFilePath))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("init execution manager error")).Times(1)
			},
			inputFileData:  `{"Execution": {"ShardID": 1}, "Result": {}}`,
			expectedOutput: ``,
			expectedError:  "Error in fix execution: : init execution manager error",
		},
		{
			name: "init history manager error",
			contextSetup: func(td *cliTestData, inputFilePath string) *cli.Context {
				set := flag.NewFlagSet("test", 0)
				set.String(FlagScanType, "", "scan type flag")
				set.String(FlagInputFile, "", "Input file flag")
				set.Var(cli.NewStringSlice("CollectionHistory", "CollectionDomain"), FlagInvariantCollection, "invariant collection flag")

				require.NoError(t, set.Set(FlagScanType, "ConcreteExecutionType"))
				require.NoError(t, set.Set(FlagInputFile, inputFilePath))

				return cli.NewContext(td.app, set, nil)
			},
			mockSetup: func(td *cliTestData) {
				ctrl := gomock.NewController(t)
				mockExecManager := persistence.NewMockExecutionManager(ctrl)
				mockExecManager.EXPECT().Close().Times(1)

				td.mockManagerFactory.EXPECT().initializeExecutionManager(gomock.Any(), gomock.Any()).Return(mockExecManager, nil).Times(1)
				td.mockManagerFactory.EXPECT().initializeHistoryManager(gomock.Any()).Return(nil, fmt.Errorf("init history manager error")).Times(1)
			},
			inputFileData:  `{"Execution": {"ShardID": 1}, "Result": {}}`,
			expectedOutput: ``,
			expectedError:  "Error in fix execution: : init history manager error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td := newCLITestData(t)
			tt.mockSetup(td)

			inputFile, err := os.CreateTemp("", "test_input_*.json")
			assert.NoError(t, err)
			defer os.Remove(inputFile.Name())

			if tt.inputFileData != "" {
				_, err = inputFile.WriteString(tt.inputFileData)
				assert.NoError(t, err)
			}
			inputFile.Close()

			c := tt.contextSetup(td, inputFile.Name())

			err = AdminDBClean(c)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedOutput, td.consoleOutput())
		})
	}
}
