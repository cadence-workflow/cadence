package metrics

import "fmt"

// MigrationConfig groups all metric migration configurations.
// Future migration types can be added here without changing NewClient's signature.
type MigrationConfig struct {
	Histogram HistogramMigration
	Gauge     GaugeMigration
	Counter   CounterMigration
}

// EmitTimer returns true if the metric should be emitted as a timer.
// A metric is suppressed from timer emission if the migration that owns it
// has been configured to move away from timer.
// Metrics not in any migration map are always emitted.
func (mc MigrationConfig) EmitTimer(name string) bool {
	return mc.Histogram.EmitTimer(name) &&
		mc.Gauge.EmitTimer(name) &&
		mc.Counter.EmitTimer(name)
}

type HistogramMigration struct {
	Default HistogramMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or histogram should be emitted.
	//
	// This is only checked for timers and histograms that are in HistogramMigrationMetrics.
	Names map[string]bool `yaml:"names"`
}

func (h *HistogramMigration) UnmarshalYAML(read func(any) error) error {
	type tmpType HistogramMigration // without the custom unmarshaler
	var tmp tmpType
	if err := read(&tmp); err != nil {
		return err
	}
	for k := range tmp.Names {
		if _, ok := HistogramMigrationMetrics[k]; !ok {
			return fmt.Errorf(
				"unknown histogram-migration metric name %q.  "+
					"if this is a valid name, add it to common/metrics.HistogramMigrationMetrics before starting the service",
				k,
			)
		}
	}
	*h = HistogramMigration(tmp)
	return nil
}

// HistogramMigrationMetrics contains all metric names being migrated, to prevent affecting
// non-migration-related timers and histograms, and to catch metric name config
// mistakes early on.
//
// It is public to allow Cadence operators to add to the collection before
// loading config, in case they have any custom migrations to perform.
// This is likely best done in an `init` func, to ensure it happens early enough
// and does not race with config reading.
var HistogramMigrationMetrics = map[string]struct{}{
	"task_attempt":                          {},
	"task_attempt_counts":                   {},
	"task_attempt_per_domain":               {},
	"task_attempt_per_domain_counts":        {},
	"task_latency":                          {},
	"task_latency_ns":                       {},
	"task_latency_per_domain":               {},
	"task_latency_per_domain_ns":            {},
	"task_latency_processing":               {},
	"task_latency_processing_ns":            {},
	"task_latency_queue":                    {},
	"task_latency_queue_ns":                 {},
	"task_latency_processing_per_domain":    {},
	"task_latency_processing_per_domain_ns": {},
	"task_latency_queue_per_domain":         {},
	"task_latency_queue_per_domain_ns":      {},

	"replication_tasks_lag":                {},
	"replication_tasks_lag_counts":         {},
	"replication_tasks_applied_latency":    {},
	"replication_tasks_applied_latency_ns": {},

	"cache_latency":     {},
	"cache_latency_ns":  {},
	"cache_size":        {},
	"cache_size_counts": {},

	"replication_task_latency":    {},
	"replication_task_latency_ns": {},

	"replication_tasks_returned":             {},
	"replication_tasks_returned_counts":      {},
	"replication_tasks_returned_diff":        {},
	"replication_tasks_returned_diff_counts": {},

	"replication_tasks_fetched":        {},
	"replication_tasks_fetched_counts": {},
	"replication_tasks_lag_raw":        {},
	"replication_tasks_lag_raw_counts": {},

	"activity_end_to_end_latency":    {},
	"activity_end_to_end_latency_ns": {},

	"schedule_to_start_history_queue_latency_per_tl":    {},
	"schedule_to_start_history_queue_latency_per_tl_ns": {},

	"processing_queue_num":        {},
	"processing_queue_num_counts": {},

	"processing_queue_max_level":        {},
	"processing_queue_max_level_counts": {},

	"persistence_latency_per_domain":    {},
	"persistence_latency_per_domain_ns": {},
	"persistence_latency":               {},
	"persistence_latency_ns":            {},
	"persistence_latency_histogram":     {},

	"persistence_latency_per_shard":    {},
	"persistence_latency_per_shard_ns": {},

	"history_size":         {},
	"history_size_counts":  {},
	"history_count":        {},
	"history_count_counts": {},

	"mutable_state_size":               {},
	"mutable_state_size_counts":        {},
	"execution_info_size":              {},
	"execution_info_size_counts":       {},
	"activity_info_size":               {},
	"activity_info_size_counts":        {},
	"timer_info_size":                  {},
	"timer_info_size_counts":           {},
	"child_info_size":                  {},
	"child_info_size_counts":           {},
	"signal_info_size":                 {},
	"signal_info_size_counts":          {},
	"buffered_events_size":             {},
	"buffered_events_size_counts":      {},
	"activity_info_count":              {},
	"activity_info_count_counts":       {},
	"timer_info_count":                 {},
	"timer_info_count_counts":          {},
	"child_info_count":                 {},
	"child_info_count_counts":          {},
	"signal_info_count":                {},
	"signal_info_count_counts":         {},
	"request_cancel_info_count":        {},
	"request_cancel_info_count_counts": {},
	"buffered_events_count":            {},
	"buffered_events_count_counts":     {},

	"elasticsearch_latency_per_domain":    {},
	"elasticsearch_latency_per_domain_ns": {},
	"elasticsearch_latency":               {},
	"elasticsearch_latency_ns":            {},

	"event_blob_size":        {},
	"event_blob_size_counts": {},

	"paralleltask_submit_latency":             {},
	"paralleltask_submit_latency_ns":          {},
	"paralleltask_task_processing_latency":    {},
	"paralleltask_task_processing_latency_ns": {},

	"decision_attempt":        {},
	"decision_attempt_counts": {},

	"acquire_shards_latency":    {},
	"acquire_shards_latency_ns": {},

	"transfer_tasks_count":           {},
	"transfer_tasks_count_counts":    {},
	"timer_tasks_count":              {},
	"timer_tasks_count_counts":       {},
	"replication_tasks_count":        {},
	"replication_tasks_count_counts": {},

	"delete_activity_info":              {},
	"delete_activity_info_counts":       {},
	"delete_timer_info":                 {},
	"delete_timer_info_counts":          {},
	"delete_child_info":                 {},
	"delete_child_info_counts":          {},
	"delete_signal_info":                {},
	"delete_signal_info_counts":         {},
	"delete_request_cancel_info":        {},
	"delete_request_cancel_info_counts": {},

	"workflow_id_external_requests_max_requests_per_seconds":        {},
	"workflow_id_external_requests_max_requests_per_seconds_counts": {},
	"workflow_id_internal_requests_max_requests_per_seconds":        {},
	"workflow_id_internal_requests_max_requests_per_seconds_counts": {},

	"decision_result_count":        {},
	"decision_result_count_counts": {},

	"sharditem_acquisition_latency":    {},
	"sharditem_acquisition_latency_ns": {},

	"get_engine_for_shard_latency":       {},
	"get_engine_for_shard_latency_ns":    {},
	"remove_engine_for_shard_latency":    {},
	"remove_engine_for_shard_latency_ns": {},

	"domain_cache_prepare_callbacks_latency":    {},
	"domain_cache_prepare_callbacks_latency_ns": {},
	"domain_cache_callbacks_latency":            {},
	"domain_cache_callbacks_latency_ns":         {},

	"history_archiver_total_upload_size":        {},
	"history_archiver_total_upload_size_counts": {},
	"history_archiver_history_size":             {},
	"history_archiver_history_size_counts":      {},
	"history_archiver_blob_size":                {},
	"history_archiver_blob_size_counts":         {},

	"shardinfo_replication_pending_task":             {},
	"shardinfo_replication_pending_task_counts":      {},
	"shardinfo_transfer_active_pending_task":         {},
	"shardinfo_transfer_active_pending_task_counts":  {},
	"shardinfo_transfer_standby_pending_task":        {},
	"shardinfo_transfer_standby_pending_task_counts": {},
	"shardinfo_timer_active_pending_task":            {},
	"shardinfo_timer_active_pending_task_counts":     {},
	"shardinfo_timer_standby_pending_task":           {},
	"shardinfo_timer_standby_pending_task_counts":    {},
	"shardinfo_cross_cluster_pending_task":           {},
	"shardinfo_cross_cluster_pending_task_counts":    {},

	"shardinfo_replication_lag":        {},
	"shardinfo_replication_lag_counts": {},
	"shardinfo_transfer_lag":           {},
	"shardinfo_transfer_lag_counts":    {},
	"shardinfo_timer_lag":              {},
	"shardinfo_timer_lag_ns":           {},

	"shardinfo_transfer_diff":        {},
	"shardinfo_transfer_diff_counts": {},
	"shardinfo_timer_diff":           {},
	"shardinfo_timer_diff_ns":        {},

	"shardinfo_transfer_failover_in_progress":        {},
	"shardinfo_transfer_failover_in_progress_counts": {},
	"shardinfo_timer_failover_in_progress":           {},
	"shardinfo_timer_failover_in_progress_counts":    {},

	"shardinfo_transfer_failover_latency":    {},
	"shardinfo_transfer_failover_latency_ns": {},
	"shardinfo_timer_failover_latency":       {},
	"shardinfo_timer_failover_latency_ns":    {},

	"history_event_notification_queueing_latency":    {},
	"history_event_notification_queueing_latency_ns": {},
	"history_event_notification_fanout_latency":      {},
	"history_event_notification_fanout_latency_ns":   {},

	"direct_query_dispatch_latency":                     {},
	"direct_query_dispatch_latency_ns":                  {},
	"direct_query_dispatch_sticky_latency":              {},
	"direct_query_dispatch_sticky_latency_ns":           {},
	"direct_query_dispatch_non_sticky_latency":          {},
	"direct_query_dispatch_non_sticky_latency_ns":       {},
	"direct_query_dispatch_clear_stickiness_latency":    {},
	"direct_query_dispatch_clear_stickiness_latency_ns": {},
	"cadence_authorization_latency":                     {},
	"cadence_authorization_latency_ns":                  {},

	"pinot_latency":               {},
	"pinot_latency_ns":            {},
	"pinot_latency_per_domain":    {},
	"pinot_latency_per_domain_ns": {},

	"sequentialtask_submit_latency":              {},
	"sequentialtask_submit_latency_ns":           {},
	"sequentialtask_queue_size":                  {},
	"sequentialtask_queue_size_counts":           {},
	"sequentialtask_queue_processing_latency":    {},
	"sequentialtask_queue_processing_latency_ns": {},
	"sequentialtask_task_processing_latency":     {},
	"sequentialtask_task_processing_latency_ns":  {},

	"prioritytask_submit_latency":    {},
	"prioritytask_submit_latency_ns": {},

	"graceful_failover_latency":    {},
	"graceful_failover_latency_ns": {},

	"async_request_payload_size_per_domain":        {},
	"async_request_payload_size_per_domain_counts": {},

	"task_redispatch_queue_pending_tasks":        {},
	"task_redispatch_queue_pending_tasks_counts": {},

	"workflow_context_lock_latency":    {},
	"workflow_context_lock_latency_ns": {},

	"get_replication_messages_for_shard":    {},
	"get_replication_messages_for_shard_ns": {},
	"get_dlq_replication_messages":          {},
	"get_dlq_replication_messages_ns":       {},

	"decision_task_query_latency":    {},
	"decision_task_query_latency_ns": {},

	"syncmatch_latency_per_tl":     {},
	"syncmatch_latency_per_tl_ns":  {},
	"asyncmatch_latency_per_tl":    {},
	"asyncmatch_latency_per_tl_ns": {},

	"asyncmatch_local_poll_attempt_per_tl":                             {},
	"asyncmatch_local_poll_attempt_per_tl_counts":                      {},
	"asyncmatch_forward_poll_attempt_per_tl":                           {},
	"asyncmatch_forward_poll_attempt_per_tl_counts":                    {},
	"asyncmatch_local_poll_after_forward_failed_attempt_per_tl":        {},
	"asyncmatch_local_poll_after_forward_failed_attempt_per_tl_counts": {},

	"poll_local_match_latency_per_tl":                         {},
	"poll_local_match_latency_per_tl_ns":                      {},
	"poll_forward_match_latency_per_tl":                       {},
	"poll_forward_match_latency_per_tl_ns":                    {},
	"poll_local_match_after_forward_failed_latency_per_tl":    {},
	"poll_local_match_after_forward_failed_latency_per_tl_ns": {},

	"es_processor_process_msg_latency":       {},
	"es_processor_process_msg_latency_ns":    {},
	"index_processor_process_msg_latency":    {},
	"index_processor_process_msg_latency_ns": {},

	"async_workflow_process_msg_latency":        {},
	"async_workflow_process_msg_latency_ns":     {},
	"diagnostics_workflow_execution_latency":    {},
	"diagnostics_workflow_execution_latency_ns": {},

	"shard_distributor_latency":    {},
	"shard_distributor_latency_ns": {},

	"global_ratelimiter_update_latency":    {},
	"global_ratelimiter_update_latency_ns": {},

	"cadence_latency":                       {},
	"cadence_latency_ns":                    {},
	"cadence_client_latency":                {},
	"cadence_client_latency_ns":             {},
	"cadence_client_latency_redirection":    {},
	"cadence_client_latency_redirection_ns": {},
	"cadence_latency_per_tl":                {},
	"cadence_latency_per_tl_ns":             {},
}

func (h HistogramMigration) EmitTimer(name string) bool {
	if _, ok := HistogramMigrationMetrics[name]; !ok {
		return true
	}
	emit, ok := h.Names[name]
	if ok {
		return emit
	}
	return h.Default.EmitTimer()
}
func (h HistogramMigration) EmitHistogram(name string) bool {
	if _, ok := HistogramMigrationMetrics[name]; !ok {
		return true
	}

	emit, ok := h.Names[name]
	if ok {
		return emit
	}
	return h.Default.EmitHistogram()
}

// HistogramMigrationMode is a pseudo-enum to provide unmarshalling config and helper methods.
// It should only be created by YAML unmarshaling, or by getting it from the HistogramMigration map.
// Zero values from the map are valid, they are just the default mode (NOT the configured default).
//
// By default / when not specified / when an empty string, it currently means "timer".
// This will likely change when most or all timers have histograms available, and will
// eventually be fully deprecated and removed.
type HistogramMigrationMode string

func (h *HistogramMigrationMode) UnmarshalYAML(read func(any) error) error {
	var value string
	if err := read(&value); err != nil {
		return fmt.Errorf("cannot read histogram migration mode as a string: %w", err)
	}
	switch value {
	case "timer", "histogram", "both":
		*h = HistogramMigrationMode(value)
	default:
		return fmt.Errorf(`unsupported histogram migration mode %q, must be "timer", "histogram", or "both"`, value)
	}
	return nil
}

func (h HistogramMigrationMode) EmitTimer() bool {
	switch h {
	case "timer", "both", "": // default == not specified == both
		return true
	default:
		return false
	}
}

func (h HistogramMigrationMode) EmitHistogram() bool {
	switch h {
	case "histogram", "both": // default == not specified == both
		return true
	default:
		return false
	}
}

type GaugeMigration struct {
	Default GaugeMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or gauge should be emitted.
	//
	// This is only checked for timers and gauges that are in GaugeMigrationMetrics.
	Names map[string]bool `yaml:"names"`
}

func (g *GaugeMigration) UnmarshalYAML(read func(any) error) error {
	type tmpType GaugeMigration // without the custom unmarshaler
	var tmp tmpType
	if err := read(&tmp); err != nil {
		return err
	}
	for k := range tmp.Names {
		if _, ok := GaugeMigrationMetrics[k]; !ok {
			return fmt.Errorf(
				"unknown gauge-migration metric name %q.  "+
					"if this is a valid name, add it to common/metrics.GaugeMigrationMetrics before starting the service",
				k,
			)
		}
	}
	*g = GaugeMigration(tmp)
	return nil
}

// GaugeMigrationMetrics contains all metric names being migrated, to prevent affecting
// non-migration-related timers and gauges, and to catch metric name config
// mistakes early on.
//
// It is public to allow Cadence operators to add to the collection before
// loading config, in case they have any custom migrations to perform.
// This is likely best done in an `init` func, to ensure it happens early enough
// and does not race with config reading.
var GaugeMigrationMetrics = map[string]struct{}{
	"cache_size_gauge":                {},
	"replication_tasks_lag_gauge":     {},
	"replication_tasks_lag_raw_gauge": {},
}

func (g GaugeMigration) EmitTimer(name string) bool {
	if _, ok := GaugeMigrationMetrics[name]; !ok {
		return true
	}
	emit, ok := g.Names[name]
	if ok {
		return emit
	}
	return g.Default.EmitTimer()
}
func (g GaugeMigration) EmitGauge(name string) bool {
	if _, ok := GaugeMigrationMetrics[name]; !ok {
		return true
	}

	emit, ok := g.Names[name]
	if ok {
		return emit
	}
	return g.Default.EmitGauge()
}

// GaugeMigrationMode is a pseudo-enum to provide unmarshalling config and helper methods.
// It should only be created by YAML unmarshaling, or by getting it from the GaugeMigration map.
// Zero values from the map are valid, they are just the default mode (NOT the configured default).
//
// By default / when not specified / when an empty string, it currently means "timer".
// This will likely change when most or all timers have gauges available, and will
// eventually be fully deprecated and removed.
type GaugeMigrationMode string

func (g *GaugeMigrationMode) UnmarshalYAML(read func(any) error) error {
	var value string
	if err := read(&value); err != nil {
		return fmt.Errorf("cannot read gauge migration mode as a string: %w", err)
	}
	switch value {
	case "timer", "gauge", "both":
		*g = GaugeMigrationMode(value)
	default:
		return fmt.Errorf(`unsupported gauge migration mode %q, must be "timer", "gauge", or "both"`, value)
	}
	return nil
}

func (g GaugeMigrationMode) EmitTimer() bool {
	switch g {
	case "timer", "both", "": // default == not specified == timer
		return true
	default:
		return false
	}
}

func (g GaugeMigrationMode) EmitGauge() bool {
	switch g {
	case "gauge", "both": // default == not specified == timer
		return true
	default:
		return false
	}
}

type CounterMigration struct {
	Default CounterMigrationMode `yaml:"default"`
	// Names maps "metric name" -> "should it be emitted".
	//
	// If a name/key does not exist, the default mode will be checked to determine
	// if a timer or counter should be emitted.
	//
	// This is only checked for timers and counters that are in CounterMigrationMetrics.
	Names map[string]bool `yaml:"names"`
}

func (c *CounterMigration) UnmarshalYAML(read func(any) error) error {
	type tmpType CounterMigration // without the custom unmarshaler
	var tmp tmpType
	if err := read(&tmp); err != nil {
		return err
	}
	for k := range tmp.Names {
		if _, ok := CounterMigrationMetrics[k]; !ok {
			return fmt.Errorf(
				"unknown counter-migration metric name %q.  "+
					"if this is a valid name, add it to common/metrics.CounterMigrationMetrics before starting the service",
				k,
			)
		}
	}
	*c = CounterMigration(tmp)
	return nil
}

// CounterMigrationMetrics contains all metric names being migrated, to prevent affecting
// non-migration-related timers and counters, and to catch metric name config
// mistakes early on.
//
// It is public to allow Cadence operators to add to the collection before
// loading config, in case they have any custom migrations to perform.
// This is likely best done in an `init` func, to ensure it happens early enough
// and does not race with config reading.
var CounterMigrationMetrics = map[string]struct{}{
	"replication_tasks_fetched_counter":       {},
	"replication_tasks_returned_counter":      {},
	"replication_tasks_returned_diff_counter": {},
}

func (c CounterMigration) EmitTimer(name string) bool {
	if _, ok := CounterMigrationMetrics[name]; !ok {
		return true
	}
	emit, ok := c.Names[name]
	if ok {
		return emit
	}
	return c.Default.EmitTimer()
}
func (c CounterMigration) EmitCounter(name string) bool {
	if _, ok := CounterMigrationMetrics[name]; !ok {
		return true
	}

	emit, ok := c.Names[name]
	if ok {
		return emit
	}
	return c.Default.EmitCounter()
}

// CounterMigrationMode is a pseudo-enum to provide unmarshalling config and helper methods.
// It should only be created by YAML unmarshaling, or by getting it from the CounterMigration map.
// Zero values from the map are valid, they are just the default mode (NOT the configured default).
//
// By default / when not specified / when an empty string, it currently means "timer".
// This will likely change when most or all timers have counters available, and will
// eventually be fully deprecated and removed.
type CounterMigrationMode string

func (c *CounterMigrationMode) UnmarshalYAML(read func(any) error) error {
	var value string
	if err := read(&value); err != nil {
		return fmt.Errorf("cannot read counter migration mode as a string: %w", err)
	}
	switch value {
	case "timer", "counter", "both":
		*c = CounterMigrationMode(value)
	default:
		return fmt.Errorf(`unsupported counter migration mode %q, must be "timer", "counter", or "both"`, value)
	}
	return nil
}

func (c CounterMigrationMode) EmitTimer() bool {
	switch c {
	case "timer", "both", "": // default == not specified == timer
		return true
	default:
		return false
	}
}

func (c CounterMigrationMode) EmitCounter() bool {
	switch c {
	case "counter", "both": // default == not specified == timer
		return true
	default:
		return false
	}
}
