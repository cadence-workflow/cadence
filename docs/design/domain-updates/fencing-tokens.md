# Domain versions and fencing tokens with Cassandra

Last Updated: October 2025

### Background 

Cadence uses [fencing tokens](https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html) heavily due to it's reliance on databases which don't allow for cross-table transactions (in Cassandra particularly). In most instances, they're used simply as a counter, to perform optimistic updates with a compare-and-set, such that if they're operating in stale data, it's integrity can be protected. However, there's a few cases where they pass additional information such as the cluster that's active as well.

There's quite a few of these tokens which have different purposes, particularly on the Domain side, and keeping track of their intent is a little tricky. This is a basic catelog of their mechanism and intent with respect to domains. The intent of this document is to provide something of an overview.

### Notification version: 

**Purpose**: cluster-level version guard for domain updates to prevent racing writes for the domains tables.
**Where it’s set**: on every domain update/create/delete it’s set via a conditional write to the following magical ‘domain’ `cadence-domain-metadata` record:

In Cassandra, this record is read like this::
```sql
SELECT notification_version FROM domains_by_name_v2 WHERE domains_partition = 0 and name = 'cadence-domain-metadata'
```

It's therefore set at the cluster level, though captured at a point in time on indivial domains (see FailoverNotificationVersion)

This is a simple monotonic counter. It goes up by \+1 every domain update for any domain in the cluster. In Cassandra, writes to the domains table are done with a conditional write to the `cadence-domain-metadata` pseudo-record as a means to guard against out-of-order updates.

In practice these values move up together accross clusters whenever there is a global domain update due to the domain's data being replicated, but these notification values are **not** comparable. For example, consider the following scenario.

### Failover Versions in each cluster:

| Cluster  | notification_version value | 
| ---------|----------------------------|
| cluster0 | 10                         | 
| cluster1 | 10                         | 

When a new global domain, replicated to both these clusters is registered, the `notification_version` will be increemented in both

| Cluster  | notification_version value | 
| ---------|----------------------------|
| cluster0 | 11                         | 
| cluster1 | 11                         | 

However, subsequently, if a 3 local domains, which are not replicated are registered in cluster1, the following will occur:

| Cluster  | notification_version value | 
| ---------|----------------------------|
| cluster0 | 11                         | 
| cluster1 | 14                         | 

### Domain FailoverVersion

**Purpose:** (in normal domains): 

- To indicate which cluster is active
- Specifies which domain updates are more recent
- Allows workflows to merge history branches with last-write-wins 

This is a somewhat more complicated counter which also indicates where it was generated (as in, which cluster generated the event). It is a monotonic counter, always going up, but the values by which it go up are incremented to indicate their origin, with something like a bitmask at the end of the counter acting as an identifier of the cluster (it's actually modulo, but it is used like a networking bitmask).

It uses the 'initial-failover-version' - an arbitrary unique int acting as a cluster identifier set a the cluster configuration level. The value changes for each failover. This is perhaps best illustrated with an example:

##### Example:

Given three clusters with the following intitial-failover-versions:
```
clusterA: 0
clusterB: 1
clusterC: 2
```

And given a `failover-increment` of `100` (this the value used to modulo the origin cluster)

And given a domain `test-domain`, available and replicated to all three clusters, 

- It is first registered (arbitrarily) to be active in clusterA, in which case the `FailoverVersion` is set to `0`. 
- If it is failed over to clusterB, then the `FailoverVersion` will become `1`.
- If it is failed back to clusterA, then it will jump up to the next increment and be `100` (which, modulo 100, gives 0, representing `clusterA`)

Since this value is attached to each workflow as it's executing and each history event as it's being written, each history event therefore can be value can therefore be modulo'd by the `failover-increment` to get its origin and given two history events in a history branch, the more recent can be picked by the value being higher (since history branches in both clusters for a given workflow may diverge during a failover briefly).

### FailoverNotificationVersion 

This is unfortunately similarly named to the Failover version, but it has a different purpose:

This is value which captures the current state of the domains at the point of processing them through the replication stack and during Graceful Failover. The values are **not** replicated and **not** comparable across clusters and its purpose is to prevent domain replication messages being replayed out of order.

**Where it’s set:** 

When a domain is replicating, in the worker that’s processing the message, it grabs the current notification version on the side that’s taking the incoming domain update message and adds it to the fields being written.


```go
// common/domain/replicationTaskExecutor.go running in the worker service as a faux singleton 
func (h *domainReplicationTaskExecutorImpl) handleDomainUpdateReplicationTask(ctx context.Context, task *types.DomainTaskAttributes) error {

	// first we need to get the current notification version since we need to it for conditional update
	metadata, err := h.domainManager.GetMetadata(ctx)
	if err != nil {
		h.logger.Error("Error getting metadata while handling replication task", tag.Error(err))
		return err
	}
	// this is the global notification_version for all domains
	notificationVersion := metadata.NotificationVersion
	
	resp, err := h.domainManager.GetDomain(ctx, &persistence.GetDomainRequest{
		Name: task.Info.GetName(),
	})

	// Where it will be set if the FailoverVersion is more recent then recently seen
	// ie, the failover event is more recent then stored in the DB
	if resp.FailoverVersion < task.GetFailoverVersion() {
		// ...
		request.FailoverNotificationVersion = notificationVersion
		// ...
	}

	// ... where it will be written to the DB
	return h.domainManager.UpdateDomain(ctx, request)
```

### Where is it used 

The failoverNotificationVersion is used as a guard to avoid out of order updates in the domain callback. 

This is running in all services, in the domain cache, and is triggered as a part of its internal refresh loop. It’s used to trigger a graceful failover trigger in history. 

### How it increments: 

- Every time there's a notification via the domain replication stack, the domains are comparing their current FailoverNotificationValue to the value (?). 

Given the following state for the domain `test35`:

| Cluster  | value | 
| ---------|-------|
| cluster0 | 19    | 
| cluster1 | 21    | 
| cluster2 | 20    | 

And the following metata values in the record `cadence-domain-metadata`

| Cluster  | value | 
| ---------|-------|
| cluster0 | 43    | 
| cluster1 | 35    | 
| cluster2 | 33    | 

After failing over domain `test35`, the expected values for the FailoverNotificationVersion in each cluster for the domain `test35` is: 

| Cluster  | value | 
| ---------|-------|
| cluster0 | 43    | 
| cluster1 | 33    | 
| cluster2 | 32    | 

And the following metata values in the record `cadence-domain-metadata`. These will have been updated by the domain data replicating

| Cluster  | value | 
| ---------|-------|
| cluster0 | 44    | 
| cluster1 | 36    | 
| cluster2 | 33    | 


### Active-Active domains and ClusterAttributes

Active-active domains split their notion of their workflows into subgroups called 'ClusterAttributes' which can be used by workflows using the domain to determine which cluster is active. They are a sub-implementation of FailoverVersion, ie the domain may have multiple ClusterAttributes, each with their own Version which functions similarly to the normal FailoverVersion.

Ie, a domain may validly have the following configuration:

For example, given three replicating clusters, with the following intitial-failover-versions
```
clusterA: 0
clusterB: 1
clusterC: 2
```

And a failover increment of 100:

For example: An active-active domain may have the following (this is a JSON representation of the domain `desc` command of a sample domain)

```
{
  "domainInfo": {
    "name": "active-active-domain",
    "status": "REGISTERED",
    "uuid": "177b38ef-ed54-4c0e-b84b-1b52e3c3598e"
  },
  "replicationConfiguration": {
    "activeClusterName": "cluster2",
    "clusters": [
      {
        "clusterName": "cluster0"
      },
      {
        "clusterName": "cluster1"
      },
      {
        "clusterName": "cluster2"
      }
    ],
    "activeClusters": {
      "attributeScopes": {
        "location": {
          "clusterAttributes": {
            "cityA": {
              "activeClusterName": "cluster0",
              "failoverVersion": 10
            },
            "cityB": {
              "activeClusterName": "cluster2",
              "failoverVersion": 12
            }
          }
        }
      }
    }
  },
  "failoverVersion": 52,
  "isGlobalDomain": true
}
```

In this instance, the domain has a 'default' active cluster (cluster2, indicated by the failoverVersion 52), as well as two cluster-attributes for the 'cities' attribute: `cityA` which is active in cluster0 and `cityB` which is active in cluster2. These can be failed over and active in clusters independently of the domain's default active cluster. 

Workflows using either of these cluster-attributes will only ever be active in one cluster at a time - that restriction remains.