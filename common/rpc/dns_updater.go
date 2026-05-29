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

package rpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/yarpc/api/peer"

	"github.com/uber/cadence/common/log"
	"github.com/uber/cadence/common/log/tag"
)

type (
	dnsUpdater struct {
		resolver     dnsHostResolver
		interval     time.Duration
		dnsAddress   string
		port         string
		currentPeers map[string]struct{}
		list         peer.List
		logger       log.Logger
		wg           sync.WaitGroup
		ctx          context.Context
		cancel       context.CancelFunc
	}
	dnsRefreshResult struct {
		updates  peer.ListUpdates
		newPeers map[string]struct{}
		changed  bool
	}
	aPeer struct {
		addrPort string
	}
)

type dnsHostResolver interface {
	LookupHost(ctx context.Context, host string) (addrs []string, err error)
}

func newDNSUpdater(list peer.List, dnsPort string, interval time.Duration, logger log.Logger) (*dnsUpdater, error) {
	ss := strings.Split(dnsPort, ":")
	if len(ss) != 2 {
		return nil, fmt.Errorf("incorrect DNS:Port format")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &dnsUpdater{
		resolver:     net.DefaultResolver,
		interval:     interval,
		logger:       logger,
		list:         list,
		dnsAddress:   ss[0],
		port:         ss[1],
		currentPeers: make(map[string]struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

func (d *dnsUpdater) Start() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			now := time.Now()
			res, err := d.refresh()
			if err != nil {
				d.logger.Error("Failed to update DNS", tag.Error(err), tag.Address(d.dnsAddress))
			}
			if res != nil && res.changed {
				if len(res.updates.Additions) > 0 {
					d.logger.Info("Add new peers by DNS lookup", tag.Address(d.dnsAddress), tag.Addresses(identifiersToStringList(res.updates.Additions)))
				}
				if len(res.updates.Removals) > 0 {
					d.logger.Info("Remove stale peers by DNS lookup", tag.Address(d.dnsAddress), tag.Addresses(identifiersToStringList(res.updates.Removals)))
				}

				err := d.list.Update(res.updates)
				if err != nil {
					d.logger.Error("Failed to update peerList", tag.Error(err), tag.Address(d.dnsAddress))
				} else {
					d.currentPeers = res.newPeers
				}
			}
			sleepDu := now.Add(d.interval).Sub(now)
			t := time.NewTimer(sleepDu)
			select {
			case <-d.ctx.Done():
				t.Stop()
				d.logger.Info("DNS updater is stopping so returning from dns update loop", tag.Address(d.dnsAddress))
				return
			case <-t.C:
				continue
			}
		}
	}()
}

func (d *dnsUpdater) Stop() {
	d.logger.Info("DNS updater is stopping", tag.Address(d.dnsAddress))
	d.cancel()
	d.wg.Wait()
	d.logger.Info("DNS updater stopped", tag.Address(d.dnsAddress))
}

func (d *dnsUpdater) refresh() (*dnsRefreshResult, error) {
	ips, err := d.resolver.LookupHost(d.ctx, d.dnsAddress)
	if err != nil {
		return nil, err
	}
	newPeers := map[string]struct{}{}
	for _, ip := range ips {
		adr := fmt.Sprintf("%v:%v", ip, d.port)
		newPeers[adr] = struct{}{}
	}

	updates := peer.ListUpdates{
		Additions: make([]peer.Identifier, 0),
		Removals:  make([]peer.Identifier, 0),
	}
	changed := false
	// remove if it doesn't exist anymore
	for addr := range d.currentPeers {
		if _, ok := newPeers[addr]; !ok {
			changed = true
			updates.Removals = append(
				updates.Removals,
				aPeer{addrPort: addr},
			)
		}
	}

	// add if it doesn't exist before
	for addr := range newPeers {
		if _, ok := d.currentPeers[addr]; !ok {
			changed = true
			updates.Additions = append(
				updates.Additions,
				aPeer{addrPort: addr},
			)
		}
	}

	return &dnsRefreshResult{
		updates:  updates,
		newPeers: newPeers,
		changed:  changed,
	}, nil
}

func (a aPeer) Identifier() string {
	return a.addrPort
}

func identifiersToStringList(ids []peer.Identifier) []string {
	ss := make([]string, 0, len(ids))
	for _, id := range ids {
		ss = append(ss, id.Identifier())
	}
	return ss
}
