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

package handler

import (
	"fmt"
	"sync"

	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/membership"
	"github.com/uber/cadence/common/service"
	"github.com/uber/cadence/service/matching/tasklist"
)

const subscriptionBufferSize = 1000

// Because there's a bunch of conditions under which matching may be holding a tasklist
// reader daemon and other live procesess but when it doesn't (according to the rest of the hashring)
// own the tasklist anymore, this listener watches for membership changes and purges anything disused
// in the hashring on membership changes.
//
// Combined with the guard on tasklist instantiation, it should prevent incorrect or poorly timed
// creating of tasklist ownership and database shard thrashing between hosts while they figure out
// which host is the real owner of the tasklist.
//
// This is not the main shutdown process, its just an optimization.
func (e *matchingEngineImpl) subscribeToMembershipChanges() {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("matching membership watcher changes caused a panic, recovering", tag.Dynamic("recovered-panic", r))
		}
	}()

	defer e.shutdownCompletion.Done()

	if !e.config.EnableTasklistOwnershipGuard() {
		return
	}

	listener := make(chan *membership.ChangedEvent, subscriptionBufferSize)
	if err := e.membershipResolver.Subscribe(service.Matching, "matching-engine", listener); err != nil {
		e.logger.Error("Failed to subscribe to membership updates")
		return
	}

	for {
		select {
		case event := <-listener:
			err := e.shutDownNonOwnedTasklists()
			if err != nil {
				e.logger.Error("Error while trying to determine if tasklists have been shutdown",
					tag.Error(err),
					tag.MembershipChangeEvent(event),
				)
			}
		case <-e.shutdown:
			return
		}
	}
}

func (e *matchingEngineImpl) shutDownNonOwnedTasklists() error {
	if !e.config.EnableTasklistOwnershipGuard() {
		return nil
	}
	noLongerOwned, err := e.getNonOwnedTasklistsLocked()
	if err != nil {
		return err
	}

	tasklistsShutdownWG := sync.WaitGroup{}

	for _, tl := range noLongerOwned {
		// for each of the tasklists that are no longer owned, kick off the
		// process of stopping them. The stopping process is IO heavy and
		// can take a while, so do them in parallel to efficiently unload tasklists not owned
		tasklistsShutdownWG.Add(1)
		go func(tl tasklist.Manager) {

			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("panic occurred while trying to shut down tasklist", tag.Dynamic("recovered-panic", r))
				}
			}()
			defer tasklistsShutdownWG.Done()

			e.logger.Info("shutting down tasklist preemptively because they are no longer owned by this host",
				tag.WorkflowTaskListType(tl.TaskListID().GetType()),
				tag.WorkflowTaskListName(tl.TaskListID().GetName()),
				tag.WorkflowDomainID(tl.TaskListID().GetDomainID()),
				tag.Dynamic("tasklist-debug-info", tl.String()),
			)

			e.unloadTaskList(tl)
		}(tl)
	}

	tasklistsShutdownWG.Wait()

	return nil
}

func (e *matchingEngineImpl) getNonOwnedTasklistsLocked() ([]tasklist.Manager, error) {
	if !e.config.EnableTasklistOwnershipGuard() {
		return nil, nil
	}

	var toShutDown []tasklist.Manager

	e.taskListsLock.RLock()
	defer e.taskListsLock.RUnlock()

	self, err := e.membershipResolver.WhoAmI()
	if err != nil {
		return nil, fmt.Errorf("failed to lookup self im membership: %w", err)
	}

	for tl, manager := range e.taskLists {
		taskListOwner, err := e.membershipResolver.Lookup(service.Matching, tl.GetName())
		if err != nil {
			return nil, fmt.Errorf("failed to lookup task list owner: %w", err)
		}

		if taskListOwner.Identity() != self.Identity() {
			toShutDown = append(toShutDown, manager)
		}
	}

	e.logger.Info("Got list of non-owned-tasklists",
		tag.Dynamic("tasklist-debug-info", toShutDown),
	)
	return toShutDown, nil
}
