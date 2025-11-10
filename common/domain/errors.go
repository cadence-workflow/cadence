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

import "github.com/uber/cadence/common/types"

var (
	// err indicating that this cluster is not the primary, so cannot do domain registration or update
	errNotPrimaryCluster                   = &types.BadRequestError{Message: "Cluster is not primary cluster, cannot do domain registration or domain update."}
	errCannotRemoveClustersFromDomain      = &types.BadRequestError{Message: "Cannot remove existing replicated clusters from a domain."}
	errActiveClusterNotInClusters          = &types.BadRequestError{Message: "Active cluster is not contained in all clusters."}
	errCannotDoDomainFailoverAndUpdate     = &types.BadRequestError{Message: "Cannot set active cluster to current cluster when other parameters are set."}
	errCannotDoGracefulFailoverFromCluster = &types.BadRequestError{Message: "Cannot start the graceful failover from a to-be-passive cluster."}
	errGracefulFailoverInActiveCluster     = &types.BadRequestError{Message: "Cannot start the graceful failover from an active cluster to an active cluster."}
	errOngoingGracefulFailover             = &types.BadRequestError{Message: "Cannot start concurrent graceful failover."}
	errInvalidGracefulFailover             = &types.BadRequestError{Message: "Cannot start graceful failover without updating active cluster or in local domain."}
	errActiveClusterNameRequired           = &types.BadRequestError{Message: "ActiveClusterName is required for all global domains."}
	errLocalDomainsCannotFailover          = &types.BadRequestError{Message: "Local domains cannot perform failovers or change replication configuration"}
	errDomainDeprecated                    = &types.BadRequestError{Message: "Domain is deprecated."}
	errInvalidRetentionPeriod              = &types.BadRequestError{Message: "A valid retention period is not set on request."}
	errInvalidArchivalConfig               = &types.BadRequestError{Message: "Invalid to enable archival without specifying a uri."}
)
