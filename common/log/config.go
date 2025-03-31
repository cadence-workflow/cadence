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

package log

// Config is a configuration of log.Logger.
type Config struct {
	// Stdout is true then the output needs to goto standard out
	// By default this is false and output will go to standard error
	Stdout bool `yaml:"stdout"`
	// Level is the desired log level
	Level string `yaml:"level"`
	// OutputFile is the path to the log output file
	// Stdout must be false, otherwise Stdout will take precedence
	OutputFile string `yaml:"outputFile"`
	// LevelKey is the desired log level, defaults to "level"
	LevelKey string `yaml:"levelKey"`
	// Encoding decides the format, supports "console" and "json".
	// "json" will print the log in JSON format(better for machine), while "console" will print in plain-text format(more human friendly)
	// Default is "json"
	Encoding string `yaml:"encoding"`
}
