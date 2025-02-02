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

package replication

import (
	"time"

	"github.com/uber/cadence/client/admin"
	"github.com/uber/cadence/client/frontend"
)

const (
	defaultTestCase = "testdata/replication_simulation_default.yaml"
	domainName      = "test-domain"
	tasklistName    = "test-tasklist"
	workflowName    = "test-workflow"
	activityName    = "test-activity"

	timerInterval = 5 * time.Second
)

type ReplicationSimulationOperation string

const (
	ReplicationSimulationOperationStartWorkflow ReplicationSimulationOperation = "start_workflow"
	ReplicationSimulationOperationFailover      ReplicationSimulationOperation = "failover"
	ReplicationSimulationOperationValidate      ReplicationSimulationOperation = "validate"
)

type ReplicationSimulationConfig struct {
	Clusters map[string]*Cluster `yaml:"clusters"`

	PrimaryCluster string `yaml:"primaryCluster"`

	Operations []*Operation `yaml:"operations"`
}

type Operation struct {
	Type    ReplicationSimulationOperation `yaml:"op"`
	At      time.Duration                  `yaml:"at"`
	Cluster string                         `yaml:"cluster"`

	WorkflowID       string        `yaml:"workflowID"`
	WorkflowDuration time.Duration `yaml:"workflowDuration"`

	NewActiveCluster   string `yaml:"newActiveCluster"`
	FailovertimeoutSec *int32 `yaml:"failoverTimeoutSec"`

	Want Validation `yaml:"want"`
}

type Validation struct {
	Status                      string `yaml:"status"`
	StartedByWorkersInCluster   string `yaml:"startedByWorkersInCluster"`
	CompletedByWorkersInCluster string `yaml:"completedByWorkersInCluster"`
}

type Cluster struct {
	GRPCEndpoint string `yaml:"grpcEndpoint"`

	AdminClient    admin.Client    `yaml:"-"`
	FrontendClient frontend.Client `yaml:"-"`
}
