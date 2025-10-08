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

package types

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestDataBlobDeepCopy(t *testing.T) {
	tests := []struct {
		name  string
		input *DataBlob
	}{
		{
			name:  "nil",
			input: nil,
		},
		{
			name:  "empty",
			input: &DataBlob{},
		},
		{
			name: "thrift ok",
			input: &DataBlob{
				EncodingType: EncodingTypeThriftRW.Ptr(),
				Data:         []byte("some thrift data"),
			},
		},
		{
			name: "json ok",
			input: &DataBlob{
				EncodingType: EncodingTypeJSON.Ptr(),
				Data:         []byte("some json data"),
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.input.DeepCopy()
			assert.Equal(t, tc.input, got)
			if tc.input != nil && tc.input.Data != nil && identicalByteArray(tc.input.Data, got.Data) {
				t.Error("expected DeepCopy to return a new data slice")
			}
		})
	}
}

func TestActiveClustersConfigDeepCopy(t *testing.T) {
	normalConfig := &ActiveClusters{
		ActiveClustersByRegion: map[string]ActiveClusterInfo{
			"us-east-1": {
				ActiveClusterName: "us-east-1-cluster",
				FailoverVersion:   1,
			},
			"us-east-2": {
				ActiveClusterName: "us-east-2-cluster",
				FailoverVersion:   2,
			},
		},
	}

	tests := []struct {
		name   string
		input  *ActiveClusters
		expect *ActiveClusters
	}{
		{
			name:   "nil case",
			input:  nil,
			expect: nil,
		},
		{
			name:  "empty case",
			input: &ActiveClusters{},
			expect: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{},
			},
		},
		{
			name:   "normal case",
			input:  normalConfig,
			expect: normalConfig,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deepCopy := tc.input.DeepCopy()
			if diff := cmp.Diff(tc.expect, deepCopy); diff != "" {
				t.Errorf("DeepCopy() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsActiveActiveDomain(t *testing.T) {
	tests := []struct {
		name           string
		activeClusters *DomainReplicationConfiguration
		want           bool
	}{
		{
			name:           "empty DomainReplicationConfiguration should return false",
			activeClusters: &DomainReplicationConfiguration{},
			want:           false,
		},
		{
			name:           "nil receiver should return false",
			activeClusters: nil,
			want:           false,
		},
		{
			name:           "empty ActiveClusters should return false",
			activeClusters: &DomainReplicationConfiguration{ActiveClusters: &ActiveClusters{}},
			want:           false,
		},
		{
			name: "ActiveClusters with only old format populated should return true",
			activeClusters: &DomainReplicationConfiguration{
				ActiveClusters: &ActiveClusters{
					ActiveClustersByRegion: map[string]ActiveClusterInfo{
						"us-east-1": {ActiveClusterName: "cluster1"},
					},
				},
			},
			want: true,
		},
		{
			name: "ActiveClusters with only new format populated should return true",
			activeClusters: &DomainReplicationConfiguration{
				ActiveClusters: &ActiveClusters{
					AttributeScopes: map[string]ClusterAttributeScope{
						"region": {ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "cluster1"},
						}},
					},
				},
			},
			want: true,
		},
		{
			name: "ActiveClusters with both formats populated should return true",
			activeClusters: &DomainReplicationConfiguration{
				ActiveClusters: &ActiveClusters{
					ActiveClustersByRegion: map[string]ActiveClusterInfo{
						"us-east-1": {ActiveClusterName: "cluster1"},
					},
					AttributeScopes: map[string]ClusterAttributeScope{
						"region": {ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "cluster1"},
						}},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.activeClusters.IsActiveActive()
			assert.Equal(t, tt.want, got)
		})
	}
}

// identicalByteArray returns true if a and b are the same slice, false otherwise.
func identicalByteArray(a, b []byte) bool {
	return len(a) == len(b) && unsafe.SliceData(a) == unsafe.SliceData(b)
}

func TestActiveClusters_GetActiveClusterByRegion(t *testing.T) {
	tests := []struct {
		name           string
		activeClusters *ActiveClusters
		region         string
		wantInfo       ActiveClusterInfo
		wantErr        error
	}{
		{
			name:           "nil receiver should return ErrDomainNotActiveActive",
			activeClusters: nil,
			region:         "us-east-1",
			wantInfo:       ActiveClusterInfo{},
			wantErr:        ErrDomainNotActiveActive,
		},
		{
			name:           "empty region string should return ErrDomainNotActiveActive",
			activeClusters: &ActiveClusters{},
			region:         "",
			wantInfo:       ActiveClusterInfo{},
			wantErr:        ErrDomainNotActiveActive,
		},
		{
			name: "only old format populated should return from old format",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   100,
					},
				},
			},
			region: "us-east-1",
			wantInfo: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantErr: nil,
		},
		{
			name: "only new format populated should return from new format",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-west-1": {
								ActiveClusterName: "cluster2",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			region: "us-west-1",
			wantInfo: ActiveClusterInfo{
				ActiveClusterName: "cluster2",
				FailoverVersion:   200,
			},
			wantErr: nil,
		},
		{
			name: "both formats populated should prefer old format for backward compatibility",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "old-cluster",
						FailoverVersion:   100,
					},
				},
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {
								ActiveClusterName: "new-cluster",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			region: "us-east-1",
			wantInfo: ActiveClusterInfo{
				ActiveClusterName: "old-cluster",
				FailoverVersion:   100,
			},
			wantErr: nil,
		},
		{
			name: "region not found in either format should return ErrActiveClusterInfoNotFound",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   100,
					},
				},
			},
			region:   "us-west-1",
			wantInfo: ActiveClusterInfo{},
			wantErr:  ErrActiveClusterInfoNotFound,
		},
		{
			name:           "empty ActiveClusters should return ErrActiveClusterInfoNotFound when region is non-empty",
			activeClusters: &ActiveClusters{},
			region:         "us-east-1",
			wantInfo:       ActiveClusterInfo{},
			wantErr:        ErrActiveClusterInfoNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInfo, gotErr := tt.activeClusters.GetActiveClusterByRegion(tt.region)

			// Check error
			if tt.wantErr != nil {
				if gotErr == nil {
					t.Errorf("GetActiveClusterByRegion() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(gotErr, tt.wantErr) {
					t.Errorf("GetActiveClusterByRegion() error = %v, want %v", gotErr, tt.wantErr)
				}
			} else if gotErr != nil {
				t.Errorf("GetActiveClusterByRegion() unexpected error = %v", gotErr)
			}

			// Check info
			if diff := cmp.Diff(tt.wantInfo, gotInfo); diff != "" {
				t.Errorf("GetActiveClusterByRegion() info mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestActiveClusters_GetAllClusters(t *testing.T) {
	tests := []struct {
		name           string
		activeClusters *ActiveClusters
		want           []string
	}{
		{
			name:           "nil receiver should return empty slice",
			activeClusters: nil,
			want:           []string{},
		},
		{
			name:           "empty ActiveClusters should return empty slice",
			activeClusters: &ActiveClusters{},
			want:           []string{},
		},
		{
			name: "only old format populated should return attribute names from old format sorted",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-west-1": {ActiveClusterName: "cluster2"},
					"us-east-1": {ActiveClusterName: "cluster1"},
					"eu-west-1": {ActiveClusterName: "cluster3"},
				},
			},
			want: []string{"eu-west-1", "us-east-1", "us-west-1"},
		},
		{
			name: "only new format populated should return attribute names from new format sorted",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"ap-south-1": {ActiveClusterName: "cluster4"},
							"eu-north-1": {ActiveClusterName: "cluster5"},
						},
					},
				},
			},
			want: []string{"ap-south-1", "eu-north-1"},
		},
		{
			name: "both formats with different attribute names should return deduplicated sorted list",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {ActiveClusterName: "cluster1"},
					"us-west-1": {ActiveClusterName: "cluster2"},
				},
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"eu-west-1":  {ActiveClusterName: "cluster3"},
							"ap-south-1": {ActiveClusterName: "cluster4"},
						},
					},
				},
			},
			want: []string{"ap-south-1", "eu-west-1", "us-east-1", "us-west-1"},
		},
		{
			name: "both formats with overlapping attribute names should return deduplicated sorted list",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {ActiveClusterName: "cluster1"},
					"us-west-1": {ActiveClusterName: "cluster2"},
				},
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {ActiveClusterName: "cluster1"},
							"eu-west-1": {ActiveClusterName: "cluster3"},
						},
					},
				},
			},
			want: []string{"eu-west-1", "us-east-1", "us-west-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.activeClusters.GetAllClusters()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetAllClusters() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestActiveClusters_SetClusterForRegion(t *testing.T) {
	tests := []struct {
		name           string
		activeClusters *ActiveClusters
		region         string
		info           ActiveClusterInfo
		wantErr        error
		wantOldFormat  map[string]ActiveClusterInfo
		wantNewFormat  map[string]ActiveClusterInfo
	}{
		{
			name:           "nil receiver should return ErrDomainNotActiveActive",
			activeClusters: nil,
			region:         "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantErr: ErrDomainNotActiveActive,
		},
		{
			name:           "empty ActiveClusters should initialize both maps and set value",
			activeClusters: &ActiveClusters{},
			region:         "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantErr: nil,
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
			wantNewFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
		},
		{
			name: "existing old format should update both formats",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-west-1": {
						ActiveClusterName: "cluster2",
						FailoverVersion:   200,
					},
				},
			},
			region: "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantErr: nil,
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
				"us-west-1": {
					ActiveClusterName: "cluster2",
					FailoverVersion:   200,
				},
			},
			wantNewFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
		},
		{
			name: "existing new format should update both formats",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"eu-west-1": {
								ActiveClusterName: "cluster3",
								FailoverVersion:   300,
							},
						},
					},
				},
			},
			region: "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantErr: nil,
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
			wantNewFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
				"eu-west-1": {
					ActiveClusterName: "cluster3",
					FailoverVersion:   300,
				},
			},
		},
		{
			name: "both formats exist should update both",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-west-1": {
						ActiveClusterName: "cluster2",
						FailoverVersion:   200,
					},
				},
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"eu-west-1": {
								ActiveClusterName: "cluster3",
								FailoverVersion:   300,
							},
						},
					},
				},
			},
			region: "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantErr: nil,
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
				"us-west-1": {
					ActiveClusterName: "cluster2",
					FailoverVersion:   200,
				},
			},
			wantNewFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
				"eu-west-1": {
					ActiveClusterName: "cluster3",
					FailoverVersion:   300,
				},
			},
		},
		{
			name: "updating existing region should overwrite in both formats",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "old-cluster",
						FailoverVersion:   50,
					},
				},
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {
								ActiveClusterName: "old-cluster",
								FailoverVersion:   50,
							},
						},
					},
				},
			},
			region: "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "new-cluster",
				FailoverVersion:   100,
			},
			wantErr: nil,
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "new-cluster",
					FailoverVersion:   100,
				},
			},
			wantNewFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "new-cluster",
					FailoverVersion:   100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.activeClusters.SetClusterForRegion(tt.region, tt.info)

			// Check error
			if tt.wantErr != nil {
				if gotErr == nil {
					t.Errorf("SetClusterForRegion() expected error %v, got nil", tt.wantErr)
				} else if !errors.Is(gotErr, tt.wantErr) {
					t.Errorf("SetClusterForRegion() error = %v, want %v", gotErr, tt.wantErr)
				}
				return
			} else if gotErr != nil {
				t.Errorf("SetClusterForRegion() unexpected error = %v", gotErr)
				return
			}

			// Verify old format
			if diff := cmp.Diff(tt.wantOldFormat, tt.activeClusters.ActiveClustersByRegion); diff != "" {
				t.Errorf("SetClusterForRegion() old format mismatch (-want +got):\n%s", diff)
			}

			// Verify new format
			if tt.activeClusters.AttributeScopes != nil {
				if scope, ok := tt.activeClusters.AttributeScopes["region"]; ok {
					gotNewFormat := scope.ClusterAttributes
					if diff := cmp.Diff(tt.wantNewFormat, gotNewFormat); diff != "" {
						t.Errorf("SetClusterForRegion() new format mismatch (-want +got):\n%s", diff)
					}
				} else if len(tt.wantNewFormat) > 0 {
					t.Error("SetClusterForRegion() did not initialize region scope when expected")
				}
			} else if len(tt.wantNewFormat) > 0 {
				t.Error("SetClusterForRegion() did not initialize new format when expected")
			}
		})
	}
}

func TestActiveClusters_GetFailoverVersionForAttribute(t *testing.T) {
	tests := []struct {
		name           string
		activeClusters *ActiveClusters
		scopeType      string
		attributeName  string
		expected       int64
		expectedErr    error
	}{
		{
			name: "normal value / success case - this should provide the failover version",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   100,
							},
							"us-west-2": {
								ActiveClusterName: "cluster2",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			scopeType:     "region",
			attributeName: "us-east-1",
			expected:      100,
		},
		{
			name: "normal value / success case for zero values",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   0,
							},
						},
					},
				},
			},
			scopeType:     "region",
			attributeName: "us-east-1",
			expected:      0,
		},
		{
			name:           "nil receiver should return an error",
			activeClusters: nil,
			scopeType:      "region",
			attributeName:  "us-east-1",
			expected:       -1,
			expectedErr: &ClusterAttributeNotFoundError{
				ScopeType:     "region",
				AttributeName: "us-east-1",
			},
		},
		{
			name:           "empty ActiveClusters should return an error",
			activeClusters: &ActiveClusters{},
			scopeType:      "region",
			attributeName:  "us-east-1",
			expected:       -1,
			expectedErr: &ClusterAttributeNotFoundError{
				ScopeType:     "region",
				AttributeName: "us-east-1",
			},
		},
		{
			name: "nil AttributeScopes should return an error",
			activeClusters: &ActiveClusters{
				AttributeScopes: nil,
			},
			scopeType:     "region",
			attributeName: "us-east-1",
			expected:      -1,
			expectedErr: &ClusterAttributeNotFoundError{
				ScopeType:     "region",
				AttributeName: "us-east-1",
			},
		},
		{
			name: "scopeType not found should return an error",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]ActiveClusterInfo{
							"us-east-1": {
								ActiveClusterName: "cluster1",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			scopeType:     "datacenter",
			attributeName: "dc1",
			expected:      -1,
			expectedErr: &ClusterAttributeNotFoundError{
				ScopeType:     "datacenter",
				AttributeName: "dc1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := tt.activeClusters.GetFailoverVersionForAttribute(tt.scopeType, tt.attributeName)
			assert.Equal(t, tt.expected, got)
			assert.Equal(t, tt.expectedErr, gotErr)
		})
	}
}
