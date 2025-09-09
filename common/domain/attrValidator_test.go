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

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/metrics"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
)

type (
	attrValidatorSuite struct {
		suite.Suite

		minRetentionDays int
		validator        *AttrValidatorImpl
	}
)

func TestAttrValidatorSuite(t *testing.T) {
	s := new(attrValidatorSuite)
	suite.Run(t, s)
}

func (s *attrValidatorSuite) SetupSuite() {
}

func (s *attrValidatorSuite) TearDownSuite() {
}

func (s *attrValidatorSuite) SetupTest() {
	s.minRetentionDays = 1
	s.validator = newAttrValidator(cluster.TestActiveClusterMetadata, int32(s.minRetentionDays))
}

func (s *attrValidatorSuite) TearDownTest() {
}

func (s *attrValidatorSuite) TestValidateDomainConfig() {
	testCases := []struct {
		testDesc           string
		retention          int32
		historyArchival    types.ArchivalStatus
		historyURI         string
		visibilityArchival types.ArchivalStatus
		visibilityURI      string
		expectedErr        error
	}{
		{
			testDesc:           "valid retention and archival settings",
			retention:          int32(s.minRetentionDays + 1),
			historyArchival:    types.ArchivalStatusEnabled,
			historyURI:         "testScheme://history",
			visibilityArchival: types.ArchivalStatusEnabled,
			visibilityURI:      "testScheme://visibility",
			expectedErr:        nil,
		},
		{
			testDesc:    "invalid retention period",
			retention:   int32(s.minRetentionDays - 1),
			expectedErr: errInvalidRetentionPeriod,
		},
		{
			testDesc:        "enabled history archival without URI",
			retention:       int32(s.minRetentionDays + 1),
			historyArchival: types.ArchivalStatusEnabled,
			expectedErr:     errInvalidArchivalConfig,
		},
		{
			testDesc:           "enabled visibility archival without URI",
			retention:          int32(s.minRetentionDays + 1),
			visibilityArchival: types.ArchivalStatusEnabled,
			expectedErr:        errInvalidArchivalConfig,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.testDesc, func() {
			err := s.validator.validateDomainConfig(&persistence.DomainConfig{
				Retention:                tc.retention,
				HistoryArchivalStatus:    tc.historyArchival,
				HistoryArchivalURI:       tc.historyURI,
				VisibilityArchivalStatus: tc.visibilityArchival,
				VisibilityArchivalURI:    tc.visibilityURI,
			})
			if tc.expectedErr != nil {
				s.ErrorIs(err, tc.expectedErr)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *attrValidatorSuite) TestValidateConfigRetentionPeriod() {
	testCases := []struct {
		retentionPeriod int32
		expectedErr     error
	}{
		{
			retentionPeriod: 10,
			expectedErr:     nil,
		},
		{
			retentionPeriod: 0,
			expectedErr:     errInvalidRetentionPeriod,
		},
		{
			retentionPeriod: -3,
			expectedErr:     errInvalidRetentionPeriod,
		},
	}
	for _, tc := range testCases {
		actualErr := s.validator.validateDomainConfig(
			&persistence.DomainConfig{Retention: tc.retentionPeriod},
		)
		s.Equal(tc.expectedErr, actualErr)
	}
}

func (s *attrValidatorSuite) TestClusterName() {
	err := s.validator.validateClusterName("some random foo bar")
	s.IsType(&types.BadRequestError{}, err)

	err = s.validator.validateClusterName(cluster.TestCurrentClusterName)
	s.NoError(err)

	err = s.validator.validateClusterName(cluster.TestAlternativeClusterName)
	s.NoError(err)
}

func (s *attrValidatorSuite) TestValidateDomainReplicationConfigForLocalDomain() {
	err := s.validator.validateDomainReplicationConfigForLocalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	)
	s.IsType(&types.BadRequestError{}, err)

	err = s.validator.validateDomainReplicationConfigForLocalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	)
	s.IsType(&types.BadRequestError{}, err)

	err = s.validator.validateDomainReplicationConfigForLocalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	)
	s.IsType(&types.BadRequestError{}, err)

	err = s.validator.validateDomainReplicationConfigForLocalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
	)
	s.NoError(err)
}

func (s *attrValidatorSuite) TestValidateDomainReplicationConfigForGlobalDomain() {
	err := s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
	)
	s.NoError(err)

	err = s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	)
	s.NoError(err)

	err = s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestAlternativeClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	)
	s.NoError(err)

	err = s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
	)
	s.NoError(err)
}

func (s *attrValidatorSuite) TestValidateDomainReplicationConfigClustersDoesNotRemove() {
	err := s.validator.validateDomainReplicationConfigClustersDoesNotRemove(
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestCurrentClusterName},
			{ClusterName: cluster.TestAlternativeClusterName},
		},
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestCurrentClusterName},
			{ClusterName: cluster.TestAlternativeClusterName},
		},
	)
	s.NoError(err)

	err = s.validator.validateDomainReplicationConfigClustersDoesNotRemove(
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestCurrentClusterName},
		},
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestCurrentClusterName},
			{ClusterName: cluster.TestAlternativeClusterName},
		},
	)
	s.NoError(err)

	err = s.validator.validateDomainReplicationConfigClustersDoesNotRemove(
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestCurrentClusterName},
			{ClusterName: cluster.TestAlternativeClusterName},
		},
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestAlternativeClusterName},
		},
	)
	s.IsType(&types.BadRequestError{}, err)

	err = s.validator.validateDomainReplicationConfigClustersDoesNotRemove(
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestCurrentClusterName},
		},
		[]*persistence.ClusterReplicationConfig{
			{ClusterName: cluster.TestAlternativeClusterName},
		},
	)
	s.IsType(&types.BadRequestError{}, err)
}

func TestCheckActiveClusterRegionMappings(t *testing.T) {
	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			ClusterGroup: map[string]config.ClusterInformation{
				"A1": {
					Region: "A",
				},
				"A2": {
					Region: "A",
				},
				"B1": {
					Region: "B",
				},
				"B2": {
					Region: "B",
				},
				"C1": {
					Region: "C",
				},
				"C2": {
					Region: "C",
				},
			},
		},
		func(d string) bool { return false },
		metrics.NewNoopMetricsClient(),
		log.NewNoop(),
	)

	tests := []struct {
		desc           string
		activeClusters *types.ActiveClusters
		wantErr        bool
	}{
		{
			desc: "non-existing cluster",
			activeClusters: &types.ActiveClusters{ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
				"D": {ActiveClusterName: "D1"},
			}},
			wantErr: true,
		},
		{
			desc: "no cycle. every region is mapped to a local cluster",
			activeClusters: &types.ActiveClusters{ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
				"A": {ActiveClusterName: "A1"},
				"B": {ActiveClusterName: "B1"},
				"C": {ActiveClusterName: "C1"},
			}},
			wantErr: false,
		},
		{
			desc: "no cycle. A and C failed over to B",
			activeClusters: &types.ActiveClusters{ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
				"A": {ActiveClusterName: "B1"},
				"B": {ActiveClusterName: "B1"},
				"C": {ActiveClusterName: "B1"},
			}},
			wantErr: false,
		},
		{
			desc: "cycle. A -> B -> C -> A",
			activeClusters: &types.ActiveClusters{ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
				"A": {ActiveClusterName: "B2"},
				"B": {ActiveClusterName: "C2"},
				"C": {ActiveClusterName: "A2"},
			}},
			wantErr: true,
		},
		{
			desc: "no cycle but more than one hop. A -> B -> C",
			activeClusters: &types.ActiveClusters{ActiveClustersByRegion: map[string]types.ActiveClusterInfo{
				"A": {ActiveClusterName: "B2"},
				"B": {ActiveClusterName: "C2"},
				"C": {ActiveClusterName: "C1"},
			}},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		validator := newAttrValidator(clusterMetadata, int32(1))
		err := validator.checkActiveClusterRegionMappings(tc.activeClusters)
		assert.Equal(t, tc.wantErr, err != nil)
	}

}
