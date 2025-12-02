package rpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/yarpc/api/peer"

	"github.com/uber/cadence/common/log"
)

type mockDNSHostResolver struct {
	addrsToReturn []string
	errToReturn   error
}

func (m *mockDNSHostResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return m.addrsToReturn, m.errToReturn
}

type dummyPeerList struct {
	updates     []peer.ListUpdates
	errToReturn error
}

func (d *dummyPeerList) Update(updates peer.ListUpdates) error {
	d.updates = append(d.updates, updates)
	return d.errToReturn
}

func TestDNSUpdater_Refresh(t *testing.T) {
	tests := []struct {
		name           string
		initialPeers   []string
		newPeers       []string
		currentPeers   map[string]struct{}
		wantChanged    bool
		wantNewPeers   map[string]struct{}
		wantAdditions  []string
		wantRemovals   []string
		wantDNSFailure bool
	}{
		{
			name:         "Initial state: two peers",
			initialPeers: nil,
			newPeers:     []string{"10.0.0.1", "10.0.0.2"},
			currentPeers: nil,
			wantChanged:  true,
			wantNewPeers: map[string]struct{}{
				"10.0.0.1:1234": {},
				"10.0.0.2:1234": {},
			},
			wantAdditions: []string{"10.0.0.1:1234", "10.0.0.2:1234"},
			wantRemovals:  nil,
		},
		{
			name:         "Change DNS, one removed, one added, one persistent",
			initialPeers: []string{"10.0.0.1", "10.0.0.2"},
			newPeers:     []string{"10.0.0.2", "10.0.0.3"},
			currentPeers: map[string]struct{}{
				"10.0.0.1:1234": {},
				"10.0.0.2:1234": {},
			},
			wantChanged: true,
			wantNewPeers: map[string]struct{}{
				"10.0.0.2:1234": {},
				"10.0.0.3:1234": {},
			},
			wantAdditions: []string{"10.0.0.3:1234"},
			wantRemovals:  []string{"10.0.0.1:1234"},
		},
		{
			name:         "No changes",
			initialPeers: []string{"10.0.0.2", "10.0.0.3"},
			newPeers:     []string{"10.0.0.2", "10.0.0.3"},
			currentPeers: map[string]struct{}{
				"10.0.0.2:1234": {},
				"10.0.0.3:1234": {},
			},
			wantChanged: false,
			wantNewPeers: map[string]struct{}{
				"10.0.0.2:1234": {},
				"10.0.0.3:1234": {},
			},
			wantAdditions: nil,
			wantRemovals:  nil,
		},
		{
			name:           "DNS failure",
			initialPeers:   []string{"10.0.0.1", "10.0.0.2"},
			newPeers:       nil,
			currentPeers:   map[string]struct{}{"10.0.0.1:1234": {}, "10.0.0.2:1234": {}},
			wantChanged:    false,
			wantNewPeers:   nil,
			wantAdditions:  nil,
			wantRemovals:   nil,
			wantDNSFailure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockResolver := &mockDNSHostResolver{}
			dnsAddr := "test.service.com:1234"
			interval := 1 * time.Second
			logger := log.NewNoop()
			testList := &dummyPeerList{}

			updater, err := newDNSUpdater(testList, dnsAddr, interval, logger)
			require.NoError(t, err, "should create dnsUpdater")
			updater.resolver = mockResolver

			// Optionally set initial currentPeers
			if tt.currentPeers != nil {
				updater.currentPeers = tt.currentPeers
			}

			if tt.wantDNSFailure {
				mockResolver.errToReturn = errors.New("mock DNS error")
			} else {
				mockResolver.errToReturn = nil
				mockResolver.addrsToReturn = tt.newPeers
			}

			res, err := updater.refresh()

			if tt.wantDNSFailure {
				require.Error(t, err)
				assert.Nil(t, res)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantChanged, res.changed)
				assert.Equal(t, tt.wantNewPeers, res.newPeers)
				actualAdditions := identifiersToStringList(res.updates.Additions)
				actualRemovals := identifiersToStringList(res.updates.Removals)
				assert.ElementsMatch(t, tt.wantAdditions, actualAdditions)
				assert.ElementsMatch(t, tt.wantRemovals, actualRemovals)
			}
		})
	}
}

func TestDNSUpdater_UpdaterFailure(t *testing.T) {
	mockResolver := &mockDNSHostResolver{
		addrsToReturn: []string{"10.0.0.3", "10.0.0.4"},
	}
	logger := log.NewNoop()
	testList := &dummyPeerList{errToReturn: errors.New("mock updater error")}

	updater, err := newDNSUpdater(testList, "host.test:1234", 100*time.Millisecond, logger)
	require.NoError(t, err)
	updater.resolver = mockResolver
	updater.currentPeers = map[string]struct{}{
		"10.0.0.1:1234": {},
		"10.0.0.2:1234": {},
	}

	updater.Start()
	defer updater.Stop()

	time.Sleep(1 * time.Second)
	assert.Equal(t, map[string]struct{}{"10.0.0.1:1234": {}, "10.0.0.2:1234": {}}, updater.currentPeers)
}
