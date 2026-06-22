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

package persistence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/uber/cadence/common/constants"
	"github.com/uber/cadence/common/types"
)

func TestDataBlob(t *testing.T) {
	t.Run("NewDataBlob", func(t *testing.T) {
		assert.Nil(t, NewDataBlob(nil, "anything"), "nil data should become nil blob")
		data, encoding := []byte("something"), constants.EncodingTypeJSON
		assert.EqualValues(t, &DataBlob{
			Data:     data,
			Encoding: encoding,
		}, NewDataBlob(data, encoding))
	})
	t.Run("Y-prefixed data may panic", func(t *testing.T) {
		allEncodings := []constants.EncodingType{
			constants.EncodingTypeJSON,
			constants.EncodingTypeThriftRW,
			constants.EncodingTypeGob,
			constants.EncodingTypeUnknown,
			constants.EncodingTypeEmpty,
			constants.EncodingTypeProto,
		}
		const problematic = "Y..."

		// I'm not entirely sure why this behavior exists, but to nail it down:
		assert.NotPanics(t, func() {
			NewDataBlob([]byte(problematic), constants.EncodingTypeThriftRW)
		}, "only thriftrw data can start with Y without panicking")

		// all others panic
		for _, encoding := range allEncodings {
			if encoding == constants.EncodingTypeThriftRW {
				continue // handled above
			}
			assert.Panicsf(t, func() {
				NewDataBlob([]byte(problematic), encoding)
			}, "non-thriftrw %v should panic if first byte is Y", encoding)
		}

		// make sure other data/encoding combinations do not panic for any encoding
		for _, dat := range []string{"Z...", "other"} {
			for _, encoding := range allEncodings {
				assert.NotPanicsf(t, func() {
					NewDataBlob([]byte(dat), encoding)
				}, "should not panic with %q on encoding %q", dat, encoding)
			}
		}
	})
	t.Run("FromDataBlob", func(t *testing.T) {
		assertEmpty := func(b []byte, s string) {
			assert.Nil(t, b, "should have nil bytes")
			assert.Empty(t, s, "should have empty string")
		}
		assertEmpty(FromDataBlob(nil))
		assertEmpty(FromDataBlob(&DataBlob{}))
		assertEmpty(FromDataBlob(&DataBlob{Encoding: constants.EncodingTypeThriftRW}))

		dat, str := FromDataBlob(&DataBlob{
			Data:     []byte("data"),
			Encoding: constants.EncodingTypeJSON,
		})
		assert.Equal(t, []byte("data"), dat, "data should be returned")
		assert.Equal(t, string(constants.EncodingTypeJSON), str, "encoding should be returned as a string")
	})
	t.Run("ToNilSafeDataBlob", func(t *testing.T) {
		orig := &DataBlob{Encoding: constants.EncodingTypeGob} // anything not empty
		assert.Equal(t, orig, orig.ToNilSafeDataBlob())
		assert.NotNil(t, (*DataBlob)(nil).ToNilSafeDataBlob(), "typed nils should convert to non-nils")
	})
	t.Run("GetEncodingString", func(t *testing.T) {
		assert.Equal(t, "", (*DataBlob)(nil).GetEncodingString())
		assert.Equal(t, "test", (&DataBlob{Encoding: "test"}).GetEncodingString())
	})
	t.Run("GetData", func(t *testing.T) {
		// this method returns empty slices, not nils.
		// I'm not sure if this needs to be maintained, but it must be checked before changing.
		assert.Equal(t, []byte{}, (*DataBlob)(nil).GetData())
		assert.Equal(t, []byte{}, (&DataBlob{}).GetData())
		assert.Equal(t, []byte{}, (&DataBlob{Data: []byte{}}).GetData())
		assert.Equal(t, []byte("test"), (&DataBlob{Data: []byte("test")}).GetData())
	})
	t.Run("GetEncoding", func(t *testing.T) {
		same := func(encoding constants.EncodingType) {
			assert.Equal(t, encoding, (&DataBlob{Encoding: encoding}).GetEncoding())
		}
		unknown := func(encoding constants.EncodingType) {
			assert.Equal(t, constants.EncodingTypeUnknown, (&DataBlob{Encoding: encoding}).GetEncoding())
		}
		// obvious
		same(constants.EncodingTypeGob)
		same(constants.EncodingTypeJSON)
		same(constants.EncodingTypeThriftRW)
		same(constants.EncodingTypeEmpty)

		// highly suspicious
		unknown(constants.EncodingTypeProto)

		// should be unknown
		unknown(constants.EncodingTypeUnknown)
		unknown("any other value")
	})
	t.Run("to and from internal", func(t *testing.T) {
		data := []byte("some data")
		t.Run("supported types", func(t *testing.T) {
			check := func(t *testing.T, encodingType types.EncodingType, encodingCommon constants.EncodingType) {
				internal := &types.DataBlob{
					EncodingType: encodingType.Ptr(),
					Data:         data,
				}
				blob := &DataBlob{
					Encoding: encodingCommon,
					Data:     data,
				}
				assert.Equalf(t, internal, blob.ToInternal(), "%v should encode to internal type %v", encodingCommon, encodingType)
				assert.Equalf(t, blob, NewDataBlobFromInternal(internal), "%v should decode from internal type %v", encodingCommon, encodingType)
				// likely proven by above, but to be explicit: this type should round-trip without losing data.
				assert.Equalf(t, blob, NewDataBlobFromInternal(blob.ToInternal()), "%v should round trip from blob %v", encodingCommon, encodingType)
				assert.Equalf(t, internal, NewDataBlobFromInternal(internal).ToInternal(), "%v should round trip from internal %v", encodingCommon, encodingType)
			}

			t.Run("json", func(t *testing.T) {
				check(t, types.EncodingTypeJSON, constants.EncodingTypeJSON)
			})
			t.Run("thriftrw", func(t *testing.T) {
				check(t, types.EncodingTypeThriftRW, constants.EncodingTypeThriftRW)
			})
		})

		t.Run("other known encodings panic to internal", func(t *testing.T) {
			for _, encoding := range []constants.EncodingType{
				constants.EncodingTypeUnknown,
				constants.EncodingTypeProto,
				constants.EncodingTypeGob,
				constants.EncodingTypeEmpty,
				"any other value",
			} {
				assert.Panicsf(t, func() {
					(&DataBlob{
						Encoding: encoding,
						Data:     data,
					}).ToInternal()
				}, "should panic when encoding to unhandled encoding %q", encoding)
			}
		})

		t.Run("unknown encodings panic from internal", func(t *testing.T) {
			// these two are known, any other value should panic.
			//
			// the easy and most-likely-to-catch-changes strategy is to just add 1,
			// so a new supported type will automatically fail here until both
			// the code and tests are updated.
			unknownType := 1 + max(types.EncodingTypeJSON, types.EncodingTypeThriftRW)
			assert.Panicsf(t, func() {
				NewDataBlobFromInternal(&types.DataBlob{
					EncodingType: &unknownType,
					Data:         data,
				})
			}, "should panic when decoding from unhandled encoding %q", unknownType)
		})
	})
}

func TestInternalReplicationTaskInfo_ToTask(t *testing.T) {
	creationTime := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	base := InternalReplicationTaskInfo{
		DomainID:     "domain-id",
		WorkflowID:   "wf-id",
		RunID:        "run-id",
		TaskID:       42,
		Version:      7,
		CreationTime: creationTime,
	}

	cases := []struct {
		name     string
		mutate   func(i *InternalReplicationTaskInfo)
		expected Task
	}{
		{
			name: "history replication task",
			mutate: func(i *InternalReplicationTaskInfo) {
				i.TaskType = ReplicationTaskTypeHistory
				i.FirstEventID = 1
				i.NextEventID = 5
				i.BranchToken = []byte("branch")
				i.NewRunBranchToken = []byte("new-run")
			},
			expected: &HistoryReplicationTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "run-id",
				},
				TaskData: TaskData{
					Version:             7,
					TaskID:              42,
					VisibilityTimestamp: creationTime,
				},
				FirstEventID:      1,
				NextEventID:       5,
				BranchToken:       []byte("branch"),
				NewRunBranchToken: []byte("new-run"),
			},
		},
		{
			name: "sync activity task",
			mutate: func(i *InternalReplicationTaskInfo) {
				i.TaskType = ReplicationTaskTypeSyncActivity
				i.ScheduledID = 99
			},
			expected: &SyncActivityTask{
				WorkflowIdentifier: WorkflowIdentifier{
					DomainID:   "domain-id",
					WorkflowID: "wf-id",
					RunID:      "run-id",
				},
				TaskData: TaskData{
					Version:             7,
					TaskID:              42,
					VisibilityTimestamp: creationTime,
				},
				ScheduledID: 99,
			},
		},
		{
			name: "failover marker task",
			mutate: func(i *InternalReplicationTaskInfo) {
				i.TaskType = ReplicationTaskTypeFailoverMarker
			},
			expected: &FailoverMarkerTask{
				TaskData: TaskData{
					Version:             7,
					TaskID:              42,
					VisibilityTimestamp: creationTime,
				},
				DomainID: "domain-id",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := base
			tc.mutate(&info)
			task, err := info.ToTask()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, task)
		})
	}

	t.Run("unknown task type returns error", func(t *testing.T) {
		info := base
		info.TaskType = -1
		task, err := info.ToTask()
		assert.Nil(t, task)
		assert.Error(t, err)
	})
}
