// Copyright (c) 2021 Uber Technologies, Inc.
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

package host

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/engine/engineimpl"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/matching/tasklist"
)

func TestIntegrationQueueV2Suite(t *testing.T) {
	flag.Parse()

	clusterConfig, err := GetTestClusterConfig("testdata/integration_test_cluster.yaml")
	if err != nil {
		panic(err)
	}
	if clusterConfig.HistoryDynamicConfigOverrides == nil {
		clusterConfig.HistoryDynamicConfigOverrides = make(map[dynamicproperties.Key]interface{})
	}
	clusterConfig.HistoryDynamicConfigOverrides[dynamicproperties.EnableTimerQueueV2] = true
	clusterConfig.HistoryDynamicConfigOverrides[dynamicproperties.EnableTransferQueueV2] = true
	testCluster := NewPersistenceTestCluster(t, clusterConfig)

	s := new(IntegrationQueueV2Suite)
	params := IntegrationBaseParams{
		DefaultTestCluster:    testCluster,
		VisibilityTestCluster: testCluster,
		TestClusterConfig:     clusterConfig,
	}
	s.IntegrationSuite = &IntegrationSuite{
		IntegrationBase: NewIntegrationBase(params),
	}
	suite.Run(t, s)
}
