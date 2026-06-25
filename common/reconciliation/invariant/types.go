// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
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

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination mocks.go -self_package github.com/uber/cadence/common/reconciliation/invariant
//go:generate enumer -type=Collection -output collection_enumer_generated.go

package invariant

import "context"

const (
	// CheckResultTypeFailed indicates a failure occurred while attempting to run check
	CheckResultTypeFailed CheckResultType = "failed"
	// CheckResultTypeCorrupted indicates check successfully ran and detected a corruption
	CheckResultTypeCorrupted CheckResultType = "corrupted"
	// CheckResultTypeHealthy indicates check successfully ran and detected no corruption
	CheckResultTypeHealthy CheckResultType = "healthy"

	// FixResultTypeSkipped indicates that fix skipped execution
	FixResultTypeSkipped FixResultType = "skipped"
	// FixResultTypeFixed indicates that fix successfully fixed an execution
	FixResultTypeFixed FixResultType = "fixed"
	// FixResultTypeFailed indicates that fix attempted to fix an execution but failed to do so
	FixResultTypeFailed FixResultType = "failed"

	// HistoryExists asserts that history must exist if concrete execution exists
	HistoryExists Name = "history_exists"

	// InactiveDomainExists asserts that if domain status is not registered that it's inactive
	InactiveDomainExists Name = "inactive_domain_exists"

	// OpenCurrentExecution asserts that an open concrete execution must have a valid current execution
	OpenCurrentExecution Name = "open_current_execution"
	// ConcreteExecutionExists asserts that an open current execution must have a valid concrete execution
	ConcreteExecutionExists Name = "concrete_execution_exists"

	// StaleWorkflow checks for workflows that exist beyond their retention window,
	// implying a failed cleanup / lost timers / etc of some kind.
	StaleWorkflow Name = "stale_workflow"

	// TimerInvalid checks for timers scheduled for non-existent or closed workflows
	TimerInvalid Name = "TimerInvalid"

	// HistoryInvalid checks that the full history is readable and not corrupted
	HistoryInvalid Name = "history_invalid"

	// MismatchedRecords checks that current and concrete execution records agree on close status
	MismatchedRecords Name = "mismatched_records"

	// CollectionMutableState is the collection of invariants relating to mutable state
	CollectionMutableState Collection = 0
	// CollectionHistory is the collection  of invariants relating to history
	CollectionHistory Collection = 1
	// CollectionDomain is the collection  of invariants relating to domain status
	CollectionDomain Collection = 2
	// CollectionStale contains the stale workflow scanner
	CollectionStale Collection = 3
)

type (
	// Name is the name of an invariant
	Name string

	// Collection is a type which indicates a collection of invariants
	Collection int

	// CheckResultType is the result type of running an invariant check
	CheckResultType string

	// FixResultType is the result type of running an invariant fix
	FixResultType string
)

// Invariant represents an invariant of a single execution.
// It can be used to check that the execution satisfies the invariant.
// It can also be used to fix the invariant for an execution.
type Invariant interface {
	Check(context.Context, interface{}) CheckResult
	Fix(context.Context, interface{}) FixResult
	Name() Name
}

// Manager represents a manager of several invariants.
// It can be used to run a group of invariant checks or fixes.
type Manager interface {
	RunChecks(context.Context, interface{}) ManagerCheckResult
	RunFixes(context.Context, interface{}) ManagerFixResult
}

// ManagerCheckResult is the result of running a list of checks
type ManagerCheckResult struct {
	CheckResultType          CheckResultType
	DeterminingInvariantType *Name
	CheckResults             []CheckResult
}

// ManagerFixResult is the result of running a list of fixes
type ManagerFixResult struct {
	FixResultType            FixResultType
	DeterminingInvariantName *Name
	FixResults               []FixResult
}

// CheckResult is the result of running Check.
type CheckResult struct {
	CheckResultType CheckResultType
	InvariantName   Name
	Info            string
	InfoDetails     string
}

// FixResult is the result of running Fix.
type FixResult struct {
	FixResultType FixResultType
	InvariantName Name
	CheckResult   CheckResult
	Info          string
	InfoDetails   string
}

// NamePtr returns a pointer to Name
func NamePtr(t Name) *Name {
	return &t
}
