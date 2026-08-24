// Copyright (c) 2019 Uber Technologies, Inc.
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

package cloudsqlmysql

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/config"
)

func TestGetServiceAccountUsername(t *testing.T) {
	tests := []struct {
		name      string
		onGCE     bool
		email     string
		emailErr  error
		expected  string
		expectErr string
	}{
		{
			name:      "not on GCE",
			onGCE:     false,
			expectErr: "missing User",
		},
		{
			name:      "metadata server error",
			onGCE:     true,
			emailErr:  errors.New("metadata server unavailable"),
			expectErr: "metadata server unavailable",
		},
		{
			name:      "empty email",
			onGCE:     true,
			email:     "",
			expectErr: "missing User",
		},
		{
			name:      "email with empty username",
			onGCE:     true,
			email:     "@project.iam.gserviceaccount.com",
			expectErr: "missing User",
		},
		{
			name:     "service account email",
			onGCE:    true,
			email:    "my-sa@project.iam.gserviceaccount.com",
			expected: "my-sa",
		},
		{
			name:     "email without domain",
			onGCE:    true,
			email:    "my-sa",
			expected: "my-sa",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			restoreOnGCE, restoreGetMetadataUser := onGCE, getMetadataUser
			t.Cleanup(func() {
				onGCE, getMetadataUser = restoreOnGCE, restoreGetMetadataUser
			})

			onGCE = func() bool { return tc.onGCE }
			getMetadataUser = func(ctx context.Context, suffix string) (string, error) {
				require.Equal(t, "default", suffix)
				return tc.email, tc.emailErr
			}

			username, err := getServiceAccountUsername()
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, username)
		})
	}
}

func TestBuildDSNAttrs(t *testing.T) {
	tests := []struct {
		name           string
		connectAttrs   map[string]string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:           "excludes iamAuthN",
			connectAttrs:   map[string]string{"iamAuthN": "true"},
			wantNotContain: []string{"iamAuthN"},
			wantContains:   []string{"parseTime=true"},
		},
		{
			name:           "excludes ipType",
			connectAttrs:   map[string]string{"ipType": "private"},
			wantNotContain: []string{"ipType"},
			wantContains:   []string{"parseTime=true"},
		},
		{
			name:           "excludes both driver options",
			connectAttrs:   map[string]string{"iamAuthN": "true", "ipType": "private"},
			wantNotContain: []string{"iamAuthN", "ipType"},
			wantContains:   []string{"parseTime=true"},
		},
		{
			name:           "includes regular attributes",
			connectAttrs:   map[string]string{"iamAuthN": "true", "customAttr": "value"},
			wantContains:   []string{"customAttr=value", "parseTime=true"},
			wantNotContain: []string{"iamAuthN"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.SQL{
				ConnectAttributes: tc.connectAttrs,
			}
			result := buildDSNAttrs(cfg)

			for _, want := range tc.wantContains {
				require.True(t, strings.Contains(result, want),
					"DSN attrs should contain %q, got: %s", want, result)
			}
			for _, notWant := range tc.wantNotContain {
				require.False(t, strings.Contains(result, notWant),
					"DSN attrs should not contain %q, got: %s", notWant, result)
			}
		})
	}
}
