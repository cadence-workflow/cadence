# Grafana V2 Dashboards (Scenes v2)

## Status

**⚠️ Work in Progress**

The v2 dashboards are currently under active development and are not yet complete. They use Grafana Scenes v2 format (`apiVersion: dashboard.grafana.app/v2`) and will eventually replace the existing dashboards in the long term.

## Design Principles

### 1. One Metric Per Query (GCM Compatibility)

**DO NOT query multiple metrics in a single PromQL query.**

✅ **Correct - Split into separate queries:**
```promql
# Query A
sum(rate(cache_requests{...}[$__rate_interval]))

# Query B
sum(rate(cache_errors{...}[$__rate_interval]))

# Query C
sum(rate(cache_miss{...}[$__rate_interval]))
```

❌ **Incorrect - Multiple metrics in one query:**
```promql
sum by (__name__) (rate({__name__=~"cache_requests|cache_errors|cache_miss", ...}[$__rate_interval]))
```

**Rationale:**
- Google Cloud Monitoring (GCM) requires one metric per query
- Multi-metric queries using `__name__` selectors are not portable
- If you find yourself needing to query multiple metrics together, **this indicates a problem with how we emit metrics in the code** - fix the metric emission instead

### 2. Query Variables Restrictions

**Query variables are limited to `label_values()` queries only.**

This is a compatibility requirement for Google Cloud Monitoring integration.

✅ **Allowed:**
```promql
label_values(cadence_requests{cadence_service="cadence-frontend"}, deployment)
label_values(cadence_requests{deployment="$deployment"}, domain)
```

❌ **Not allowed:**
- Complex metric queries
- Aggregations in variables
- Multi-metric variable queries

### 3. Datasource Portability

All datasource references use **type-based** references instead of hardcoded UIDs:

```json
{
  "datasource": {
    "type": "prometheus"
  }
}
```

This makes dashboards portable across different Grafana instances and enables the "Share dashboard with another instance" feature.

## Dashboard Files

### Complete Dashboards
- `cadence-frontend-v2.json` - Frontend service monitoring
- `cadence-history-v2.json` - History service monitoring
- `cadence-matching-v2.json` - Matching service monitoring
- `cadence-persistence-v2.json` - Persistence layer monitoring
- `cadence-overall-v2.json` - Overall system monitoring

### Known Issues

**Some panels are currently broken:**
- **Replication-related panels** - Need investigation and fixes
- **Cross-cluster metrics** - May require metric emission changes
- Other panels may show incorrect data or fail to load

## Migration Notes

### From M3QL to PromQL

The v2 dashboards have been converted from M3 Query Language (M3QL) to Prometheus Query Language (PromQL):

- **Rate calculations:** Use `rate(metric[$__rate_interval:$__interval])` for counters
- **Histogram quantiles:** Use `histogram_quantile(0.99, sum by (le) (rate(metric_bucket{...}[$__rate_interval])))`
- **Label filters:** Use `cadence_service=~"cadence[-_]frontend"` to match both hyphenated and underscored formats
- **Operations:** Use PascalCase (e.g., `RespondDecisionTaskCompleted`, `PollForActivityTask`)

### Removed Features

The following features from v1 dashboards have been removed in v2:

- **Rollup queries** - Simplified to single metric queries
- **Zone variable** - Removed (not used)
- **Latency variable** - Removed (hardcoded percentiles instead)
- **latency_hist variable** - Removed (use p99/0.99 directly)
- **topN variable** - Removed (hardcoded to 20)

### Dashboard Variables

**Remaining variables:**

- **Overall dashboard:** `deployment`, `summarize`
- **Frontend dashboard:** `deployment`, `domain`, `tasklist`
- **History dashboard:** `deployment`, `summarize`, `shard`, `procedure`
- **Matching dashboard:** `deployment`, `domain`, `tasklist`, `procedure`
- **Persistence dashboard:** `deployment`, `service`, `operation`

## Contributing

When adding or modifying panels:

1. **Split multi-metric queries** into separate queries (one metric each)
2. **Use type-based datasource references** (`{"type": "prometheus"}`)
3. **Test with Google Cloud Monitoring** if available
4. **Avoid `label_replace`** - use native metric names instead
5. **Follow naming conventions:**
   - Services: `cadence_service=~"cadence[-_]<service>"`
   - Aggregations: Explicit `sum by (label1, label2)`

## Future Work

- Fix broken replication panels
- Complete conversion of remaining TODO panels
- Validate all queries against Google Cloud Monitoring
- Add automated tests for dashboard JSON structure
- Document panel-by-panel conversion status

## Support

For issues or questions about the v2 dashboards:
1. Check the conversion documentation in the `.md` files in this directory
2. File an issue if you find broken panels or conversion errors
