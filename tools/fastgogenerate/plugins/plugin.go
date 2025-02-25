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

package plugins

import "context"

// Plugin is an interface that defines the methods required for a plugin
type Plugin interface {
	// Name returns the name of the plugin
	// It is used to register the plugin and to identify it
	Name() string

	// ComputeInfo parses the arguments and computes the input and output file names
	// and any extra arguments needed for the plugin
	// It is used to cache the plugin execution results to avoid re-execution
	ComputeInfo(args []string) (ComputeInfo, error)

	// Execute executes the plugin with the given arguments
	// It is used to perform the actual work of the plugin
	Execute(ctx context.Context, args []string) error
}

// ComputeInfo holds the information needed to cache the plugin execution
// It includes the input file names, output file names, and any extra arguments
// Plugin parse the arguments via Plugin.ComputeInfo method and populate this struct
type ComputeInfo struct {
	// PluginName is the name of the plugin
	PluginName string

	// InputFileNames are the names of the input files for the plugin
	InputFileNames []string

	// OutputFileNames are the names of the output files for the plugin
	OutputFileNames []string

	// Args is a list of provided arguments needed for the plugin
	Args []string
}
