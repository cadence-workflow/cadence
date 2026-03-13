package leaderstore

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx/fxtest"
	"gopkg.in/yaml.v2"

	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
	"github.com/uber/cadence/testflags"
)

// TestCreateElection tests that an election can be created successfully
func TestCreateElection(t *testing.T) {
	tc := setupETCDCluster(t)

	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	namespace := "test-namespace"
	elect, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	require.NotNil(t, elect)

	// Clean up
	err = elect.Cleanup(ctx)
	require.NoError(t, err)
}

// TestCampaign tests that a node can campaign for leadership
func TestCampaign(t *testing.T) {
	tc := setupETCDCluster(t)

	// Use a separate context for test operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-namespace-campaign"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	// Start campaigning for leadership
	host := "test-host-1"
	err = election.Campaign(ctx, host)
	require.NoError(t, err)

	// Verify leadership was obtained by checking the key in etcd
	// First create a client to connect to etcd
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   tc.endpoints,
		DialTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer client.Close()

	// Get the key and verify it exists
	key := fmt.Sprintf("%s/%s/leader", tc.storeConfig.Prefix, namespace)
	resp, err := client.Get(ctx, key)
	require.NoError(t, err)
	require.NotEqual(t, 0, resp.Count, "Leader key should exist")
}

// TestResign tests resigning leadership
func TestResign(t *testing.T) {
	tc := setupETCDCluster(t)

	// Use a separate context for test operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-namespace-resign"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election.Cleanup(ctx)

	// Start campaigning for leadership
	host := "test-host-1"
	err = election.Campaign(ctx, host)
	require.NoError(t, err)

	// Resign the leadership
	err = election.Resign(ctx)
	require.NoError(t, err)

	// Verify leadership was resigned by checking that someone else can become leader
	election2, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election2.Cleanup(ctx)

	err = election2.Campaign(ctx, "host-2")
	require.NoError(t, err, "Second host should be able to become leader after first resigned")
}

// TestMultipleNodes tests multiple nodes competing for leadership
func TestMultipleNodes(t *testing.T) {
	tc := setupETCDCluster(t)

	// Use a separate context for test operations
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	namespace := "test-namespace-multiple"

	// Create first election
	election1, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election1.Cleanup(ctx)

	// Create second election
	election2, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election2.Cleanup(ctx)

	// First node campaigns
	err = election1.Campaign(ctx, "host1")
	require.NoError(t, err)

	// Second node campaigns - this should block as first node already has leadership
	// We'll use a channel to track when it's done and a shorter context to timeout
	campaignDone := make(chan struct{})
	campaignErr := make(chan error, 1)

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 1*time.Second)
	defer cancelTimeout()

	go func() {
		err := election2.Campaign(ctxTimeout, "host2")
		if err != nil {
			campaignErr <- err
		}
		close(campaignDone)
	}()

	// Verify second node is blocked (should timeout)
	select {
	case err := <-campaignErr:
		// Expected to get a timeout error
		require.Error(t, err, "Expected a timeout error for the second campaign")
		require.Contains(t, err.Error(), "context deadline exceeded", "Expected a context deadline error")
	case <-campaignDone:
		t.Error("Second node should not have been able to become leader while first node holds leadership")
	case <-time.After(2 * time.Second):
		t.Error("Expected the second campaign to timeout quickly")
	}

	// First node resigns
	err = election1.Resign(ctx)
	require.NoError(t, err)

	// Now the second node should be able to become leader
	// Create a new election for the second host
	election3, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)
	defer election3.Cleanup(ctx)

	err = election3.Campaign(ctx, "host3")
	require.NoError(t, err, "Third host should be able to become leader after first host resigned")
}

// TestSessionDone tests the Done channel behavior
func TestSessionDone(t *testing.T) {
	tc := setupETCDCluster(t)

	// Use a separate context for test operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namespace := "test-namespace-done"
	election, err := tc.store.CreateElection(ctx, namespace)
	require.NoError(t, err)

	doneCh := election.Done()
	require.NotNil(t, doneCh)

	// Clean up should close the session, which should close the Done channel
	err = election.Cleanup(ctx)
	require.NoError(t, err)

	// Verify the Done channel is closed
	select {
	case <-doneCh:
		// Expected - channel should be closed
	case <-time.After(2 * time.Second):
		t.Error("Done channel should be closed after Cleanup")
	}
}

// testCluster represents a test etcd cluster with its resources
type testCluster struct {
	store       store.Elector
	storeConfig etcdCfg
	endpoints   []string
}

// setupETCDCluster initializes an etcd cluster for testing
func setupETCDCluster(t *testing.T) *testCluster {
	t.Helper()

	flag.Parse()
	testflags.RequireEtcd(t)

	endpoints := strings.Split(os.Getenv("ETCD_ENDPOINTS"), ",")
	if endpoints == nil || len(endpoints) == 0 || endpoints[0] == "" {
		// default etcd port
		endpoints = []string{"localhost:2379"}
	}

	t.Logf("ETCD endpoints: %v", endpoints)

	testConfig := etcdCfg{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		Prefix:      fmt.Sprintf("/election/%s", t.Name()),
		ElectionTTL: 5 * time.Second,
	}

	// Create store
	storeParams := StoreParams{
		Cfg:       config.ShardDistribution{LeaderStore: config.Store{StorageParams: createConfig(t, testConfig)}},
		Lifecycle: fxtest.NewLifecycle(t),
	}

	store, err := NewLeaderStore(storeParams)
	require.NoError(t, err)

	return &testCluster{
		store:       store,
		storeConfig: testConfig,
		endpoints:   endpoints,
	}
}

func createConfig(t *testing.T, cfg etcdCfg) *config.YamlNode {
	t.Helper()

	yamlCfg, err := yaml.Marshal(cfg)
	require.NoError(t, err)

	var res *config.YamlNode

	err = yaml.Unmarshal(yamlCfg, &res)
	require.NoError(t, err)

	return res
}
