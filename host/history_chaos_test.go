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

import (
	"testing"

	"go.uber.org/mock/gomock"
)

// TestHistoryCadenceController_StopHost verifies the adapter delegates StopHost
// to Cadence.StopHistoryHost.
func TestHistoryCadenceController_StopHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := NewMockCadence(ctrl)
	mc.EXPECT().StopHistoryHost(2).Times(1)

	c := &historyCadenceController{cadence: mc}
	c.StopHost(2)
}

// TestHistoryCadenceController_StartHost verifies the adapter delegates
// StartHost to Cadence.StartHistoryHost.
func TestHistoryCadenceController_StartHost(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := NewMockCadence(ctrl)
	mc.EXPECT().StartHistoryHost(2).Times(1)

	c := &historyCadenceController{cadence: mc}
	c.StartHost(2)
}

// TestHistoryCadenceController_HostIdentity verifies the adapter delegates
// HostIdentity to Cadence.HistoryHostIdentity.
func TestHistoryCadenceController_HostIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mc := NewMockCadence(ctrl)
	mc.EXPECT().HistoryHostIdentity(2).Return("127.0.0.1_7201").Times(1)

	c := &historyCadenceController{cadence: mc}
	got := c.HostIdentity(2)
	if got != "127.0.0.1_7201" {
		t.Errorf("HostIdentity(2) = %q, want %q", got, "127.0.0.1_7201")
	}
}
