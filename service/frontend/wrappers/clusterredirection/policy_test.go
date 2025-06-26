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

package clusterredirection

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common/activecluster"
	"github.com/uber/cadence/common/cache"
	"github.com/uber/cadence/common/cluster"
	"github.com/uber/cadence/common/dynamicconfig"
	"github.com/uber/cadence/common/dynamicconfig/dynamicproperties"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	frontendcfg "github.com/uber/cadence/service/frontend/config"
)

type (
	noopDCRedirectionPolicySuite struct {
		suite.Suite
		*require.Assertions

		currentClusterName string
		policy             *noopRedirectionPolicy
	}

	selectedAPIsForwardingRedirectionPolicySuite struct {
		suite.Suite
		*require.Assertions

		controller           *gomock.Controller
		activeClusterManager *activecluster.MockManager

		domainName             string
		domainID               string
		currentClusterName     string
		alternativeClusterName string
		mockConfig             *frontendcfg.Config

		policy *selectedOrAllAPIsForwardingRedirectionPolicy
	}
)

func TestNoopDCRedirectionPolicySuite(t *testing.T) {
	s := new(noopDCRedirectionPolicySuite)
	suite.Run(t, s)
}

func (s *noopDCRedirectionPolicySuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.currentClusterName = cluster.TestCurrentClusterName
	s.policy = newNoopRedirectionPolicy(s.currentClusterName)
}

func (s *noopDCRedirectionPolicySuite) TearDownTest() {

}

func (s *noopDCRedirectionPolicySuite) TestWithDomainRedirect() {
	domainName := "some random domain name"
	domainID := "some random domain ID"
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: domainName},
		&persistence.DomainConfig{},
		cluster.TestCurrentClusterName,
	)

	apiName := "any random API name"
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
	s.Nil(err)

	err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
	s.Nil(err)

	s.Equal(2, callCount)
}

func (s *noopDCRedirectionPolicySuite) TestWithDomainRedirectForAllowedAPIs() {
	domainName := "some random domain name"
	domainID := "some random domain ID"
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: domainID, Name: domainName},
		&persistence.DomainConfig{},
		cluster.TestCurrentClusterName,
	)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	// Test all allowed APIs for deprecated domains
	allowedAPIs := []string{
		"ListWorkflowExecutions",
		"CountWorkflowExecutions",
		"ScanWorkflowExecutions",
		"TerminateWorkflowExecution",
	}

	for _, apiName := range allowedAPIs {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)
	}

	// Verify that each API was tested for both domain ID and domain name redirects
	s.Equal(2*len(allowedAPIs), callCount)
}

func TestSelectedAPIsForwardingRedirectionPolicySuite(t *testing.T) {
	s := new(selectedAPIsForwardingRedirectionPolicySuite)
	suite.Run(t, s)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) SetupSuite() {
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TearDownSuite() {

}

func (s *selectedAPIsForwardingRedirectionPolicySuite) SetupTest() {
	s.Assertions = require.New(s.T())

	s.controller = gomock.NewController(s.T())

	s.domainName = "some random domain name"
	s.domainID = "some random domain ID"
	s.currentClusterName = cluster.TestCurrentClusterName
	s.alternativeClusterName = cluster.TestAlternativeClusterName
	logger := testlogger.New(s.T())

	s.mockConfig = frontendcfg.NewConfig(dynamicconfig.NewCollection(
		dynamicconfig.NewNopClient(),
		logger,
	),
		0,
		false,
		"hostname",
		logger,
	)

	s.activeClusterManager = activecluster.NewMockManager(s.controller)

	s.policy = newSelectedOrAllAPIsForwardingPolicy(
		s.currentClusterName,
		s.mockConfig,
		false,
		selectedAPIsForwardingRedirectionPolicyAPIAllowlist,
		"",
		logger,
		s.activeClusterManager,
	)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TearDownTest() {
	s.controller.Finish()
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_LocalDomain() {
	domainEntry := s.setupLocalDomain()

	apiName := "any random API name"
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
	s.Nil(err)

	err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
	s.Nil(err)

	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)
	}

	s.Equal(2*(len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist)+1), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_GlobalDomain_NoForwarding_DomainNotWhitelisted() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(false, true)

	domainNotActiveErr := &types.DomainNotActiveError{
		CurrentCluster: s.currentClusterName,
		ActiveCluster:  s.alternativeClusterName,
	}
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return domainNotActiveErr
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.NotNil(err)
		s.Equal(err.Error(), domainNotActiveErr.Error())

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.NotNil(err)
		s.Equal(err.Error(), domainNotActiveErr.Error())

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelStrong, callFn)
		s.NotNil(err)
		s.Equal(err.Error(), domainNotActiveErr.Error())
	}

	s.Equal(3*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestWithDomainRedirect_GlobalDomain_Forwarding_APINotWhitelisted() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(true, true)

	apiName := "any random API name"
	domainNotActiveErr := &types.DomainNotActiveError{
		CurrentCluster: s.currentClusterName,
		ActiveCluster:  s.alternativeClusterName,
	}
	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return domainNotActiveErr
	}

	err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
	s.NotNil(err)
	s.Equal(err.Error(), domainNotActiveErr.Error())

	err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
	s.NotNil(err)
	s.Equal(err.Error(), domainNotActiveErr.Error())

	s.Equal(2, callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_CurrentCluster() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(true, true)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.currentClusterName, targetCluster)
		return nil
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_AlternativeCluster() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(true, false)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.alternativeClusterName, targetCluster)
		return nil
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_AlternativeCluster_StrongConsistency() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(true, false)

	callCount := 0
	callFn := func(targetCluster string) error {
		callCount++
		s.Equal(s.alternativeClusterName, targetCluster)
		return nil
	}

	apiName := "any random API name"
	err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelStrong, callFn)
	s.Nil(err)

	err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelStrong, callFn)
	s.Nil(err)

	s.Equal(2, callCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_CurrentClusterToAlternativeCluster() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(true, true)

	currentClustercallCount := 0
	alternativeClustercallCount := 0
	callFn := func(targetCluster string) error {
		switch targetCluster {
		case s.currentClusterName:
			currentClustercallCount++
			return &types.DomainNotActiveError{
				CurrentCluster: s.currentClusterName,
				ActiveCluster:  s.alternativeClusterName,
			}
		case s.alternativeClusterName:
			alternativeClustercallCount++
			return nil
		default:
			panic(fmt.Sprintf("unknown cluster name %v", targetCluster))
		}
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), currentClustercallCount)
	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), alternativeClustercallCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_AlternativeClusterToCurrentCluster() {
	domainEntry := s.setupGlobalDomainWithTwoReplicationCluster(true, false)

	currentClustercallCount := 0
	alternativeClustercallCount := 0
	callFn := func(targetCluster string) error {
		switch targetCluster {
		case s.currentClusterName:
			currentClustercallCount++
			return nil
		case s.alternativeClusterName:
			alternativeClustercallCount++
			return &types.DomainNotActiveError{
				CurrentCluster: s.alternativeClusterName,
				ActiveCluster:  s.currentClusterName,
			}
		default:
			panic(fmt.Sprintf("unknown cluster name %v", targetCluster))
		}
	}

	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Nil(err)
	}

	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), currentClustercallCount)
	s.Equal(2*len(selectedAPIsForwardingRedirectionPolicyAPIAllowlist), alternativeClustercallCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) TestGetTargetDataCenter_GlobalDomain_Forwarding_DeprecatedDomain() {
	domainEntry := s.setupGlobalDeprecatedDomainWithTwoReplicationCluster(true, false)

	currentClustercallCount := 0
	alternativeClustercallCount := 0
	callFn := func(targetCluster string) error {
		switch targetCluster {
		case s.currentClusterName:
			currentClustercallCount++
			return nil
		case s.alternativeClusterName:
			alternativeClustercallCount++
			return &types.DomainNotActiveError{
				CurrentCluster: s.alternativeClusterName,
				ActiveCluster:  s.currentClusterName,
			}
		default:
			panic(fmt.Sprintf("unknown cluster name %v", targetCluster))
		}
	}

	// Test non-allowed APIs
	for apiName := range selectedAPIsForwardingRedirectionPolicyAPIAllowlist {
		if _, ok := allowedAPIsForDeprecatedDomains[apiName]; ok {
			continue // Skip allowed APIs
		}

		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Error(err)
		s.Equal("domain is deprecated or deleted.", err.Error())

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.Error(err)
		s.Equal("domain is deprecated or deleted.", err.Error())
	}
	s.Equal(0, currentClustercallCount)
	s.Equal(0, alternativeClustercallCount)

	// Test allowed APIs
	for apiName := range allowedAPIsForDeprecatedDomains {
		err := s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.NoError(err)

		err = s.policy.Redirect(context.Background(), domainEntry, nil, nil, apiName, types.QueryConsistencyLevelEventual, callFn)
		s.NoError(err)
	}

	// Verify that allowed APIs were called on the current cluster
	s.Equal(2*len(allowedAPIsForDeprecatedDomains), currentClustercallCount)
	s.Equal(2, alternativeClustercallCount)
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) setupLocalDomain() *cache.DomainCacheEntry {
	domainEntry := cache.NewLocalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: s.domainID, Name: s.domainName},
		&persistence.DomainConfig{Retention: 1},
		cluster.TestCurrentClusterName,
	)

	return domainEntry
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) setupGlobalDomainWithTwoReplicationCluster(forwardingEnabled bool, isRecordActive bool) *cache.DomainCacheEntry {
	activeCluster := s.alternativeClusterName
	if isRecordActive {
		activeCluster = s.currentClusterName
	}
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: s.domainID, Name: s.domainName},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: activeCluster,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234, // not used
	)

	s.mockConfig.EnableDomainNotActiveAutoForwarding = dynamicproperties.GetBoolPropertyFnFilteredByDomain(forwardingEnabled)
	return domainEntry
}

func (s *selectedAPIsForwardingRedirectionPolicySuite) setupGlobalDeprecatedDomainWithTwoReplicationCluster(forwardingEnabled bool, isRecordActive bool) *cache.DomainCacheEntry {
	activeCluster := s.alternativeClusterName
	if isRecordActive {
		activeCluster = s.currentClusterName
	}
	domainEntry := cache.NewGlobalDomainCacheEntryForTest(
		&persistence.DomainInfo{ID: s.domainID, Name: s.domainName, Status: persistence.DomainStatusDeprecated},
		&persistence.DomainConfig{Retention: 1},
		&persistence.DomainReplicationConfig{
			ActiveClusterName: activeCluster,
			Clusters: []*persistence.ClusterReplicationConfig{
				{ClusterName: cluster.TestCurrentClusterName},
				{ClusterName: cluster.TestAlternativeClusterName},
			},
		},
		1234, // not used
	)

	s.mockConfig.EnableDomainNotActiveAutoForwarding = dynamicproperties.GetBoolPropertyFnFilteredByDomain(forwardingEnabled)
	return domainEntry
}
