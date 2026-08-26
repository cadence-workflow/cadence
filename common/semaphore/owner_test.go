// Copyright (c) 2025 Uber Technologies, Inc.
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

package semaphore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOwnerStringRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		owner Owner
		want  string
	}{
		{
			name:  "plain ids",
			owner: Owner{WorkflowID: "wf-1", RunID: "run-1", HoldID: 7},
			want:  "4:wf-1:run-1:7",
		},
		{
			// The worked example from the design doc: without the length prefix this would
			// split into the wrong four pieces.
			name:  "workflow id containing the separator",
			owner: Owner{WorkflowID: "a:b", RunID: "run-1", HoldID: 7},
			want:  "3:a:b:run-1:7",
		},
		{
			// Five consecutive colons: the prefix separator, the three the workflow id is
			// made of, and the separator that closes it. Only the declared length of 3 tells
			// a reader where the workflow id ends.
			name:  "workflow id is only separators",
			owner: Owner{WorkflowID: ":::", RunID: "run-1", HoldID: 7},
			want:  "3:::::run-1:7",
		},
		{
			name:  "workflow id with leading and trailing separators",
			owner: Owner{WorkflowID: ":wf:", RunID: "run-1", HoldID: 1},
			want:  "4::wf::run-1:1",
		},
		{
			// A workflow id that itself looks like a length prefix. A parser that scanned for
			// digits rather than trusting the declared length could be led off course here.
			name:  "workflow id looks like a length prefix",
			owner: Owner{WorkflowID: "12:xy", RunID: "run-1", HoldID: 2},
			want:  "5:12:xy:run-1:2",
		},
		{
			name:  "empty workflow id",
			owner: Owner{WorkflowID: "", RunID: "run-1", HoldID: 3},
			want:  "0::run-1:3",
		},
		{
			// A run id is a UUID in practice, but the encoding does not depend on that: the
			// length prefix pins where the workflow id stops and the trailing decimal pins
			// where the hold id starts, so anything in between round-trips.
			name:  "run id containing the separator",
			owner: Owner{WorkflowID: "wf", RunID: "run:1", HoldID: 7},
			want:  "2:wf:run:1:7",
		},
		{
			name:  "both ids contain separators",
			owner: Owner{WorkflowID: "a:b", RunID: ":c:", HoldID: -9},
			want:  "3:a:b::c::-9",
		},
		{
			name:  "uuid run id",
			owner: Owner{WorkflowID: "wf", RunID: "3f2504e0-4f89-11d3-9a0c-0305e82c3301", HoldID: 12345},
			want:  "2:wf:3f2504e0-4f89-11d3-9a0c-0305e82c3301:12345",
		},
		{
			name:  "zero hold id",
			owner: Owner{WorkflowID: "wf", RunID: "run-1", HoldID: 0},
			want:  "2:wf:run-1:0",
		},
		{
			name:  "negative hold id",
			owner: Owner{WorkflowID: "wf", RunID: "run-1", HoldID: -8},
			want:  "2:wf:run-1:-8",
		},
		{
			name:  "max hold id",
			owner: Owner{WorkflowID: "wf", RunID: "run-1", HoldID: 9223372036854775807},
			want:  "2:wf:run-1:9223372036854775807",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.owner.String()
			assert.Equal(t, tc.want, got, "encoding must be byte-for-byte stable")

			parsed, err := ParseOwner(got)
			require.NoError(t, err)
			assert.Equal(t, tc.owner, parsed, "must round-trip")
		})
	}
}

func TestParseOwnerRejectsMalformed(t *testing.T) {
	tests := []struct {
		name    string
		ownerID string
	}{
		{name: "empty", ownerID: ""},
		{name: "no separator at all", ownerID: "nonsense"},
		{name: "length prefix is not a number", ownerID: "x:wf:run-1:1"},
		{name: "negative length prefix", ownerID: "-1:wf:run-1:1"},
		{name: "length prefix with leading zeros", ownerID: "004:wf-1:run-1:1"},
		{name: "length prefix with a plus sign", ownerID: "+4:wf-1:run-1:1"},
		{name: "length overruns the string", ownerID: "99:wf:run-1:1"},
		{name: "length leaves no room for a separator", ownerID: "2:wf"},
		{name: "no separator after the workflow id", ownerID: "2:wfrun-1:1"},
		{name: "missing hold id", ownerID: "2:wf:run-1"},
		{name: "hold id is not a number", ownerID: "2:wf:run-1:abc"},
		{name: "hold id with leading zeros", ownerID: "2:wf:run-1:007"},
		{name: "hold id overflows int64", ownerID: "2:wf:run-1:9223372036854775808"},
		{name: "trailing separator leaves an empty hold id", ownerID: "2:wf:run-1:1:"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseOwner(tc.ownerID)
			assert.Error(t, err)
		})
	}
}

func TestOwnerIDNeverCollidesWithStorageSentinels(t *testing.T) {
	// semaphore_tokens stores "__FREE__" and "__NONE__" in the same columns an owner_id can
	// occupy, and "__FREE__" is compared by the grant's LWT. An encoded owner_id always starts
	// with a decimal digit, so it can never equal either.
	for _, sentinel := range []string{"__FREE__", "__NONE__"} {
		_, err := ParseOwner(sentinel)
		assert.Error(t, err, "sentinel %q must not parse as an owner_id", sentinel)
	}

	owners := []Owner{
		{WorkflowID: "__FREE__", RunID: "run-1", HoldID: 1},
		{WorkflowID: "", RunID: "", HoldID: 0},
	}
	for _, o := range owners {
		encoded := o.String()
		assert.NotEqual(t, "__FREE__", encoded)
		assert.NotEqual(t, "__NONE__", encoded)
		assert.True(t, encoded[0] >= '0' && encoded[0] <= '9',
			"an encoded owner_id must start with a digit, got %q", encoded)
		assert.False(t, strings.HasPrefix(encoded, "_"))
	}
}

// FuzzOwnerStringRoundTrip checks the property the whole format exists for: every Owner
// encodes to a string that parses back into the same Owner. Round-tripping for all inputs
// also proves the encoding is injective, since two Owners sharing an encoding could not both
// come back out of it. The table above pins specific cases; this covers the ones nobody
// thought to write down.
func FuzzOwnerStringRoundTrip(f *testing.F) {
	f.Add("wf-1", "run-1", int64(7))
	f.Add("a:b", ":c:", int64(-9))
	f.Add("", "", int64(0))
	f.Add(":::", "run-1", int64(9223372036854775807))
	f.Add("12:xy", "3f2504e0-4f89-11d3-9a0c-0305e82c3301", int64(-9223372036854775808))

	f.Fuzz(func(t *testing.T, workflowID, runID string, holdID int64) {
		owner := Owner{WorkflowID: workflowID, RunID: runID, HoldID: holdID}
		parsed, err := ParseOwner(owner.String())
		require.NoError(t, err)
		require.Equal(t, owner, parsed)
	})
}

// FuzzParseOwnerReencodes checks the direction the canonical-form rules exist for: any string
// ParseOwner accepts must encode back to itself. Without it a caller could read a stored
// owner_id, re-encode it, and get different bytes — and the release guard, comparing bytes,
// would stop matching the row those bytes came from.
func FuzzParseOwnerReencodes(f *testing.F) {
	f.Add("4:wf-1:run-1:7")
	f.Add("0::run-1:3")
	f.Add("3:a:b::c::-9")
	f.Add("2:wf:run-1:-8")
	// Non-canonical spellings that are otherwise structurally valid, so they reach the
	// canonical checks instead of failing earlier. They must be rejected, never accepted and
	// silently re-encoded into different bytes. "wfabcde" is exactly the 7 bytes "007" claims.
	f.Add("007:wfabcde:run-1:7")
	f.Add("+7:wfabcde:run-1:7")
	f.Add("2:wf:run-1:007")

	f.Fuzz(func(t *testing.T, ownerID string) {
		owner, err := ParseOwner(ownerID)
		if err != nil {
			return // a rejected string carries no obligation
		}
		require.Equal(t, ownerID, owner.String())
	})
}
