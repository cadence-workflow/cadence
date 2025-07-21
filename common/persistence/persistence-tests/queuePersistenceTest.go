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
	"context"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type (
	// QueuePersistenceSuite contains queue persistence tests
	QueuePersistenceSuite struct {
		*TestBase
		// override suite.Suite.Assertions with require.Assertions; this means that s.NotNil(nil) will stop the test,
		// not merely log an error
		*require.Assertions
	}
)

// SetupSuite implementation
func (s *QueuePersistenceSuite) SetupSuite() {
	if testing.Verbose() {
		log.SetOutput(os.Stdout)
	}
}

// SetupTest implementation
func (s *QueuePersistenceSuite) SetupTest() {
	// Have to define our overridden assertions in the test setup. If we did it earlier, s.T() will return nil
	s.Assertions = require.New(s.T())
}

// TearDownSuite implementation
func (s *QueuePersistenceSuite) TearDownSuite() {
	s.TearDownWorkflowStore()
}

// TestDomainReplicationQueue tests domain replication queue operations
func (s *QueuePersistenceSuite) TestDomainReplicationQueue() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	numMessages := 100
	concurrentSenders := 10
	messageChan := make(chan []byte, numMessages)
	var publishErrors []error
	var mu sync.Mutex

	go func() {
		for i := 0; i < numMessages; i++ {
			messageChan <- []byte{byte(i)}
		}
		close(messageChan)
	}()

	wg := sync.WaitGroup{}
	wg.Add(concurrentSenders)

	// Helper function for publishing with retry
	publishWithRetry := func(message []byte) error {
		var lastErr error
		for i := 0; i < 3; i++ {
			err := s.Publish(ctx, message)
			if err == nil {
				return nil
			}
			lastErr = err
			time.Sleep(20 * time.Millisecond)
		}
		return lastErr
	}

	// Concurrent publishing
	for i := 0; i < concurrentSenders; i++ {
		go func() {
			defer wg.Done()
			for message := range messageChan {
				err := publishWithRetry(message)
				if err != nil {
					mu.Lock()
					publishErrors = append(publishErrors, err)
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	result, err := s.GetReplicationMessages(ctx, -1, numMessages)
	s.Nil(err, "GetReplicationMessages failed.")
	s.Len(result, numMessages, "Expected %d messages, got %d", numMessages, len(result))

	// Verify message content
	messageSet := make(map[byte]bool)
	for _, msg := range result {
		messageSet[msg.Payload[0]] = true
	}
	s.Len(messageSet, numMessages, "Expected %d unique messages, got %d", numMessages, len(messageSet))
}

// TestQueueMetadataOperations tests queue metadata operations
func (s *QueuePersistenceSuite) TestQueueMetadataOperations() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	clusterAckLevels, err := s.GetAckLevels(ctx)
	s.Require().NoError(err)
	s.Assert().Len(clusterAckLevels, 0)

	err = s.UpdateAckLevel(ctx, 10, "test1")
	s.Require().NoError(err)

	clusterAckLevels, err = s.GetAckLevels(ctx)
	s.Require().NoError(err)
	s.Assert().Len(clusterAckLevels, 1)
	s.Assert().Equal(int64(10), clusterAckLevels["test1"])

	err = s.UpdateAckLevel(ctx, 20, "test1")
	s.Require().NoError(err)

	clusterAckLevels, err = s.GetAckLevels(ctx)
	s.Require().NoError(err)
	s.Assert().Len(clusterAckLevels, 1)
	s.Assert().Equal(int64(20), clusterAckLevels["test1"])

	err = s.UpdateAckLevel(ctx, 25, "test2")
	s.Require().NoError(err)

	err = s.UpdateAckLevel(ctx, 24, "test2")
	s.Require().NoError(err)

	clusterAckLevels, err = s.GetAckLevels(ctx)
	s.Require().NoError(err)
	s.Assert().Len(clusterAckLevels, 2)
	s.Assert().Equal(int64(20), clusterAckLevels["test1"])
	s.Assert().Equal(int64(25), clusterAckLevels["test2"])
}

// TestDomainReplicationDLQ tests domain DLQ operations
func (s *QueuePersistenceSuite) TestDomainReplicationDLQ() {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	maxMessageID := int64(100)
	numMessages := 100
	concurrentSenders := 10
	messageChan := make(chan []byte, numMessages) // Buffered channel
	var publishErrors []error
	var mu sync.Mutex

	go func() {
		for i := 0; i < numMessages; i++ {
			messageChan <- []byte{}
		}
		close(messageChan)
	}()

	wg := sync.WaitGroup{}
	wg.Add(concurrentSenders)

	// Helper function for publishing with retry
	publishWithRetry := func(message []byte) error {
		var lastErr error
		for i := 0; i < 3; i++ {
			err := s.PublishToDomainDLQ(ctx, message)
			if err == nil {
				return nil
			}
			lastErr = err
			time.Sleep(20 * time.Millisecond)
		}
		return lastErr
	}

	// Concurrent publishing
	for i := 0; i < concurrentSenders; i++ {
		go func() {
			defer wg.Done()
			for message := range messageChan {
				err := publishWithRetry(message)
				if err != nil {
					mu.Lock()
					publishErrors = append(publishErrors, err)
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Check for any publish errors
	s.Empty(publishErrors, "Some messages failed to publish")

	// Verify initial message count
	size, err := s.GetDomainDLQSize(ctx)
	s.NoError(err, "GetDomainDLQSize failed")
	s.Equal(int64(numMessages), size, "Unexpected initial message count")
	result1, token, err := s.GetMessagesFromDomainDLQ(ctx, -1, maxMessageID, numMessages/2, nil)
	s.Nil(err, "GetReplicationMessages failed.")
	s.NotNil(token)
	result2, token, err := s.GetMessagesFromDomainDLQ(ctx, -1, maxMessageID, numMessages, token)
	s.Nil(err, "GetReplicationMessages failed.")
	s.Equal(len(token), 0)
	s.Equal(len(result1)+len(result2), numMessages, "Total messages retrieved mismatch")

	// Verify all messages were retrieved
	_, _, err = s.GetMessagesFromDomainDLQ(ctx, -1, 1<<63-1, numMessages, nil)
	s.NoError(err, "GetReplicationMessages failed.")
	s.Equal(len(token), 0)

	// Verify message count after retrieval
	size, err = s.GetDomainDLQSize(ctx)
	s.NoError(err, "GetDomainDLQSize failed")
	s.Equal(int64(numMessages), size, "Message count changed after retrieval")

	// Delete last message
	lastMessageID := result2[len(result2)-1].ID
	err = s.DeleteMessageFromDomainDLQ(ctx, lastMessageID)
	s.NoError(err, "Failed to delete message")

	// Verify message count after deletion
	size, err = s.GetDomainDLQSize(ctx)
	s.NoError(err, "GetDomainDLQSize failed")
	s.Equal(int64(numMessages-1), size, "Message count incorrect after deletion")

	// Get messages after deletion
	result3, token, err := s.GetMessagesFromDomainDLQ(ctx, -1, maxMessageID, numMessages, token)
	s.Nil(err, "GetReplicationMessages failed.")
	s.Equal(len(token), 0)
	s.Equal(len(result3), numMessages-1, "Unexpected number of messages after deletion")

	// Range delete remaining messages
	err = s.RangeDeleteMessagesFromDomainDLQ(ctx, -1, lastMessageID)
	s.NoError(err, "Failed to range delete messages")

	// Verify final message count
	size, err = s.GetDomainDLQSize(ctx)
	s.NoError(err, "GetDomainDLQSize failed")
	s.Equal(int64(0), size, "Messages not fully deleted")

	// Verify no messages remain
	result4, token, err := s.GetMessagesFromDomainDLQ(ctx, -1, maxMessageID, numMessages, token)
	s.Nil(err, "GetReplicationMessages failed.")
	s.Equal(len(token), 0)
	s.Equal(len(result4), 0, "Messages still exist after range deletion")
}

// TestDomainDLQMetadataOperations tests queue metadata operations
func (s *QueuePersistenceSuite) TestDomainDLQMetadataOperations() {
	clusterName := "test"
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()

	ackLevel, err := s.GetDomainDLQAckLevel(ctx)
	s.Require().NoError(err)
	s.Equal(0, len(ackLevel))

	err = s.UpdateDomainDLQAckLevel(ctx, 10, clusterName)
	s.NoError(err)

	ackLevel, err = s.GetDomainDLQAckLevel(ctx)
	s.Require().NoError(err)
	s.Equal(int64(10), ackLevel[clusterName])

	err = s.UpdateDomainDLQAckLevel(ctx, 1, clusterName)
	s.NoError(err)

	ackLevel, err = s.GetDomainDLQAckLevel(ctx)
	s.Require().NoError(err)
	s.Equal(int64(10), ackLevel[clusterName])
}
