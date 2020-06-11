// The MIT License (MIT)
//
// Copyright (c) 2017-2020 Uber Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//go:generate mockgen -copyright_file ../../../LICENSE -package $GOPACKAGE -source $GOFILE -destination marker_notifier_mock.go -self_package github.com/uber/cadence/service/history/failover

package failover

import (
	"sync/atomic"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
	"github.com/uber/cadence/service/history/shard"
)

type (
	// MarkerNotifier notifies failover markers to the remote failover coordinator
	MarkerNotifier interface {
		common.Daemon
	}

	markerNotifierImpl struct {
		status              int32
		shutdownCh          chan struct{}
		shard               shard.Context
		failoverCoordinator Coordinator
		logger              log.Logger
	}
)

// NewMarkerNotifier creates a new instance of failover marker notifier
func NewMarkerNotifier(
	shard shard.Context,
	failoverCoordinator Coordinator,
) MarkerNotifier {

	return &markerNotifierImpl{
		status:              common.DaemonStatusInitialized,
		shutdownCh:          make(chan struct{}, 1),
		shard:               shard,
		failoverCoordinator: failoverCoordinator,
		logger:              shard.GetLogger().WithTags(tag.ComponentFailoverMarkerNotifier),
	}
}

func (m *markerNotifierImpl) Start() {

	if !atomic.CompareAndSwapInt32(
		&m.status,
		common.DaemonStatusInitialized,
		common.DaemonStatusStarted,
	) {
		return
	}

	go m.notifyPendingFailoverMarker()
	m.logger.Info("Failover marker notifier is started", tag.LifeCycleStarted)
}

func (m *markerNotifierImpl) Stop() {

	if !atomic.CompareAndSwapInt32(
		&m.status,
		common.DaemonStatusStarted,
		common.DaemonStatusStopped,
	) {
		return
	}
	close(m.shutdownCh)
	m.logger.Info("Failover marker notifier is stopped", tag.LifeCycleStopped)
}

func (m *markerNotifierImpl) notifyPendingFailoverMarker() {

	for {
		select {
		case <-m.shutdownCh:
			return
		default:
			markers, err := m.shard.ValidateAndUpdateFailoverMarkers()
			if err != nil {
				m.logger.Error("Failed to update pending failover markers in shard info.", tag.Error(err))
			}

			respCh := m.failoverCoordinator.NotifyFailoverMarkers(int32(m.shard.GetShardID()), markers)
			select {
			case <-m.shutdownCh:
				return
			case <-respCh:
				// continue the next round to notify failover markers
			}
		}
	}
}
