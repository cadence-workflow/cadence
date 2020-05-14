---
layout: default
title: Synchronous query
permalink: /docs/concepts/queries
---

# Synchronous query

:workflow:Workflow: code is stateful with the Cadence framework preserving it over various software and hardware failures. The state is constantly mutated during :workflow_execution:. To expose this internal state to the external world Cadence provides a synchronous :query: feature. From the :workflow: implementer point of view the :query: is exposed as a synchronous callback that is invoked by external entities. Multiple such callbacks can be provided per :workflow: type exposing different information to different external systems.

To execute a :query: an external client calls a synchronous Cadence API providing _:domain:, workflowID, :query: name_ and optional _:query: arguments_.

:query:Query: callbacks must be read-only not mutating the :workflow: state in any way. The other limitation is that the :query: callback cannot contain any blocking code. Both above limitations rule out ability to invoke :activity:activities: from the :query: handlers.

Cadence team is currently working on implementing _update_ feature that would be similar to :query: in the way it is invoked, but would support :workflow: state mutation and :local_activity: invocations.

## Stack Trace Query

The Cadence client libraries expose some predefined :query:queries: out of the box. Currently the only supported built-in :query: is _stack_trace_. This :query: returns stacks of all :workflow: owned threads. This is a great way to troubleshoot any :workflow: in production.
