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

package metrics

import (
	"regexp"
	"strconv"
)

const (
	revisionTag       = "revision"
	branchTag         = "branch"
	buildDateTag      = "build_date"
	buildVersionTag   = "build_version"
	goVersionTag      = "go_version"
	cadenceVersionTag = "cadence_version"

	instance                  = "instance"
	domain                    = "domain"
	domainType                = "domain_type"
	clusterGroup              = "cluster_group"
	sourceCluster             = "source_cluster"
	targetCluster             = "target_cluster"
	activeCluster             = "active_cluster"
	taskList                  = "tasklist"
	taskListType              = "tasklistType"
	taskListRootPartition     = "tasklist_root_partition"
	workflowType              = "workflowType"
	activityType              = "activityType"
	decisionType              = "decisionType"
	invariantType             = "invariantType"
	shardScannerScanResult    = "shardscanner_scan_result"
	shardScannerFixResult     = "shardscanner_fix_result"
	kafkaPartition            = "kafkaPartition"
	transport                 = "transport"
	caller                    = "caller"
	service                   = "service"
	destService               = "dest_service"
	signalName                = "signalName"
	workflowVersion           = "workflow_version"
	shardID                   = "shard_id"
	matchingHost              = "matching_host"
	host                      = "host"
	pollerIsolationGroup      = "poller_isolation_group"
	asyncWFRequestType        = "async_wf_request_type"
	workflowTerminationReason = "workflow_termination_reason"
	workflowCloseStatus       = "workflow_close_status"
	isolationEnabled          = "isolation_enabled"
	isolationGroup            = "isolation_group"
	leakCause                 = "leak_cause"
	topic                     = "topic"
	mode                      = "mode"

	// limiter-side tags
	globalRatelimitKey            = "global_ratelimit_key"
	globalRatelimitType           = "global_ratelimit_type"
	globalRatelimitIsPrimary      = "is_primary"
	globalRatelimitCollectionName = "global_ratelimit_collection"

	allValue     = "all"
	unknownValue = "_unknown_"
)

var (
	safeAlphaNumericStringRE = regexp.MustCompile(`[^a-zA-Z0-9]`)
)

// Tag is an interface to define metrics tags
type (
	Tag interface {
		Key() string
		Value() string
	}

	simpleMetric struct {
		key   string
		value string
	}
)

func (s simpleMetric) Key() string   { return s.key }
func (s simpleMetric) Value() string { return s.value }

func metricWithUnknown(key, value string) Tag {
	if len(value) == 0 {
		value = unknownValue
	}
	return simpleMetric{key: key, value: value}
}

func ShardIDTag(shardIDVal int) Tag {
	return metricWithUnknown(shardID, strconv.Itoa(shardIDVal))
}

// DomainTag returns a new domain tag. For timers, this also ensures that we
// dual emit the metric with the all tag. If a blank domain is provided then
// this converts that to an unknown domain.
func DomainTag(value string) Tag {
	return metricWithUnknown(domain, value)
}

// DomainTypeTag returns a tag for domain type.
// This allows differentiating between global/local domains.
func DomainTypeTag(isGlobal bool) Tag {
	var value string
	if isGlobal {
		value = "global"
	} else {
		value = "local"
	}
	return simpleMetric{key: domainType, value: value}
}

// DomainUnknownTag returns a new domain:unknown tag-value
func DomainUnknownTag() Tag {
	return DomainTag("")
}

// ClusterGroupTag return a new cluster group tag
func ClusterGroupTag(value string) Tag {
	return simpleMetric{key: clusterGroup, value: value}
}

// InstanceTag returns a new instance tag
func InstanceTag(value string) Tag {
	return simpleMetric{key: instance, value: value}
}

// SourceClusterTag returns a new source cluster tag.
func SourceClusterTag(value string) Tag {
	return metricWithUnknown(sourceCluster, value)
}

// TargetClusterTag returns a new target cluster tag.
func TargetClusterTag(value string) Tag {
	return metricWithUnknown(targetCluster, value)
}

// ActiveClusterTag returns a new active cluster type tag.
func ActiveClusterTag(value string) Tag {
	return metricWithUnknown(activeCluster, value)
}

// TaskListTag returns a new task list tag.
func TaskListTag(value string) Tag {
	if len(value) == 0 {
		value = unknownValue
	}
	return simpleMetric{key: taskList, value: sanitizer.Value(value)}
}

// TaskListUnknownTag returns a new tasklist:unknown tag-value
func TaskListUnknownTag() Tag {
	return simpleMetric{key: taskList, value: unknownValue}
}

// TaskListTypeTag returns a new task list type tag.
func TaskListTypeTag(value string) Tag {
	return metricWithUnknown(taskListType, value)
}

// TaskListRootPartition returns a new task list root partition tag.
func TaskListRootPartitionTag(value string) Tag {
	if len(value) == 0 {
		value = unknownValue
	}
	return simpleMetric{key: taskListRootPartition, value: sanitizer.Value(value)}
}

// WorkflowTypeTag returns a new workflow type tag.
func WorkflowTypeTag(value string) Tag {
	return metricWithUnknown(workflowType, value)
}

// ActivityTypeTag returns a new activity type tag.
func ActivityTypeTag(value string) Tag {
	return metricWithUnknown(activityType, value)
}

// DecisionTypeTag returns a new decision type tag.
func DecisionTypeTag(value string) Tag {
	return metricWithUnknown(decisionType, value)
}

// ShardScannerScanResult returns a new shardscanner scan result type tag.
func ShardScannerScanResult(value string) Tag {
	return metricWithUnknown(shardScannerScanResult, value)
}

// ShardScannerFixResult returns a new shardscanner fix result type tag.
func ShardScannerFixResult(value string) Tag {
	return metricWithUnknown(shardScannerFixResult, value)
}

// InvariantTypeTag returns a new invariant type tag.
func InvariantTypeTag(value string) Tag {
	return metricWithUnknown(invariantType, value)
}

// KafkaPartitionTag returns a new KafkaPartition type tag.
func KafkaPartitionTag(value int32) Tag {
	return simpleMetric{key: kafkaPartition, value: strconv.Itoa(int(value))}
}

// TransportTag returns a new RPC Transport type tag.
func TransportTag(value string) Tag {
	return simpleMetric{key: transport, value: value}
}

// CallerTag returns a new RPC Caller type tag.
func CallerTag(value string) Tag {
	return simpleMetric{key: caller, value: value}
}

// ServiceTag returns a new service tag.
func ServiceTag(value string) Tag {
	return simpleMetric{key: service, value: value}
}

// DestServiceTag returns a new destination service tag.
func DestServiceTag(value string) Tag {
	return simpleMetric{key: destService, value: value}
}

// Hosttag emits the host identifier
func HostTag(value string) Tag {
	return metricWithUnknown(host, value)
}

// SignalNameTag returns a new SignalName tag
func SignalNameTag(value string) Tag {
	return metricWithUnknown(signalName, value)
}

// SignalNameAllTag returns a new SignalName tag with all value
func SignalNameAllTag() Tag {
	return metricWithUnknown(signalName, allValue)
}

// WorkflowVersionTag returns a new WorkflowVersion tag
func WorkflowVersionTag(value string) Tag {
	return metricWithUnknown(workflowVersion, value)
}

func MatchingHostTag(value string) Tag {
	return metricWithUnknown(matchingHost, value)
}

// PollerIsolationGroupTag returns a new PollerIsolationGroup tag
func PollerIsolationGroupTag(value string) Tag {
	return metricWithUnknown(pollerIsolationGroup, value)
}

// AsyncWFRequestTypeTag returns a new AsyncWFRequestTypeTag tag
func AsyncWFRequestTypeTag(value string) Tag {
	return metricWithUnknown(asyncWFRequestType, value)
}

// GlobalRatelimiterKeyTag reports the local ratelimit key being used, e.g. "domain-x".
// This will likely be ambiguous if it is not combined with the collection name,
// but keeping this untouched helps keep the values template-friendly and correlate-able
// in metrics dashboards and queries.
func GlobalRatelimiterKeyTag(value string) Tag {
	return simpleMetric{key: globalRatelimitKey, value: value}
}

// GlobalRatelimiterTypeTag reports the "limit usage type" being reported, e.g. global vs local
func GlobalRatelimiterTypeTag(value string) Tag {
	return simpleMetric{key: globalRatelimitType, value: value}
}

func GlobalRatelimiterIsPrimary(isPrimary bool) Tag {
	value := "false"
	if isPrimary {
		value = "true"
	}
	return simpleMetric{key: globalRatelimitIsPrimary, value: value}
}

// GlobalRatelimiterCollectionName is a namespacing tag to uniquely identify metrics
// coming from the different ratelimiter collections (user, worker, visibility, async).
func GlobalRatelimiterCollectionName(value string) Tag {
	return simpleMetric{key: globalRatelimitCollectionName, value: value}
}

// WorkflowTerminationReasonTag reports the reason for workflow termination
func WorkflowTerminationReasonTag(value string) Tag {
	value = safeAlphaNumericStringRE.ReplaceAllString(value, "_")
	return simpleMetric{key: workflowTerminationReason, value: value}
}

// WorkflowCloseStatusTag is a stringified workflow status
func WorkflowCloseStatusTag(value string) Tag {
	value = safeAlphaNumericStringRE.ReplaceAllString(value, "_")
	return simpleMetric{key: workflowCloseStatus, value: value}
}

func IsolationGroupTag(group string) Tag {
	return simpleMetric{key: isolationGroup, value: sanitizer.Value(group)}
}

func IsolationLeakCause(cause string) Tag {
	return simpleMetric{key: leakCause, value: sanitizer.Value(cause)}
}

// IsolationEnabledTag returns whether isolation is enabled
func IsolationEnabledTag(enabled bool) Tag {
	v := "false"
	if enabled {
		v = "true"
	}
	return simpleMetric{key: isolationEnabled, value: v}
}

func TopicTag(value string) Tag {
	return metricWithUnknown(topic, value)
}

func ModeTag(value string) Tag {
	return metricWithUnknown(mode, value)
}

func NamespaceTag(namespace string) Tag {
	return metricWithUnknown("namespace", namespace)
}

func TaskCategoryTag(category string) Tag {
	return metricWithUnknown("task_category", category)
}

// ReasonTag returns a new reason tag
func ReasonTag(reason string) Tag {
	return metricWithUnknown("reason", reason)
}

// DecisionTag returns a new decision tag
func DecisionTag(decision string) Tag {
	return metricWithUnknown("decision", decision)
}
