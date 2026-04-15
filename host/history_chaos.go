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

package host

import "github.com/uber/cadence/common/log"

// historyCadenceController adapts the Cadence interface to HostController for
// use with ChaosMonkey.
type historyCadenceController struct {
	cadence Cadence
}

func (h *historyCadenceController) StopHost(index int) {
	h.cadence.StopHistoryHost(index)
}

func (h *historyCadenceController) StartHost(index int) {
	h.cadence.StartHistoryHost(index)
}

func (h *historyCadenceController) HostIdentity(index int) string {
	return h.cadence.HistoryHostIdentity(index)
}

// HistoryChaosMonkey is ChaosMonkey specialised for history hosts. It adapts
// the Cadence interface to HostController via historyCadenceController.
type HistoryChaosMonkey struct {
	*ChaosMonkey
}

// NewHistoryChaosMonkey creates a HistoryChaosMonkey. It converts
// HistoryChaosConfig to ChaosConfig; StopChance and StartChance default to 0.5.
func NewHistoryChaosMonkey(cadence Cadence, cfg HistoryChaosConfig, numHosts int, logger log.Logger) *HistoryChaosMonkey {
	controller := &historyCadenceController{cadence: cadence}
	chaosCfg := ChaosConfig{
		Enable:     cfg.Enable,
		IntervalMs: cfg.IntervalMs,
		JitterMs:   cfg.JitterMs,
		MinHosts:   cfg.MinHistoryHosts,
		// StopChance and StartChance default to 0.5 via NewChaosMonkey
	}
	return &HistoryChaosMonkey{
		ChaosMonkey: NewChaosMonkey(controller, chaosCfg, numHosts, logger),
	}
}
