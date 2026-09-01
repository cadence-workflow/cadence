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
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

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
			name:  "every field empty or zero",
			owner: Owner{},
			want:  "0:::0",
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
		{
			// The most negative int64. Its magnitude has no positive counterpart, which is
			// where sign handling tends to go wrong.
			name:  "min hold id",
			owner: Owner{WorkflowID: "wf", RunID: "run-1", HoldID: -9223372036854775808},
			want:  "2:wf:run-1:-9223372036854775808",
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

// The two tests below are the randomized half of this file: the tables above pin the cases
// worth naming, these go looking for the ones nobody named. They are plain tests rather than
// Go fuzz targets on purpose. Without -fuzz a fuzz target only replays its seed corpus, and
// nothing in CI passes -fuzz, so a fuzz target here would never see an input its author had
// not already written down by hand.
//
// Being random, they are the one part of this file that can fail on a commit that changed
// nothing. Both log their seed so a failing run can be repeated.

// randomIterations is how many cases each of those tests draws, matching the mapper fuzz
// helper's default.
const randomIterations = 100

// ownerAlphabet is deliberately narrow. Every hard case in this format involves a separator or
// a digit — a workflow id holding separators, or one that looks like its own length prefix —
// and random unicode produces neither in any useful quantity. Drawing from these few bytes
// makes those shapes common instead of unreachable.
var ownerAlphabet = []byte(":0123456789-+abz")

func randomOwnerText(r *rand.Rand, maxLen int) string {
	b := make([]byte, r.Intn(maxLen+1))
	for i := range b {
		b[i] = ownerAlphabet[r.Intn(len(ownerAlphabet))]
	}
	return string(b)
}

func randomOwner(r *rand.Rand) Owner {
	holdID := int64(r.Uint64())
	// The extremes are worth hitting far more often than chance would give them, since both
	// bounds sit next to overflow in the parser.
	switch r.Intn(8) {
	case 0:
		holdID = 0
	case 1:
		holdID = math.MinInt64
	case 2:
		holdID = math.MaxInt64
	}
	return Owner{
		WorkflowID: randomOwnerText(r, 12),
		RunID:      randomOwnerText(r, 12),
		HoldID:     holdID,
	}
}

func TestRandomOwnersRoundTrip(t *testing.T) {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	defer func() {
		if t.Failed() {
			t.Logf("random seed: %d", seed)
		}
	}()

	for i := 0; i < randomIterations; i++ {
		owner := randomOwner(r)

		encoded := owner.String()
		parsed, err := ParseOwner(encoded)
		require.NoError(t, err, "String wrote %q, which does not parse", encoded)
		require.Equal(t, owner, parsed, "round trip changed the owner, encoded as %q", encoded)
	}
}

// numberDecorations are spellings strconv accepts for a number that String never writes. They
// are prepended to the two numeric fields directly rather than left to a byte edit: reaching
// "007" by chance means inserting one particular byte at one particular index, which over a
// hundred iterations happens near enough to never.
var numberDecorations = []string{"0", "00", "+"}

// randomOwnerID builds a string shaped like an encoding but not always canonical, so that a
// good share of them parse. Both failure directions matter: a string that parses when it
// should not, and one that parses to something re-encoding differently. Either would give one
// hold two spellings, and the release guard compares bytes.
func randomOwnerID(r *rand.Rand, owner Owner) string {
	lengthPrefix := strconv.Itoa(len(owner.WorkflowID))
	if r.Intn(4) == 0 {
		lengthPrefix = numberDecorations[r.Intn(len(numberDecorations))] + lengthPrefix
	}
	holdID := strconv.FormatInt(owner.HoldID, 10)
	if r.Intn(4) == 0 {
		holdID = numberDecorations[r.Intn(len(numberDecorations))] + holdID
	}

	id := lengthPrefix + ":" + owner.WorkflowID + ":" + owner.RunID + ":" + holdID
	if r.Intn(2) == 0 {
		id = editOneByte(r, id)
	}
	return id
}

// editOneByte damages the structure rather than the numbers: a separator moved, lost, or
// gained is what tells apart a parser that trusts the declared length from one that guesses.
func editOneByte(r *rand.Rand, id string) string {
	b := []byte(id)
	i := r.Intn(len(b))
	switch r.Intn(3) {
	case 0: // replace one byte
		b[i] = ownerAlphabet[r.Intn(len(ownerAlphabet))]
		return string(b)
	case 1: // insert one byte
		out := make([]byte, 0, len(b)+1)
		out = append(out, b[:i]...)
		out = append(out, ownerAlphabet[r.Intn(len(ownerAlphabet))])
		return string(append(out, b[i:]...))
	default: // drop one byte
		return string(append(b[:i], b[i+1:]...))
	}
}

func TestRandomIDsParseOnlyWhenCanonical(t *testing.T) {
	seed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(seed))
	defer func() {
		if t.Failed() {
			t.Logf("random seed: %d", seed)
		}
	}()

	var accepted int
	for i := 0; i < randomIterations; i++ {
		id := randomOwnerID(r, randomOwner(r))

		owner, err := ParseOwner(id)
		if err != nil {
			continue // a rejected string owes nothing
		}
		accepted++
		require.Equal(t, id, owner.String(), "ParseOwner accepted a spelling String would not write")
	}
	// Only the accepted strings exercise the assertion, so none accepted would mean this test
	// passed without checking anything.
	require.NotZero(t, accepted, "no mutated id parsed; the generator has stopped producing valid shapes")
}
