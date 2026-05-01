// Copyright (c) 2026 Uber Technologies, Inc.
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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/archiver"
)

func TestSoftValidateURI(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr error
	}{
		{"wrong scheme", "wrongscheme:///a/b/c", archiver.ErrURISchemeMismatch},
		{"s3 no bucket", "s3://", errNoBucketSpecified},
		{"s3 valid bucket", "s3://bucket", nil},
		{"s3 valid bucket with path", "s3://bucket/a/b/c", nil},

		{"s3-ap valid no path", "s3-ap://710914175400/dummy-accesspoint", nil},
		{"s3-ap valid with path", "s3-ap://710914175400/dummy-accesspoint/foo/bar", nil},
		{"s3-ap missing account", "s3-ap:///dummy-accesspoint", errInvalidAccessPointAcct},
		{"s3-ap account too short", "s3-ap://12345/dummy-accesspoint", errInvalidAccessPointAcct},
		{"s3-ap account too long", "s3-ap://1234567890123/dummy-accesspoint", errInvalidAccessPointAcct},
		{"s3-ap account non-numeric", "s3-ap://abcdefghijkl/dummy-accesspoint", errInvalidAccessPointAcct},
		{"s3-ap missing access point name", "s3-ap://710914175400", errInvalidAccessPointURI},
		{"s3-ap empty access point name", "s3-ap://710914175400/", errInvalidAccessPointURI},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := archiver.NewURI(tc.uri)
			require.NoError(t, err)
			require.Equal(t, tc.wantErr, softValidateURI(u))
		})
	}
}

func TestS3Bucket(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		region     string
		wantBucket string
		wantErr    error
	}{
		{
			name:       "plain bucket",
			uri:        "s3://test-bucket/some/path",
			region:     "us-east-1",
			wantBucket: "test-bucket",
		},
		{
			name:       "plain bucket empty region",
			uri:        "s3://test-bucket",
			region:     "",
			wantBucket: "test-bucket",
		},
		{
			name:       "access point us-east-1",
			uri:        "s3-ap://710914175400/dummy-accesspoint",
			region:     "us-east-1",
			wantBucket: "arn:aws:s3:us-east-1:710914175400:accesspoint/dummy-accesspoint",
		},
		{
			name:       "access point eu-west-1 same URI",
			uri:        "s3-ap://710914175400/dummy-accesspoint/ignored/path",
			region:     "eu-west-1",
			wantBucket: "arn:aws:s3:eu-west-1:710914175400:accesspoint/dummy-accesspoint",
		},
		{
			name:       "access point GovCloud uses aws-us-gov partition",
			uri:        "s3-ap://710914175400/dummy-accesspoint",
			region:     "us-gov-west-1",
			wantBucket: "arn:aws-us-gov:s3:us-gov-west-1:710914175400:accesspoint/dummy-accesspoint",
		},
		{
			name:       "access point China uses aws-cn partition",
			uri:        "s3-ap://710914175400/dummy-accesspoint",
			region:     "cn-north-1",
			wantBucket: "arn:aws-cn:s3:cn-north-1:710914175400:accesspoint/dummy-accesspoint",
		},
		{
			name:    "access point empty region",
			uri:     "s3-ap://710914175400/dummy-accesspoint",
			region:  "",
			wantErr: errEmptyAwsRegion,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := archiver.NewURI(tc.uri)
			require.NoError(t, err)
			got, err := s3Bucket(u, tc.region)
			if tc.wantErr != nil {
				require.Equal(t, tc.wantErr, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantBucket, got)
		})
	}
}

func TestS3KeyPath(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"plain bucket no path", "s3://test-bucket", ""},
		{"plain bucket with path", "s3://test-bucket/a/b/c", "/a/b/c"},
		{"access point no sub-path", "s3-ap://710914175400/dummy-accesspoint", ""},
		{"access point with sub-path", "s3-ap://710914175400/dummy-accesspoint/a/b/c", "/a/b/c"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := archiver.NewURI(tc.uri)
			require.NoError(t, err)
			require.Equal(t, tc.want, s3KeyPath(u))
		})
	}
}

func TestPartitionForRegion(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{"us-east-1", "aws"},
		{"eu-west-1", "aws"},
		{"ap-south-1", "aws"},
		{"us-gov-west-1", "aws-us-gov"},
		{"us-gov-east-1", "aws-us-gov"},
		{"cn-north-1", "aws-cn"},
		{"cn-northwest-1", "aws-cn"},
		{"us-iso-east-1", "aws-iso"},
		{"us-isob-east-1", "aws-iso-b"},
		{"", "aws"},
	}
	for _, tc := range tests {
		t.Run(tc.region, func(t *testing.T) {
			require.Equal(t, tc.want, partitionForRegion(tc.region))
		})
	}
}
