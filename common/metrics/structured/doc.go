/*
Package structured contains the base objects for a struct-based metrics system.

This is intended to be used with internal/tools/metricsgen, but the Emitter is
public on StructTags to ensure ad-hoc metrics are still simple to emit (and to
make codegen reasonably easy).

For more details, check the generated code of any StructTag, or the generator
in [github.com/uber/cadence/internal/tools/metricsgen].

To add a new StructTags type, it must:
  - be named "...Tags"
  - have `//go:generate metricsgen` in the file (no arguments are needed)
  - use `tag:"name"` to declare the metrics name tag

And you may also want to:
  - embed a parent StructTags, for data to inherit
  - use e.g. `convert:"stringify({{.}})"` or `convert:"{{.}}.Method()"` to
    convert a field's type to a string for the tag value (this is a template
    string, used verbatim in the generated output)
  - use `struct{}`-typed fields to reserve tags for documentation purposes, and
    for more efficient pre-allocation of tag maps
  - skip code generation steps with "skip:Something" magic comments

---

If you need to break a chain of metrics, e.g. to hide fields, the generator can
be easily modified to allow you to include parent struct-tags as private fields.

These should still be required for construction, but will not expose their fields
or methods.  Be careful when doing this, as it will hide the data being emitted
unless you duplicate fields, but it's a simple workaround if it becomes necessary.

---

Generally speaking, you are encouraged to make structs to represent tags on
metrics, for documentation and consistency purposes:
  - Tag values and metric names can simply be inline strings to allow easier
    grep / navigating, unless there is some benefit to be had from a shared const.
  - Metrics that are "stable" (i.e. may have alerts or dashboards based on them)
    should use a "semantic" method to emit the metric rather than directly using
    `Count` or similar.  This provides a place to document the intent (and any
    relevant panels or alerts), and helps simplify emitting multiple kinds of
    metrics to represent a single event so they can be discovered together.
  - As a corollary: if you see a semantic metrics-emitting method, BE CAREFUL
    about making changes to it.  You may be impacting users and server operators,
    and likely need a gradual migration and documentation.

Anywhere this structure is not beneficial, just use the convenience methods on the
tags object to emit custom metrics.  Or even consider using the low-level Emitter
if there is no better option available - this is equivalent to the base metrics
client, but with a stricter API to encourage performance best practices.  Ad-hoc
metrics should not be prevented, they're useful for light-weight investigations
and experimental data.
*/
package structured
