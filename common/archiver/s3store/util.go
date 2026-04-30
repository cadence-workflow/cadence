// Copyright (c) 2020 Uber Technologies, Inc.
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

package s3store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"go.uber.org/multierr"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/archiver"
	"github.com/uber/cadence/common/types"
)

// encoding & decoding util

func encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func decodeHistoryBlob(data []byte) (*archiver.HistoryBlob, error) {
	historyBlob := &archiver.HistoryBlob{}
	err := json.Unmarshal(data, historyBlob)
	if err != nil {
		return nil, err
	}
	return historyBlob, nil
}
func decodeVisibilityRecord(data []byte) (*visibilityRecord, error) {
	record := &visibilityRecord{}
	err := json.Unmarshal(data, record)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func serializeToken(token interface{}) ([]byte, error) {
	if token == nil {
		return nil, nil
	}
	return json.Marshal(token)
}

func deserializeGetHistoryToken(bytes []byte) (*getHistoryToken, error) {
	token := &getHistoryToken{}
	err := json.Unmarshal(bytes, token)
	return token, err
}

func deserializeQueryVisibilityToken(bytes []byte) *string {
	var ret = string(bytes)
	return &ret
}
func serializeQueryVisibilityToken(token string) []byte {
	return []byte(token)
}

// Only validates the scheme and buckets are passed
func softValidateURI(URI archiver.URI) error {
	switch URI.Scheme() {
	case URIScheme:
		if len(URI.Hostname()) == 0 {
			return errNoBucketSpecified
		}
		return nil
	case URISchemeAccessPoint:
		return validateAccessPointURI(URI)
	default:
		return archiver.ErrURISchemeMismatch
	}
}

// validateAccessPointURI checks that an s3-ap URI has a 12-digit account id
// and a non-empty access point name. The region is not validated here because
// it is supplied by the archiver's configuration at request time.
func validateAccessPointURI(URI archiver.URI) error {
	account := URI.Hostname()
	if len(account) != 12 {
		return errInvalidAccessPointAcct
	}
	for _, r := range account {
		if r < '0' || r > '9' {
			return errInvalidAccessPointAcct
		}
	}
	if _, _, ok := splitAccessPointPath(URI.Path()); !ok {
		return errInvalidAccessPointURI
	}
	return nil
}

// splitAccessPointPath splits the URL path of an s3-ap URI into the access
// point name and the remaining object-key prefix. Returns ok=false when no
// non-empty access point name is present.
func splitAccessPointPath(p string) (name, keyPath string, ok bool) {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return "", "", false
	}
	if idx := strings.Index(p, "/"); idx >= 0 {
		return p[:idx], p[idx:], p[:idx] != ""
	}
	return p, "", true
}

// s3Bucket returns the string to pass as the S3 SDK Bucket parameter.
// For s3:// URIs this is the hostname. For s3-ap:// URIs this is a full
// access point ARN constructed from the configured region.
// An empty region for an s3-ap URI returns errEmptyAwsRegion.
func s3Bucket(URI archiver.URI, region string) (string, error) {
	switch URI.Scheme() {
	case URISchemeAccessPoint:
		name, _, ok := splitAccessPointPath(URI.Path())
		if !ok {
			return "", errInvalidAccessPointURI
		}
		if region == "" {
			return "", errEmptyAwsRegion
		}
		return fmt.Sprintf("arn:%s:s3:%s:%s:accesspoint/%s", partitionForRegion(region), region, URI.Hostname(), name), nil
	default:
		return URI.Hostname(), nil
	}
}

// partitionForRegion returns the AWS ARN partition for the given region.
// AWS partitions an account's resources by jurisdiction; ARNs in different
// partitions use different prefixes. Defaults to the standard "aws" partition.
func partitionForRegion(region string) string {
	switch {
	case strings.HasPrefix(region, "cn-"):
		return "aws-cn"
	case strings.HasPrefix(region, "us-gov-"):
		return "aws-us-gov"
	case strings.HasPrefix(region, "us-iso-"):
		return "aws-iso"
	case strings.HasPrefix(region, "us-isob-"):
		return "aws-iso-b"
	default:
		return "aws"
	}
}

// s3KeyPath returns the path component used for object-key construction.
// For s3-ap URIs, the access point name is stripped from the URL path so the
// remainder matches what users would get from an s3:// URI.
func s3KeyPath(URI archiver.URI) string {
	if URI.Scheme() == URISchemeAccessPoint {
		_, keyPath, _ := splitAccessPointPath(URI.Path())
		return keyPath
	}
	return URI.Path()
}

func bucketExists(ctx context.Context, s3cli s3iface.S3API, URI archiver.URI, region string) error {
	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()
	bucket, err := s3Bucket(URI, region)
	if err != nil {
		return err
	}
	_, err = s3cli.HeadBucketWithContext(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	})
	if err == nil {
		return nil
	}
	if isNotFoundError(err) {
		return errBucketNotExists
	}
	return err
}

func keyExists(ctx context.Context, s3cli s3iface.S3API, URI archiver.URI, key, region string) (bool, error) {
	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()
	bucket, err := s3Bucket(URI, region)
	if err != nil {
		return false, err
	}
	_, err = s3cli.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isNotFoundError(err error) bool {
	aerr, ok := err.(awserr.Error)
	return ok && (aerr.Code() == "NotFound")
}

// Key construction
func constructHistoryKey(path, domainID, workflowID, runID string, version int64, batchIdx int) string {
	prefix := constructHistoryKeyPrefixWithVersion(path, domainID, workflowID, runID, version)
	return fmt.Sprintf("%s%d", prefix, batchIdx)
}

func constructHistoryKeyPrefixWithVersion(path, domainID, workflowID, runID string, version int64) string {
	prefix := constructHistoryKeyPrefix(path, domainID, workflowID, runID)
	return fmt.Sprintf("%s/%v/", prefix, version)
}

func constructHistoryKeyPrefix(path, domainID, workflowID, runID string) string {
	return strings.TrimLeft(strings.Join([]string{path, domainID, "history", workflowID, runID}, "/"), "/")
}

func constructTimeBasedSearchKey(path, domainID, primaryIndexKey, primaryIndexValue, secondaryIndexKey string, timestamp int64, precision string) string {
	t := time.Unix(0, timestamp).In(time.UTC)
	var timeFormat = ""
	switch precision {
	case PrecisionSecond:
		timeFormat = ":05"
		fallthrough
	case PrecisionMinute:
		timeFormat = ":04" + timeFormat
		fallthrough
	case PrecisionHour:
		timeFormat = "15" + timeFormat
		fallthrough
	case PrecisionDay:
		timeFormat = "2006-01-02T" + timeFormat
	}

	return fmt.Sprintf("%s/%s", constructVisibilitySearchPrefix(path, domainID, primaryIndexKey, primaryIndexValue, secondaryIndexKey), t.Format(timeFormat))
}

func constructTimestampIndex(path, domainID, primaryIndexKey, primaryIndexValue, secondaryIndexKey string, timestamp int64, runID string) string {
	t := time.Unix(0, timestamp).In(time.UTC)
	return fmt.Sprintf("%s/%s/%s", constructVisibilitySearchPrefix(path, domainID, primaryIndexKey, primaryIndexValue, secondaryIndexKey), t.Format(time.RFC3339), runID)
}

func constructVisibilitySearchPrefix(path, domainID, primaryIndexKey, primaryIndexValue, secondaryIndexType string) string {
	return strings.TrimLeft(strings.Join([]string{path, domainID, "visibility", primaryIndexKey, primaryIndexValue, secondaryIndexType}, "/"), "/")
}

func ensureContextTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultBlobstoreTimeout)
}
func upload(ctx context.Context, s3cli s3iface.S3API, URI archiver.URI, region, key string, data []byte) error {
	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()

	bucket, err := s3Bucket(URI, region)
	if err != nil {
		return err
	}
	_, err = s3cli.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchBucket {
				return &types.BadRequestError{Message: errBucketNotExists.Error()}
			}
		}
		return err
	}
	return nil
}

func download(ctx context.Context, s3cli s3iface.S3API, URI archiver.URI, region, key string) ([]byte, error) {
	ctx, cancel := ensureContextTimeout(ctx)
	defer cancel()
	bucket, err := s3Bucket(URI, region)
	if err != nil {
		return nil, err
	}
	result, err := s3cli.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeNoSuchBucket {
				return nil, &types.BadRequestError{Message: errBucketNotExists.Error()}
			}

			if aerr.Code() == s3.ErrCodeNoSuchKey {
				return nil, &types.EntityNotExistsError{Message: archiver.ErrHistoryNotExist.Error()}
			}
		}
		return nil, err
	}

	defer func() {
		if ierr := result.Body.Close(); ierr != nil {
			err = multierr.Append(err, ierr)
		}
	}()

	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func contextExpired(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func convertToExecutionInfo(record *visibilityRecord) *types.WorkflowExecutionInfo {
	return &types.WorkflowExecutionInfo{
		Execution: &types.WorkflowExecution{
			WorkflowID: record.WorkflowID,
			RunID:      record.RunID,
		},
		Type: &types.WorkflowType{
			Name: record.WorkflowTypeName,
		},
		StartTime:     common.Int64Ptr(record.StartTimestamp),
		ExecutionTime: common.Int64Ptr(record.ExecutionTimestamp),
		CloseTime:     common.Int64Ptr(record.CloseTimestamp),
		CloseStatus:   record.CloseStatus.Ptr(),
		HistoryLength: record.HistoryLength,
		Memo:          record.Memo,
		SearchAttributes: &types.SearchAttributes{
			IndexedFields: archiver.ConvertSearchAttrToBytes(record.SearchAttributes),
		},
	}
}
