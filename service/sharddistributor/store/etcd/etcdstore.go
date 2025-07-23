package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/fx"

	"github.com/uber/cadence/service/sharddistributor/config"
	"github.com/uber/cadence/service/sharddistributor/store"
)

const (
	heartbeatKey      = "heartbeat"
	stateKey          = "state"
	reportedShardsKey = "reported_shards"
	assignedShardsKey = "assigned_shards"
)

// Store implements the generic store.Store interface using etcd as the backend.
type Store struct {
	client *clientv3.Client
	prefix string
}

// StoreParams defines the dependencies for the etcd store, for use with fx.
type StoreParams struct {
	fx.In

	Client    *clientv3.Client `optional:"true"`
	Cfg       config.LeaderElection
	Lifecycle fx.Lifecycle
}

// NewStore creates a new etcd-backed store and provides it to the fx application.
func NewStore(p StoreParams) (store.Store, error) {
	if !p.Cfg.Enabled {
		return nil, nil
	}

	var err error
	var etcdCfg struct {
		Endpoints   []string      `yaml:"endpoints"`
		DialTimeout time.Duration `yaml:"dialTimeout"`
		Prefix      string        `yaml:"prefix"`
	}

	if err := p.Cfg.Store.StorageParams.Decode(&etcdCfg); err != nil {
		return nil, fmt.Errorf("bad config for etcd store: %w", err)
	}

	etcdClient := p.Client
	if etcdClient == nil {
		etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   etcdCfg.Endpoints,
			DialTimeout: etcdCfg.DialTimeout,
		})
		if err != nil {
			return nil, err
		}
	}

	p.Lifecycle.Append(fx.StopHook(etcdClient.Close))

	return &Store{
		client: etcdClient,
		prefix: etcdCfg.Prefix,
	}, nil
}

// --- HeartbeatStore Implementation ---

func (s *Store) RecordHeartbeat(ctx context.Context, namespace string, request store.HeartbeatState) error {
	heartbeatKey := s.buildExecutorKey(namespace, request.ExecutorID, heartbeatKey)
	stateKey := s.buildExecutorKey(namespace, request.ExecutorID, stateKey)
	reportedShardsKey := s.buildExecutorKey(namespace, request.ExecutorID, reportedShardsKey)

	reportedShardsData, err := json.Marshal(request.ReportedShards)
	if err != nil {
		return fmt.Errorf("marshal assinged shards: %w", err)
	}

	// Atomically update both the timestamp and the state.
	_, err = s.client.Txn(ctx).Then(
		clientv3.OpPut(heartbeatKey, fmt.Sprintf("%d", time.Now().Unix())),
		clientv3.OpPut(stateKey, string(request.State)),
		clientv3.OpPut(reportedShardsKey, string(reportedShardsData)),
	).Commit()

	if err != nil {
		return fmt.Errorf("record heartbeat: %w", err)
	}
	return nil
}

// GetHeartbeat retrieves the last known heartbeat state for a single executor.
func (s *Store) GetHeartbeat(ctx context.Context, namespace string, executorID string) (*store.HeartbeatState, error) {
	// The prefix for all keys related to a single executor.
	executorPrefix := s.buildExecutorKey(namespace, executorID, "")
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get failed for executor %s: %w", executorID, err)
	}

	if resp.Count == 0 {
		return nil, store.ErrExecutorNotFound
	}

	state := &store.HeartbeatState{ExecutorID: executorID}
	found := false

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)
		_, keyType, keyErr := s.parseExecutorKey(namespace, key)
		if keyErr != nil {
			continue // Ignore unexpected keys
		}

		found = true // We found at least one valid key part for the executor.
		switch keyType {
		case heartbeatKey:
			timestamp, _ := strconv.ParseInt(value, 10, 64)
			state.LastHeartbeat = timestamp
		case stateKey:
			state.State = store.ExecutorState(value)
		case reportedShardsKey:
			err = json.Unmarshal(kv.Value, &state.ReportedShards)
			if err != nil {
				return nil, fmt.Errorf("unmarshal reported shards: %w", err)
			}
		}
	}

	if !found {
		// This case is unlikely if resp.Count > 0, but is a good safeguard.
		return nil, store.ErrExecutorNotFound
	}

	return state, nil
}

// --- ShardStore Implementation ---

func (s *Store) GetState(ctx context.Context, namespace string) (map[string]store.HeartbeatState, map[string]store.AssignedState, int64, error) {
	heartbeatStates := make(map[string]store.HeartbeatState)
	assignedStates := make(map[string]store.AssignedState)

	executorPrefix := s.buildExecutorPrefix(namespace)
	resp, err := s.client.Get(ctx, executorPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, nil, 0, fmt.Errorf("get executor data: %w", err)
	}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		value := string(kv.Value)
		executorID, keyType, keyErr := s.parseExecutorKey(namespace, key)
		if keyErr != nil {
			continue
		}
		if _, ok := heartbeatStates[executorID]; !ok {
			heartbeatStates[executorID] = store.HeartbeatState{ExecutorID: executorID}
		}
		if _, ok := assignedStates[executorID]; !ok {
			assignedStates[executorID] = store.AssignedState{
				ExecutorID:     executorID,
				ReportedShards: make(map[string]store.ShardState),
				AssignedShards: make(map[string]store.ShardAssignment),
			}
		}
		heartbeat := heartbeatStates[executorID]
		assigned := assignedStates[executorID]
		switch keyType {
		case heartbeatKey:
			timestamp, _ := strconv.ParseInt(value, 10, 64)
			heartbeat.LastHeartbeat = timestamp
		case stateKey:
			heartbeat.State = store.ExecutorState(value)
		case reportedShardsKey:
			err = json.Unmarshal(kv.Value, &assigned.ReportedShards)
			if err != nil {
				return nil, nil, 0, fmt.Errorf("unmarshal reported shards: %w", err)
			}
		case assignedShardsKey:
			err = json.Unmarshal(kv.Value, &assigned.AssignedShards)
			if err != nil {
				return nil, nil, 0, fmt.Errorf("unmarshal assigned shards: %w", err)
			}
		}
		heartbeatStates[executorID] = heartbeat
		assignedStates[executorID] = assigned
	}
	return heartbeatStates, assignedStates, resp.Header.Revision, nil
}

func (s *Store) Subscribe(ctx context.Context, namespace string) (<-chan int64, error) {
	revisionChan := make(chan int64, 1)
	watchPrefix := s.buildExecutorPrefix(namespace)
	go func() {
		defer close(revisionChan)
		watchChan := s.client.Watch(ctx, watchPrefix, clientv3.WithPrefix())
		for watchResp := range watchChan {
			if err := watchResp.Err(); err != nil {
				return
			}
			isSignificantChange := false
			for _, event := range watchResp.Events {
				if !event.IsCreate() && !event.IsModify() {
					isSignificantChange = true
					break
				}
				_, keyType, err := s.parseExecutorKey(namespace, string(event.Kv.Key))
				if err != nil {
					continue
				}
				if keyType != heartbeatKey && keyType != assignedShardsKey {
					isSignificantChange = true
					break
				}
			}
			if isSignificantChange {
				select {
				case <-revisionChan:
				default:
				}
				revisionChan <- watchResp.Header.Revision
			}
		}
	}()
	return revisionChan, nil
}

func (s *Store) AssignShards(ctx context.Context, namespace string, newState map[string]store.AssignedState, guard store.GuardFunc) error {
	var ops []clientv3.Op
	for executorID, state := range newState {
		key := s.buildExecutorKey(namespace, executorID, assignedShardsKey)
		value, err := json.Marshal(state.AssignedShards)
		if err != nil {
			return fmt.Errorf("marshal assigned shards: %w", err)
		}
		ops = append(ops, clientv3.OpPut(key, string(value)))
	}
	if len(ops) == 0 {
		return nil
	}

	nativeTxn := s.client.Txn(ctx)
	guardedTxn, err := guard(nativeTxn)
	if err != nil {
		return fmt.Errorf("apply transaction guard: %w", err)
	}
	etcdGuardedTxn, ok := guardedTxn.(clientv3.Txn)
	if !ok {
		return fmt.Errorf("guard function returned invalid transaction type")
	}

	etcdGuardedTxn = etcdGuardedTxn.Then(ops...)
	resp, err := etcdGuardedTxn.Commit()
	if err != nil {
		return fmt.Errorf("commit shard assignments: %w", err)
	}
	if !resp.Succeeded {
		return fmt.Errorf("transaction failed, leadership may have changed")
	}
	return nil
}

func (s *Store) DeleteExecutors(ctx context.Context, namespace string, executorIDs []string, guard store.GuardFunc) error {
	if len(executorIDs) == 0 {
		return nil
	}
	var ops []clientv3.Op
	for _, executorID := range executorIDs {
		executorPrefix := fmt.Sprintf("%s%s/", s.buildExecutorPrefix(namespace), executorID)
		ops = append(ops, clientv3.OpDelete(executorPrefix, clientv3.WithPrefix()))
	}

	nativeTxn := s.client.Txn(ctx)
	guardedTxn, err := guard(nativeTxn)
	if err != nil {
		return fmt.Errorf("apply transaction guard: %w", err)
	}
	etcdGuardedTxn, ok := guardedTxn.(clientv3.Txn)
	if !ok {
		return fmt.Errorf("guard function returned invalid transaction type")
	}

	etcdGuardedTxn = etcdGuardedTxn.Then(ops...)
	resp, err := etcdGuardedTxn.Commit()
	if err != nil {
		return fmt.Errorf("commit executor deletion: %w", err)
	}
	if !resp.Succeeded {
		return fmt.Errorf("transaction failed, leadership may have changed")
	}
	return nil
}

// --- Key Management Utilities ---

func (s *Store) buildNamespacePrefix(namespace string) string {
	return fmt.Sprintf("%s/%s", s.prefix, namespace)
}

func (s *Store) buildExecutorPrefix(namespace string) string {
	return fmt.Sprintf("%s/executors/", s.buildNamespacePrefix(namespace))
}

func (s *Store) buildExecutorKey(namespace, executorID, keyType string) string {
	return fmt.Sprintf("%s%s/%s", s.buildExecutorPrefix(namespace), executorID, keyType)
}

func (s *Store) parseExecutorKey(namespace, key string) (executorID, keyType string, err error) {
	prefix := s.buildExecutorPrefix(namespace)
	if !strings.HasPrefix(key, prefix) {
		return "", "", fmt.Errorf("key '%s' does not have expected prefix '%s'", key, prefix)
	}
	remainder := strings.TrimPrefix(key, prefix)
	parts := strings.Split(remainder, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected key format: %s", key)
	}
	return parts[0], parts[1], nil
}
