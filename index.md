---
title: Welcome
layout: marketing
callout: Open Source, Scalable, Durable Workflows
callout_description: Develop resilient long-running business applications with straightforward code
---

<section class="features">
  <div class="feature dynamic-execution">
    <a href="{{ '/docs/02_key_features#dynamic-workflow-execution-graphs' | relative_url }}">
      <div class="icon">
        <svg class="icon-arrow-divert">
          <use xlink:href="#icon-arrow-divert"></use>
        </svg>
      </div>
    </a>
    <a href="{{ '/docs/02_key_features#dynamic-workflow-execution-graphs' | relative_url }}">
      <span class="description">Dynamic Workflow Execution Graphs</span>
    </a>
    <p>Determine the workflow execution graphs at runtime based on the data you are processing</p>
  </div>

  <div class="feature child-workflows">
    <a href="{{ '/docs/03_goclient/05_child_workflows' | relative_url }}">
      <div class="icon">
        <svg class="icon-person-unaccompanied-minor">
          <use xlink:href="#icon-person-unaccompanied-minor"></use>
        </svg>
      </div>
    </a>
    <a href="{{ '/docs/03_goclient/05_child_workflows' | relative_url }}">
      <span class="description">Child Workflows</span>
    </a>
    <p>Execute other workflows and receive results upon completion</p>
  </div>

  <div class="feature timers">
    <a href="{{ '/docs/02_key_features#durable-timers' | relative_url }}">
      <div class="icon">
        <svg class="icon-stopwatch">
          <use xlink:href="#icon-stopwatch"></use>
        </svg>
      </div>
    </a>
    <a href="{{ '/docs/02_key_features#durable-timers' | relative_url }}">
      <span class="description">Durable Timers</span>
    </a>
    <p>Persisted timers are robust to worker failures</p>
  </div>

  <div class="feature signals">
    <a href="{{ '/docs/03_goclient/08_signals' | relative_url }}">
      <div class="icon">
        <svg class="icon-signal">
          <use xlink:href="#icon-signal"></use>
        </svg>
      </div>
    </a>
    <a href="{{ '/docs/03_goclient/08_signals' | relative_url }}">
      <span class="description">Signals</span>
    </a>
    <p>Influence workflow execution path by sending data directly using a signal</p>
  </div>

  <div class="feature at-most-once">
    <a href="{{ '/docs/02_key_features#at-most-once-activity-execution' | relative_url }}">
      <div class="icon">
        <svg class="icon-umbrella">
          <use xlink:href="#icon-umbrella"></use>
        </svg>
      </div>
    </a>
    <a href="{{ '/docs/02_key_features#at-most-once-activity-execution' | relative_url }}">
      <span class="description">At-Most-Once Activity Execution</span>
    </a>
    <p>Activities need not be idempotent</p>
  </div>

  <div class="feature heartbeating">
    <a href="{{ '/docs/03_goclient/03_activities#heartbeating' | relative_url }}">
      <div class="icon">
        <svg class="icon-heart">
          <use xlink:href="#icon-heart"></use>
        </svg>
      </div>
    </a>
    <a href="{{ '/docs/03_goclient/03_activities#heartbeating' | relative_url }}">
      <span class="description">Activity Heartbeating</span>
    </a>
    <p>Detect failures and track progress in long-running activities</p>
  </div>

</section>
