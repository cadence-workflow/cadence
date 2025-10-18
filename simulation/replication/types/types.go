// Copyright (c) 2018 Uber Technologies, Inc.
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

package types

import (
	"fmt"
	"testing"
	"time"
)

const (
	DefaultTestCase = "testdata/replication_simulation_default.yaml"
	TasklistName    = "test-tasklist"

	TimerInterval = 5 * time.Second
)

type OperationFunction func(t *testing.T, op *Operation, simCfg *ReplicationSimulationConfig) error

type WorkflowInput struct {
	Duration             time.Duration
	ActivityCount        int
	ChildWorkflowID      string
	ChildWorkflowTimeout time.Duration
}

type WorkflowOutput struct {
	Count int
}

type ReplicationSimulation struct {
	RunIDRegistry map[string]string
}

func NewReplicationSimulation() *ReplicationSimulation {
	return &ReplicationSimulation{
		RunIDRegistry: make(map[string]string),
	}
}

func (s *ReplicationSimulation) StoreRunID(key, runID string) error {
	if s.RunIDRegistry == nil {
		return fmt.Errorf("runIDRegistry is nil")
	}
	s.RunIDRegistry[key] = runID
	return nil
}

func (s *ReplicationSimulation) GetRunID(key string) (string, error) {
	if s.RunIDRegistry == nil {
		return "", fmt.Errorf("runIDRegistry is nil")
	}
	return s.RunIDRegistry[key], nil
}

func WorkerIdentityFor(clusterName string, domainName string) string {
	if domainName == "" {
		return fmt.Sprintf("worker-%s", clusterName)
	}
	return fmt.Sprintf("worker-%s-%s", domainName, clusterName)
}

func Logf(t *testing.T, msg string, args ...interface{}) {
	t.Helper()
	msg = time.Now().Format(time.RFC3339Nano) + "\t" + msg
	t.Logf(msg, args...)
}
