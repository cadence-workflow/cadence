// Copyright (c) 2020 Uber Technologies, Inc.
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

package collection

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type (
	concurrentQueueSuite struct {
		*require.Assertions
		suite.Suite

		concurrentQueue *concurrentQueueImpl[int]
	}
)

func TestConcurrentQueueSuite(t *testing.T) {
	s := new(concurrentQueueSuite)
	suite.Run(t, s)
}

func (s *concurrentQueueSuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.concurrentQueue = NewConcurrentQueue[int]().(*concurrentQueueImpl[int])
}

func (s *concurrentQueueSuite) TearDownTest() {
}

func (s *concurrentQueueSuite) TestAddAndRemove() {
	s.Equal(0, s.concurrentQueue.Len())
	s.True(s.concurrentQueue.IsEmpty())
	_, err := s.concurrentQueue.Peek()
	s.Error(err)
	_, err = s.concurrentQueue.Remove()
	s.Error(err)

	numItems := 100
	items := make([]int, 0, numItems)
	for i := 0; i != 100; i++ {
		num := rand.Int()
		items = append(items, num)
		s.concurrentQueue.Add(num)
		s.Equal(i+1, s.concurrentQueue.Len())
	}
	s.False(s.concurrentQueue.IsEmpty())
	num, err := s.concurrentQueue.Peek()
	s.NoError(err)
	s.Equal(items[0], num)

	for i := 0; i != 100; i++ {
		num, err := s.concurrentQueue.Remove()
		s.NoError(err)
		s.Equal(items[i], num)
		s.Equal(numItems-i-1, s.concurrentQueue.Len())
	}
	s.True(s.concurrentQueue.IsEmpty())
	_, err = s.concurrentQueue.Peek()
	s.Error(err)
	_, err = s.concurrentQueue.Remove()
	s.Error(err)
}

func (s *concurrentQueueSuite) TestMultipleProducer() {
	concurrency := 10
	numItemsPerProducer := 10

	var wg sync.WaitGroup
	for i := 0; i != concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j != numItemsPerProducer; j++ {
				s.concurrentQueue.Add(rand.Int())
			}
		}()
	}
	wg.Wait()

	expectedLength := concurrency * numItemsPerProducer
	s.Equal(expectedLength, s.concurrentQueue.Len())
	s.False(s.concurrentQueue.IsEmpty())
	for i := 0; i != expectedLength; i++ {
		_, _ = s.concurrentQueue.Remove()
	}
}

func BenchmarkConcurrentQueue(b *testing.B) {
	queue := NewConcurrentQueue[testTask]()

	for i := 0; i < 100; i++ {
		go send(queue)
	}

	for n := 0; n < b.N; n++ {
		remove(queue)
	}
}
