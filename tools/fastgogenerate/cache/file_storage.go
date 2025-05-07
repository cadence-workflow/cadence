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
	"fmt"
	"os"
	"path"

	"go.uber.org/zap"
)

// FileStorage interface for file-based cache storage
type FileStorage struct {
	rootDir string
	logger  *zap.Logger
}

// NewFileStorage creates a new FileStorage instance
func NewFileStorage(rootDir string, logger *zap.Logger) *FileStorage {
	return &FileStorage{
		rootDir: rootDir,
		logger:  logger,
	}
}

// IsExist return true if the compute info already present in the cache
func (c *FileStorage) IsExist(id ID) (isExists bool, err error) {
	c.logger.Debug("Check if cache exists", zap.String("filename", c.fileName(id)))

	// create a directory if it does not exist
	if err := os.MkdirAll(c.rootDir, os.ModePerm); err != nil {
		return false, fmt.Errorf("failed to create directory %s: %w", c.rootDir, err)
	}

	_, err = os.Stat(c.fileName(id))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Save saves that compute info in the cache
func (c *FileStorage) Save(id ID) error {
	c.logger.Debug("Save cache", zap.String("filename", c.fileName(id)))

	// create a directory if it does not exist
	if err := os.MkdirAll(c.rootDir, os.ModePerm); err != nil {
		return err
	}

	file, err := os.OpenFile(c.fileName(id), os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// fileName generates a file name based on the ComputeInfo
func (c *FileStorage) fileName(id ID) string {
	return path.Join(c.rootDir, string(id))
}
