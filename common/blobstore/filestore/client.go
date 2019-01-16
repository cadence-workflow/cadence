// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package filestore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/uber/cadence/common/blobstore"
)

const (
	metadataFilename = "metadata"
)

type client struct {
	sync.Mutex
	storeDirectory string
}

// NewClient returns a new Client backed by file system
func NewClient(cfg *Config) (blobstore.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := setupDirectories(cfg); err != nil {
		return nil, err
	}
	if err := writeMetadataFiles(cfg); err != nil {
		return nil, err
	}
	return &client{
		storeDirectory: cfg.StoreDirectory,
	}, nil
}

func (c *client) Upload(_ context.Context, bucket string, key string, blob blobstore.Blob) error {
	c.Lock()
	defer c.Unlock()

	exists, err := directoryExists(bucketDirectory(c.storeDirectory, bucket))
	if err != nil {
		return err
	}
	if !exists {
		return blobstore.ErrBucketNotExists
	}

	cBlob, err := blob.Compress()
	if err != nil {
		return err
	}
	fileBytes, err := serializeBlob(cBlob)
	if err != nil {
		return err
	}
	blobPath := bucketItemPath(c.storeDirectory, bucket, key)
	return writeFile(blobPath, fileBytes)
}

func (c *client) Download(_ context.Context, bucket string, key string) (blobstore.Blob, error) {
	c.Lock()
	defer c.Unlock()

	exists, err := directoryExists(bucketDirectory(c.storeDirectory, bucket))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, blobstore.ErrBucketNotExists
	}
	blobPath := bucketItemPath(c.storeDirectory, bucket, key)
	fileBytes, err := readFile(blobPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, blobstore.ErrBlobNotExists
		}
		return nil, err
	}
	cBlob, err := deserializeBlob(fileBytes)
	if err != nil {
		return nil, err
	}
	return cBlob.Decompress()
}

func (c *client) Exists(_ context.Context, bucket string, key string) (bool, error) {
	c.Lock()
	defer c.Unlock()

	exists, err := directoryExists(bucketDirectory(c.storeDirectory, bucket))
	if err != nil {
		return false, err
	}
	if !exists {
		return false, blobstore.ErrBucketNotExists
	}
	blobPath := bucketItemPath(c.storeDirectory, bucket, key)
	return fileExists(blobPath)
}

func (c *client) Delete(_ context.Context, bucket string, key string) (bool, error) {
	c.Lock()
	defer c.Unlock()

	exists, err := directoryExists(bucketDirectory(c.storeDirectory, bucket))
	if err != nil {
		return false, err
	}
	if !exists {
		return false, blobstore.ErrBucketNotExists
	}
	blobPath := bucketItemPath(c.storeDirectory, bucket, key)
	return deleteFile(blobPath)
}

func (c *client) ListByPrefix(_ context.Context, bucket string, prefix string) ([]string, error) {
	c.Lock()
	defer c.Unlock()

	bucketDir := bucketDirectory(c.storeDirectory, bucket)
	exists, err := directoryExists(bucketDir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, blobstore.ErrBucketNotExists
	}
	files, err := listFiles(bucketDir)
	if err != nil {
		return nil, err
	}
	var filesWithPrefix []string
	for _, f := range files {
		if strings.HasPrefix(f, prefix) {
			filesWithPrefix = append(filesWithPrefix, f)
		}
	}
	return filesWithPrefix, nil
}

func (c *client) BucketMetadata(_ context.Context, bucket string) (*blobstore.BucketMetadataResponse, error) {
	c.Lock()
	defer c.Unlock()

	exists, err := directoryExists(bucketDirectory(c.storeDirectory, bucket))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, blobstore.ErrBucketNotExists
	}

	metadataFilepath := bucketItemPath(c.storeDirectory, bucket, metadataFilename)
	exists, err = fileExists(metadataFilepath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("failed to get metadata, file at %v does not exist", metadataFilepath)
	}

	data, err := readFile(metadataFilepath)
	if err != nil {
		return nil, err
	}
	bucketCfg, err := deserializeBucketConfig(data)
	if err != nil {
		return nil, err
	}

	return &blobstore.BucketMetadataResponse{
		Owner:         bucketCfg.Owner,
		RetentionDays: bucketCfg.RetentionDays,
	}, nil
}

func setupDirectories(cfg *Config) error {
	if err := mkdirAll(cfg.StoreDirectory); err != nil {
		return err
	}
	if err := mkdirAll(bucketDirectory(cfg.StoreDirectory, cfg.DefaultBucket.Name)); err != nil {
		return err
	}
	for _, b := range cfg.CustomBuckets {
		if err := mkdirAll(bucketDirectory(cfg.StoreDirectory, b.Name)); err != nil {
			return err
		}
	}
	return nil
}

func writeMetadataFiles(cfg *Config) error {
	writeMetadataFile := func(bucketConfig BucketConfig) error {
		path := bucketItemPath(cfg.StoreDirectory, bucketConfig.Name, metadataFilename)
		bytes, err := serializeBucketConfig(&bucketConfig)
		if err != nil {
			return fmt.Errorf("failed to write metadata file for bucket %v: %v", bucketConfig.Name, err)
		}
		if err := writeFile(path, bytes); err != nil {
			return fmt.Errorf("failed to write metadata file for bucket %v: %v", bucketConfig.Name, err)
		}
		return nil
	}

	if err := writeMetadataFile(cfg.DefaultBucket); err != nil {
		return err
	}
	for _, b := range cfg.CustomBuckets {
		if err := writeMetadataFile(b); err != nil {
			return err
		}
	}
	return nil
}

func bucketDirectory(storeDirectory string, bucketName string) string {
	return filepath.Join(storeDirectory, bucketName)
}

func bucketItemPath(storeDirectory string, bucketName string, filename string) string {
	return filepath.Join(bucketDirectory(storeDirectory, bucketName), filename)
}
