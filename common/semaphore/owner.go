package semaphore

import (
	"fmt"
	"strconv"
	"strings"
)

const ownerIDSeparator = ':'

// Owner identifies one hold on a semaphore: a specific run, and which of that run's holds.
type Owner struct {
	// WorkflowID is the user-supplied workflow name, so it can hold any byte, separator
	// included.
	WorkflowID string
	// RunID is the server-generated UUID for one execution of that workflow; a retry, a cron
	// fire, or a continue-as-new is a new run with a new id. A hold belongs to the run, not
	// to the workflow name.
	RunID string
	// HoldID is the id of the event that started the acquire. Taking it from the run's
	// history rather than minting one is what lets an owner_id survive replay, retries,
	// and failover. Event ids start at 1, though the encoding does not enforce it.
	HoldID int64
}

// String returns the canonical owner_id string:
//
//	<byteLen(WorkflowID)> ":" <WorkflowID> ":" <RunID> ":" <HoldID>
//
// The length prefix is what makes the string splittable again. WorkflowID is user-supplied and
// may contain the separator, so a plain join could not be taken apart; giving its byte length up
// front lets a reader skip over it without interpreting its contents.
//
// semaphore_tokens stores this string twice: as the token row's holder column and as the owner
// row's owner_id key. ReleaseSemaphoreToken guards its write on `IF holder = ?` with it, and
// Cassandra compares text bytewise. So one hold must have exactly one byte form — a release
// that supplies a string differing by a single byte matches nothing and silently leaves the
// slot held with no live run behind it.
//
// RunID needs no prefix of its own. HoldID is a decimal number, so the last separator in the
// string is always the one in front of it, and everything between the workflow id and that
// separator is the run id, whatever bytes it holds. Every Owner therefore has an encoding and
// String cannot fail.
func (o Owner) String() string {
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(o.WorkflowID)))
	b.WriteByte(ownerIDSeparator)
	b.WriteString(o.WorkflowID)
	b.WriteByte(ownerIDSeparator)
	b.WriteString(o.RunID)
	b.WriteByte(ownerIDSeparator)
	b.WriteString(strconv.FormatInt(o.HoldID, 10))
	return b.String()
}

// ParseOwner takes an owner_id back apart into the three ids that built it.
//
// It reads the digits before the first separator as the byte length of WorkflowID, takes
// exactly that many bytes verbatim, then splits what is left on its last separator into RunID
// and HoldID. Both string fields may therefore contain separators.
//
// It accepts only the spellings String produces: "7", never "007" or "+7", though a negative
// HoldID keeps its minus.
func ParseOwner(ownerID string) (Owner, error) {
	sep := strings.IndexByte(ownerID, ownerIDSeparator)
	if sep < 0 {
		return Owner{}, fmt.Errorf("owner_id %q has no length prefix", ownerID)
	}

	prefix := ownerID[:sep]
	length, err := strconv.Atoi(prefix)
	if err != nil || length < 0 {
		return Owner{}, fmt.Errorf("owner_id %q has an invalid length prefix %q", ownerID, prefix)
	}
	// Atoi accepts "007" and "+7"; String writes neither, so reject them rather than let one
	// hold have several spellings.
	if prefix != strconv.Itoa(length) {
		return Owner{}, fmt.Errorf("owner_id %q has a non-canonical length prefix %q", ownerID, prefix)
	}

	rest := ownerID[sep+1:]
	// The declared length must fit in rest, with a byte to spare for the separator.
	if length >= len(rest) {
		return Owner{}, fmt.Errorf("owner_id %q is shorter than its length prefix %d claims", ownerID, length)
	}
	workflowID := rest[:length]
	if rest[length] != ownerIDSeparator {
		return Owner{}, fmt.Errorf("owner_id %q has no separator after its %d-byte workflow id", ownerID, length)
	}

	// From the right: HoldID is a decimal number and cannot contain a separator, so the last
	// one always sits in front of it. Scanning from the left instead would cut a run id that
	// holds a separator in the wrong place.
	tail := rest[length+1:]
	mid := strings.LastIndexByte(tail, ownerIDSeparator)
	if mid < 0 {
		return Owner{}, fmt.Errorf("owner_id %q has no separator between run id and hold id", ownerID)
	}
	runID := tail[:mid]
	holdText := tail[mid+1:]

	holdID, err := strconv.ParseInt(holdText, 10, 64)
	if err != nil {
		return Owner{}, fmt.Errorf("owner_id %q has an invalid hold id %q", ownerID, holdText)
	}
	if holdText != strconv.FormatInt(holdID, 10) {
		return Owner{}, fmt.Errorf("owner_id %q has a non-canonical hold id %q", ownerID, holdText)
	}

	return Owner{WorkflowID: workflowID, RunID: runID, HoldID: holdID}, nil
}
