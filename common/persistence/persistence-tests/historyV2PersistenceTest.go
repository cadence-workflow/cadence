// Copyright (c) 2017 Uber Technologies, Inc.
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

package persistencetests

import (
	"os"
	"testing"

	"time"

	"sync/atomic"

	"sync"

	"fmt"

	"github.com/gocql/gocql"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	workflow "github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/backoff"
	p "github.com/uber/cadence/common/persistence"
)

type (
	// HistoryV2PersistenceSuite contains history persistence tests
	HistoryV2PersistenceSuite struct {
		suite.Suite
		TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

var historyTestRetryPolicy = createHistoryTestRetryPolicy()

func createHistoryTestRetryPolicy() backoff.RetryPolicy {
	policy := backoff.NewExponentialRetryPolicy(time.Millisecond * 50)
	policy.SetMaximumInterval(time.Second * 3)
	policy.SetExpirationInterval(time.Second * 30)

	return policy
}

func isConditionFail(err error) bool {
	switch err.(type) {
	case *p.ConditionFailedError:
		return true
	case *p.UnexpectedConditionFailedError:
		//TODO we need to understand why it can return UnexpectedConditionFailedError
		return true
	default:
		return false
	}
}

// SetupSuite implementation
func (s *HistoryV2PersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// SetupTest implementation
func (s *HistoryV2PersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

// TearDownSuite implementation
func (s *HistoryV2PersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

func (s *HistoryV2PersistenceSuite) genRandomUUIDString() string {
	uuid, err := gocql.RandomUUID()
	s.Nil(err)
	return uuid.String()
}

func (s *HistoryV2PersistenceSuite) TestConcurrentlyCreateAndDeleteEmptyBranches() {
	treeID := s.genRandomUUIDString()

	wg := sync.WaitGroup{}
	newCount := int32(0)
	concurrency := 5
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			brID := s.genRandomUUIDString()
			isNewTree, err := s.newHistoryBranch(treeID, brID)
			s.Nil(err)
			if isNewTree {
				atomic.AddInt32(&newCount, 1)
			}
		}()
	}

	wg.Wait()
	newCount = atomic.LoadInt32(&newCount)
	s.Equal(int32(1), newCount)

	branches := s.descTree(treeID)
	s.Equal(concurrency, len(branches))

	wg = sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(brID string) {
			defer wg.Done()
			err := s.deleteHistoryBranch(treeID, brID)
			s.Nil(err)
		}(branches[i].BranchID)
	}

	wg.Wait()
	branches = s.descTree(treeID)
	s.Equal(0, len(branches))

	// create one more try after delete the whole tree
	brID := s.genRandomUUIDString()
	isNewTree, err := s.newHistoryBranch(treeID, brID)
	s.Nil(err)
	s.Equal(true, isNewTree)
}

func (s *HistoryV2PersistenceSuite) TestConcurrentlyCreateAndAppendBranches() {
	treeID := s.genRandomUUIDString()

	wg := sync.WaitGroup{}
	newCount := int32(0)
	concurrency := 1

	// test create new branch along with appending new nodes
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			brID := s.genRandomUUIDString()
			isNewTree, err := s.newHistoryBranch(treeID, brID)
			s.Nil(err)
			if isNewTree {
				atomic.AddInt32(&newCount, 1)
			}
			fmt.Printf("done created branch %v \n", brID)
			historyW := &workflow.History{}
			events := s.genRandomEvents([]int64{1, 2, 3}, 1, 100*int64(1+idx))

			overrides, err := s.append(treeID, brID, 1, events, 0)
			s.Nil(err)
			s.Equal(0, overrides)
			historyW.Events = events

			events = s.genRandomEvents([]int64{4}, 1, 100*int64(1+idx))
			overrides, err = s.append(treeID, brID, 4, events, 0)
			s.Nil(err)
			s.Equal(0, overrides)
			historyW.Events = append(historyW.Events, events...)

			events = s.genRandomEvents([]int64{5, 6, 7, 8}, 1, 100*int64(1+idx))
			overrides, err = s.append(treeID, brID, 5, events, 0)
			s.Nil(err)
			s.Equal(0, overrides)
			historyW.Events = append(historyW.Events, events...)

			events = s.genRandomEvents([]int64{9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}, 1, 100*int64(1+idx))
			overrides, err = s.append(treeID, brID, 9, events, 0)
			s.Nil(err)
			s.Equal(0, overrides)
			historyW.Events = append(historyW.Events, events...)

			//read branch to verify
			branch := p.HistoryBranch{
				TreeID:    treeID,
				BranchID:  brID,
				Ancestors: nil,
			}
			historyR := &workflow.History{}
			bi, events, token, err := s.read(branch, 1, 21, 1, 10, nil)
			s.Nil(err)
			s.Equal(10, len(events))
			historyR.Events = events

			_, events, token, err = s.read(*bi, 1, 21, 1, 10, token)
			s.Nil(err)
			s.Equal(10, len(events))
			historyR.Events = append(historyR.Events, events...)

			// the next page should return empty events
			_, events, token, err = s.read(*bi, 1, 21, 1, 10, token)
			s.Nil(err)
			s.Equal(0, len(events))

			s.True(historyW.Equals(historyR))
		}(i)
	}

	wg.Wait()
	newCount = atomic.LoadInt32(&newCount)
	s.Equal(int32(1), newCount)
	branches := s.descTree(treeID)
	s.Equal(concurrency, len(branches))

	// test appending nodes(override and new nodes) on each branch concurrently
	//for i := 0; i < concurrency; i++ {
	//	wg.Add(1)
	//	go func(idx int) {
	//     defer wg.Done()
	//
	//	}(i)
	//}

	// Finally lets clean up all branches
	wg = sync.WaitGroup{}
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(brID string) {
			defer wg.Done()
			err := s.deleteHistoryBranch(treeID, brID)
			s.Nil(err)
		}(branches[i].BranchID)
	}

	wg.Wait()
	branches = s.descTree(treeID)
	s.Equal(0, len(branches))

}

func (s *HistoryV2PersistenceSuite) genRandomEvents(eventIDs []int64, version int64, timestamp int64) []*workflow.HistoryEvent {
	var events []*workflow.HistoryEvent
	for _, eid := range eventIDs {
		e := &workflow.HistoryEvent{EventId: common.Int64Ptr(eid), Version: common.Int64Ptr(version), Timestamp: int64Ptr(timestamp)}
		events = append(events, e)
	}

	return events
}

// NewHistoryBranch helper
func (s *HistoryV2PersistenceSuite) newHistoryBranch(treeID, branchID string) (bool, error) {

	isNewT := false

	op := func() error {
		var err error
		resp, err := s.HistoryMgr.NewHistoryBranch(&p.NewHistoryBranchRequest{
			BranchInfo: p.HistoryBranch{
				TreeID:   treeID,
				BranchID: branchID,
			},
		})
		if resp != nil && resp.IsNewTree {
			isNewT = true
		}
		return err
	}

	err := backoff.Retry(op, historyTestRetryPolicy, isConditionFail)
	return isNewT, err
}

func (s *HistoryV2PersistenceSuite) deleteHistoryBranch(treeID, branchID string) error {

	op := func() error {
		var err error
		err = s.HistoryMgr.DeleteHistoryBranch(&p.DeleteHistoryBranchRequest{
			BranchInfo: p.HistoryBranch{
				TreeID:   treeID,
				BranchID: branchID,
			},
		})
		return err
	}

	return backoff.Retry(op, historyTestRetryPolicy, isConditionFail)
}

func (s *HistoryV2PersistenceSuite) descTree(treeID string) []p.HistoryBranch {
	resp, err := s.HistoryMgr.GetHistoryTree(&p.GetHistoryTreeRequest{
		TreeID: treeID,
	})
	s.Nil(err)
	return resp.Branches
}

func (s *HistoryV2PersistenceSuite) read(branch p.HistoryBranch, minNodeID, maxNodeID, lastVersion int64, pageSize int, token []byte) (*p.HistoryBranch, []*workflow.HistoryEvent, []byte, error) {

	resp, err := s.HistoryMgr.ReadHistoryBranch(&p.ReadHistoryBranchRequest{
		BranchInfo:       branch,
		MinNodeID:        minNodeID,
		MaxNodeID:        maxNodeID,
		PageSize:         pageSize,
		NextPageToken:    token,
		LastEventVersion: lastVersion,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	if len(resp.History) > 0 {
		s.True(resp.Size > 0)
	}
	return &resp.BranchInfo, resp.History, resp.NextPageToken, nil
}

func (s *HistoryV2PersistenceSuite) append(treeID, branchID string, nextNodeID int64, events []*workflow.HistoryEvent, txnID int64) (int, error) {

	var resp *p.AppendHistoryNodesResponse

	op := func() error {
		var err error
		resp, err = s.HistoryMgr.AppendHistoryNodes(&p.AppendHistoryNodesRequest{
			BranchInfo: p.HistoryBranch{
				TreeID:   treeID,
				BranchID: branchID,
			},
			NextNodeID:    nextNodeID,
			Events:        events,
			TransactionID: txnID,
			Encoding:      pickRandomEncoding(),
		})
		return err
	}

	err := backoff.Retry(op, historyTestRetryPolicy, isConditionFail)
	if err != nil {
		return 0, err
	}
	s.True(resp.Size > 0)

	return resp.OverrideCount, err
}
