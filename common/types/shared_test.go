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

// identicalByteArray returns true if a and b are the same slice, false otherwise.
func identicalByteArray(a, b []byte) bool {
	return len(a) == len(b) && unsafe.SliceData(a) == unsafe.SliceData(b)
}

func TestActiveClusters_GetClusterByRegion(t *testing.T) {
	tests := []struct {
		name           string
		activeClusters *ActiveClusters
		region         string
		wantInfo       *ActiveClusterInfo
		wantFound      bool
	}{
		{
			name:           "nil receiver should return nil and false",
			activeClusters: nil,
			region:         "us-east-1",
			wantInfo:       nil,
			wantFound:      false,
		},
		{
			name:           "empty ActiveClusters should return nil and false",
			activeClusters: &ActiveClusters{},
			region:         "us-east-1",
			wantInfo:       nil,
			wantFound:      false,
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
			wantInfo: &ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantFound: true,
		},
		{
			name: "only new format populated should return from new format",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
							"us-west-1": {
								ActiveClusterName: "cluster2",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			region: "us-west-1",
			wantInfo: &ActiveClusterInfo{
				ActiveClusterName: "cluster2",
				FailoverVersion:   200,
			},
			wantFound: true,
		},
		{
			name: "both formats populated should prefer new format",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "old-cluster",
						FailoverVersion:   100,
					},
				},
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
							"us-east-1": {
								ActiveClusterName: "new-cluster",
								FailoverVersion:   200,
							},
						},
					},
				},
			},
			region: "us-east-1",
			wantInfo: &ActiveClusterInfo{
				ActiveClusterName: "new-cluster",
				FailoverVersion:   200,
			},
			wantFound: true,
		},
		{
			name: "region not found in either format should return nil and false",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   100,
					},
				},
			},
			region:    "us-west-1",
			wantInfo:  nil,
			wantFound: false,
		},
		{
			name: "empty region string should return nil and false",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {
						ActiveClusterName: "cluster1",
						FailoverVersion:   100,
					},
				},
			},
			region:    "",
			wantInfo:  nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInfo, gotFound := tt.activeClusters.GetClusterByRegion(tt.region)
			if gotFound != tt.wantFound {
				t.Errorf("GetClusterByRegion() gotFound = %v, want %v", gotFound, tt.wantFound)
			}
			if diff := cmp.Diff(tt.wantInfo, gotInfo); diff != "" {
				t.Errorf("GetClusterByRegion() info mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestActiveClusters_GetAllRegions(t *testing.T) {
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
			name: "only old format populated should return regions from old format sorted",
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
			name: "only new format populated should return regions from new format sorted",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
							"ap-south-1": {ActiveClusterName: "cluster4"},
							"eu-north-1": {ActiveClusterName: "cluster5"},
						},
					},
				},
			},
			want: []string{"ap-south-1", "eu-north-1"},
		},
		{
			name: "both formats with different regions should return deduplicated sorted list",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {ActiveClusterName: "cluster1"},
					"us-west-1": {ActiveClusterName: "cluster2"},
				},
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
							"eu-west-1":  {ActiveClusterName: "cluster3"},
							"ap-south-1": {ActiveClusterName: "cluster4"},
						},
					},
				},
			},
			want: []string{"ap-south-1", "eu-west-1", "us-east-1", "us-west-1"},
		},
		{
			name: "both formats with overlapping regions should return deduplicated sorted list",
			activeClusters: &ActiveClusters{
				ActiveClustersByRegion: map[string]ActiveClusterInfo{
					"us-east-1": {ActiveClusterName: "cluster1"},
					"us-west-1": {ActiveClusterName: "cluster2"},
				},
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
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
			got := tt.activeClusters.GetAllRegions()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetAllRegions() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestActiveClusters_SetClusterForRegion(t *testing.T) {
	// Note: SetClusterForRegion stores a copy (not a pointer to the original)
	// in the new format to prevent aliasing issues
	tests := []struct {
		name              string
		activeClusters    *ActiveClusters
		region            string
		info              ActiveClusterInfo
		wantOldFormat     map[string]ActiveClusterInfo
		wantNewFormat     map[string]*ActiveClusterInfo
		skipNilRecvrCheck bool
	}{
		{
			name:           "nil receiver should not panic",
			activeClusters: nil,
			region:         "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			skipNilRecvrCheck: true,
		},
		{
			name:           "empty ActiveClusters should initialize both maps and set value",
			activeClusters: &ActiveClusters{},
			region:         "us-east-1",
			info: ActiveClusterInfo{
				ActiveClusterName: "cluster1",
				FailoverVersion:   100,
			},
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
			wantNewFormat: map[string]*ActiveClusterInfo{
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
			wantNewFormat: map[string]*ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
		},
		{
			name: "existing new format should update both formats",
			activeClusters: &ActiveClusters{
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
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
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "cluster1",
					FailoverVersion:   100,
				},
			},
			wantNewFormat: map[string]*ActiveClusterInfo{
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
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
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
			wantNewFormat: map[string]*ActiveClusterInfo{
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
				AttributeScopes: map[string]*ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]*ActiveClusterInfo{
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
			wantOldFormat: map[string]ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "new-cluster",
					FailoverVersion:   100,
				},
			},
			wantNewFormat: map[string]*ActiveClusterInfo{
				"us-east-1": {
					ActiveClusterName: "new-cluster",
					FailoverVersion:   100,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.activeClusters.SetClusterForRegion(tt.region, tt.info)

			if tt.skipNilRecvrCheck {
				return
			}

			// Verify old format
			if diff := cmp.Diff(tt.wantOldFormat, tt.activeClusters.ActiveClustersByRegion); diff != "" {
				t.Errorf("SetClusterForRegion() old format mismatch (-want +got):\n%s", diff)
			}

			// Verify new format
			if tt.activeClusters.AttributeScopes != nil && tt.activeClusters.AttributeScopes["region"] != nil {
				gotNewFormat := tt.activeClusters.AttributeScopes["region"].ClusterAttributes
				if diff := cmp.Diff(tt.wantNewFormat, gotNewFormat); diff != "" {
					t.Errorf("SetClusterForRegion() new format mismatch (-want +got):\n%s", diff)
				}
			} else if len(tt.wantNewFormat) > 0 {
				t.Error("SetClusterForRegion() did not initialize new format when expected")
			}
		})
	}
}
