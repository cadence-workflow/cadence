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
			name:  "workflow id containing the separator",
			owner: Owner{WorkflowID: "a:b", RunID: "run-1", HoldID: 7},
			want:  "3:a:b:run-1:7",
		},
		{
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
		// MaxInt64 clears the earlier checks, so the bounds guard is all that stops a slice
		// panic. Writing that guard as length+1 would overflow and skip it.
		{name: "length prefix is max int64", ownerID: "9223372036854775807:wf:run:1"},
		{name: "length prefix is one below max int64", ownerID: "9223372036854775806:wf:run:1"},
		{name: "length prefix overflows int", ownerID: "99999999999999999999:wf:run:1"},
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

// FuzzParseOwnerAcceptsOnlyCanonicalIDs checks that every string ParseOwner accepts is exactly
// what String would have written; strings it rejects are skipped. That is what stops one hold
// from having more than one spelling, which matters because the release guard compares bytes.
func FuzzParseOwnerAcceptsOnlyCanonicalIDs(f *testing.F) {
	f.Add("4:wf-1:run-1:7")
	f.Add("0::run-1:3")
	f.Add("3:a:b::c::-9")
	f.Add("2:wf:run-1:-8")
	f.Add("007:wfabcde:run-1:7")
	f.Add("+7:wfabcde:run-1:7")
	f.Add("2:wf:run-1:007")

	f.Fuzz(func(t *testing.T, ownerID string) {
		owner, err := ParseOwner(ownerID)
		if err != nil {
			return // a rejected string owes nothing
		}
		require.Equal(t, ownerID, owner.String())
	})
}
