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

package cache

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"

	"github.com/uber/cadence/tools/fastgogenerate/plugins"
)

// ID represents a unique identifier computed based on plugins.ComputeInfo.
type ID string

// IDComputer computes a unique ID and uses sha256 to hash
// the content of the input files, output file names, and extra arguments.
type IDComputer interface {
	// Compute computes a cache ID based on the provided info.
	// It calculates a hash based on the content of the input files, extra arguments, and outbound files.
	Compute(info plugins.ComputeInfo) (id ID, err error)
}

type idComputer struct {
	logger *zap.Logger
}

// NewIDComputer creates a new IDComputer instance.
func NewIDComputer(logger *zap.Logger) IDComputer {
	return &idComputer{
		logger: logger,
	}
}

// Compute computes a cache ID based on the provided info.
func (c *idComputer) Compute(info plugins.ComputeInfo) (id ID, err error) {
	var hashInput = info.PluginName

	// Calculate the hash based on the content of input files
	for _, fileName := range info.InputFileNames {
		hash, err := c.calculateHashFromFile(fileName)
		if err != nil {
			return "", fmt.Errorf("failed to calculate hash from file %s: %w", fileName, err)
		}
		hashInput += hash
	}

	// Calculate the hash based on the output file names
	for _, fileName := range info.OutputFileNames {
		hash, err := c.calculateHashFromString(fileName)
		if err != nil {
			return "", fmt.Errorf("failed to calculate hash from file %s: %w", fileName, err)
		}
		hashInput += hash

	}

	// Calculate the hash based on the extra arguments
	for _, arg := range info.Args {
		hash, err := c.calculateHashFromString(arg)
		if err != nil {
			return "", fmt.Errorf("failed to calculate hash from arg %s: %w", arg, err)
		}
		hashInput += hash

	}

	// Compute the final hash
	hash, err := c.calculateHashFromString(hashInput)
	if err != nil {
		return "", fmt.Errorf("failed to calculate final hash: %w", err)
	}

	id = ID(fmt.Sprintf("%x", hash))
	c.logger.Debug("Computed cache id", zap.String("cache id", string(id)))
	return id, nil
}

func (c *idComputer) calculateHashFromFile(fileName string) (string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return string(h.Sum(nil)), nil
}

func (c *idComputer) calculateHashFromString(str string) (string, error) {
	h := sha256.New()
	if _, err := h.Write([]byte(str)); err != nil {
		return "", fmt.Errorf("failed to write string to hash: %w", err)
	}

	return string(h.Sum(nil)), nil
}
