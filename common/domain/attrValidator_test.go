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

	// When ActiveClusterName is not provided, and ActiveClusters are not provided, it should return an error
	err = s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: "",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
			},
		},
	)
	s.Error(err)
	s.IsType(&types.BadRequestError{}, err)

	// When ActiveClusterName and ActiveClusters are provided, it should not return an error
	err = s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: cluster.TestCurrentClusterName,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							cluster.TestRegion1: {ActiveClusterName: cluster.TestCurrentClusterName},
							cluster.TestRegion2: {ActiveClusterName: cluster.TestAlternativeClusterName},
						},
					},
				},
			},
		},
	)
	s.NoError(err)

	// When ActiveClusterName is not provided, and ActiveClusters are provided, it should return an error
	err = s.validator.validateDomainReplicationConfigForGlobalDomain(
		&persistence.DomainReplicationConfig{
			ActiveClusterName: "",
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
			ActiveClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"region": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							cluster.TestRegion1: {ActiveClusterName: cluster.TestCurrentClusterName},
							cluster.TestRegion2: {ActiveClusterName: cluster.TestAlternativeClusterName},
						},
					},
				},
			},
		},
	)
	s.Error(err)
	s.IsType(&types.BadRequestError{}, err)
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

func TestValidateActiveActiveDomainReplicationConfig(t *testing.T) {
	// Define test cluster names
	const (
		clusterA = "cluster-a"
		clusterB = "cluster-b"
	)

	// Create explicit cluster metadata for the test
	clusterMetadata := cluster.NewMetadata(
		config.ClusterGroupMetadata{
			FailoverVersionIncrement: 10,
			PrimaryClusterName:       clusterA,
			CurrentClusterName:       clusterA,
			ClusterGroup: map[string]config.ClusterInformation{
				clusterA: {
					Enabled:                true,
					InitialFailoverVersion: 1,
				},
				clusterB: {
					Enabled:                true,
					InitialFailoverVersion: 2,
				},
			},
		},
		func(d string) bool { return false },
		metrics.NewNoopMetricsClient(),
		log.NewNoop(),
	)

	validator := newAttrValidator(clusterMetadata, 1)

	testCases := []struct {
		name           string
		activeClusters *types.ActiveClusters
		expectedErr    bool
		errType        interface{}
	}{
		{
			name: "invalid cluster in AttributeScopes",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"city": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"seattle": {
								ActiveClusterName: "invalid-cluster",
								FailoverVersion:   100,
							},
						},
					},
				},
			},
			expectedErr: true,
			errType:     &types.BadRequestError{},
		},
		{
			name: "empty ActiveClusters - all maps nil",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: nil,
			},
			expectedErr: false,
		},
		{
			name: "empty ActiveClusters - maps initialized but empty",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{},
			},
			expectedErr: false,
		},
		{
			name: "an invalid failover version should return an error",
			activeClusters: &types.ActiveClusters{
				AttributeScopes: map[string]types.ClusterAttributeScope{
					"city": {
						ClusterAttributes: map[string]types.ActiveClusterInfo{
							"seattle": {
								ActiveClusterName: clusterA,
								FailoverVersion:   -1,
							},
						},
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.validateActiveActiveDomainReplicationConfig(tc.activeClusters)
			if tc.expectedErr {
				assert.Error(t, err)
				if tc.errType != nil {
					assert.IsType(t, tc.errType, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
