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

package isolationgroup

import (
	"context"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/types"
)

//go:generate mockgen -package $GOPACKAGE -source $GOFILE -destination isolation_group_mock.go -self_package github.com/uber/cadence/common/isolationgroup

// State is a heavily cached in-memory library for returning the state of what zones are healthy or
// drained presently. It may return an inclusive (allow-list based) or an exclusive (deny-list based) set of IsolationGroups
// depending on the implementation.
type State interface {
	common.Daemon
	// IsDrained answers the question - "is this particular isolationGroup drained?". Used by startWorkflow calls
	// and similar sync frontend calls to make routing decisions
	IsDrained(ctx context.Context, Domain string, IsolationGroup string) (bool, error)
	IsDrainedByDomainID(ctx context.Context, DomainID string, IsolationGroup string) (bool, error)

	// AvailableIsolationGroupsByDomainID returns the available isolation zones for a domain.
	// Takes into account global and domain zones
	AvailableIsolationGroupsByDomainID(ctx context.Context, domainID string) (types.IsolationGroupConfiguration, error)

	Subscribe(domainID, key string, notifyChannel chan<- ChangeEvent) error
	Unsubscribe(domainID, key string) error

	// GetGlobalState returns the current configuration of isolation-groups which apply to all domains
	GetGlobalState(ctx context.Context) (*types.GetGlobalIsolationGroupsResponse, error)
	// UpdateGlobalState updates isolation-groups which apply to all domains
	UpdateGlobalState(ctx context.Context, state types.UpdateGlobalIsolationGroupsRequest) error
	// GetDomainState is the read operation for getting the current state of a domain's isolation-groups
	GetDomainState(ctx context.Context, request types.GetDomainIsolationGroupsRequest) (*types.GetDomainIsolationGroupsResponse, error)
	// UpdateDomainState is the read operation for updating a domain's isolation-groups
	UpdateDomainState(ctx context.Context, state types.UpdateDomainIsolationGroupsRequest) error
}
