/*
Package structured contains the base objects for a struct-based metrics system.

This is intended to be used with internal/tools/metricsgen, but the Emitter is
public on many StructTags to ensure ad-hoc metrics are still simple to emit (and
to make codegen reasonably easy).

For concrete details, check the generated code of any StructTag, or the generator
in [github.com/uber/cadence/internal/tools/metricsgen].

# To make a new metrics-tag-containing struct

  - Define a `type ...Tags struct` anywhere.  These can be public or private.
  - Embed any parent ...Tags structs desired, and add any fields to store tag values
    (or declare that they will be emitted, if they are not static)
  - Add a `//go:generate metricsgen` comment to the file (if not already present)
  - Run `make metrics` to generate the supporting code

In many cases, that's likely enough.  Construct your new thing and use it:

	thing.Count("name", 1) // "name" must be unique within Cadence

to emit a metric with all the associated tags.  For ad-hoc / "unstable" metrics
that are not worth documenting, that's all you need to do.

If the metric you're emitting should be considered "stable" (i.e. it may have
alerts or dashboards based on it), strongly consider moving the metrics to a
method on your ...Tags struct.  This helps imply stability during reviews, and
provides a place to document the intent / current uses of the metrics.

# To add new tags to existing metrics / structs

Add the field and run `make metrics`.

This will re-generate the constructor(s), which will lead to a broken build.
Just chase build failures until you've ensured that every code path has access
to the new data you wanted to add.

# To see what tags an existing metric has

Find the name string (e.g. grep for it), open it in an IDE, and just ask the
IDE to auto-complete a field access:

	yourTagsInstance.<ctrl-space to request autocomplete>

In Goland and VSCode, this will give you a drop-down of all fields inherited
from all parents, for easy reading.

# Best practices

Use constant, in-line strings for metric names.  Prometheus requires that each
"name" must have a stable set of tags, so there is no safety benefit to using a
const - generally speaking it must NOT be shared.

Ad-hoc metrics are encouraged to use the convenience methods for simplicity.
When in doubt, just emit a metric and find out later (just watch out for
cardinality).

Avoid pointers, both for the ...Tags struct and its values, to prevent mutation.
This also implies you should generally use "simple" and minimal field types, as
they will be copied repeatedly - avoid e.g. complex thrift objects.  Hopefully
this will end up being nicer to the garbage collector than pointers everywhere.

For any metrics (or "events" which have multiple metrics) you consider "stable"
or have alerts or dashboards based on them, strongly consider declaring a method
on your ...Tags struct and emitting in there.  This helps inform reviewers that
changing the metrics might cause problems elsewhere, and documents intent for
Cadence operators if they get an alert or see strange numbers.

# Code generation customization

Fields have two major options available: they can declare a custom to-string
conversion, and they can "reserve" a tag without defining a value:

	type YourTags struct {
		Fancy protobuf.Thing `tag:"fancy" convert:"{{.}}.String()"`
		Reserved struct{} `tag:"reserved"`
	}

Custom conversion is just a text/template string, where `.` will be filled in
with the field access (i.e. `y.Fancy`).  Strings work automatically, and
integers (int, int32, and int64) will be automatically `strconv.Itoa`-converted,
but all other types will require custom conversion.  As you cannot declare new
imports in this string, make sure you've imported any packages you need to
stringify a value in the same file as the ...Tags is declared.

Reserved tags serve two purposes:
  - They document that a tag will be emitted, so it can be discovered
  - They reserve space in the map returned by `GetTags()`, so you can
    efficiently add it at runtime

Because reserved tags will not be filled in by convenience methods like `Count`,
they are almost exclusively useful for methods that emit specific metrics.

For the simplest use cases, use a method on the ...Tags struct and add the tags
by hand:

	func (s SomethingTags) ItHappened(times int) {
		tags := structured.DynamicTags(s.GetTags())  // get all static tags
		tags["reserved"] = fmt.Sprint(rand.Intn(10)) // add the reserved one(s)
		s.Emitter.Count(tags, "it_happened", times)  // use the lower-level Emitter funcs
	}

# Accidental collision prevention

If we are concerned about accidental name collisions, it's fairly easy to build
an Analyzer that finds all calls and checks the in-line strings (export package
facts upward, check for dups each time), or prints all calls to be checked with
a simple `sort | uniq -c | grep -v 1`.

This is VASTLY easier to build when the strings involved are inline constants,
and it also means grepping for metrics is trivial because the whole "name"
exists exactly where the metric is emitted.
(this also works for metric tags, just grep for `tag:"..."` and find usages)

Or just namespace everything by the thing doing stuff that causes metrics.
Our metric names right now are extremely short, which works fine for Tally but
not for Prometheus, and is not ideal for grep either.
*/
package structured
