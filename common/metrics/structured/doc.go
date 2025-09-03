/*
Package structured contains the base objects for a struct-based metrics system.

This is intended to be used with internal/tools/metricsgen, but the Emitter is
public on many StructTags to ensure ad-hoc metrics are still simple to emit (and
to make codegen reasonably easy).

For concrete details, check the generated code of any ...Tags structs, or the
generator in [github.com/uber/cadence/internal/tools/metricsgen].

# To make a new metrics-tag-containing struct

  - Define a `type ...Tags struct` anywhere.  These can be public or private.
  - Embed any parent ...Tags structs desired, and add any fields to store tag values
    (or declare that they will be emitted, if they are not static)
  - Add a `//go:generate metricsgen` comment to the file (if not already present)
  - Run `make metrics` to generate the supporting code

In many cases, that's likely enough.  Construct your new thing and use it:

	thing := NewYourTags(parents, and, tags) // get it from somewhere
	thing.Count("name", 1)                   // "name" must be unique within Cadence
	// or inside a method on YourTags:
	func (y YourTags) ItHappened() {
		y.Count("it_happened", 1)
	}

to emit a metric with all the associated tags.

# To add new tags to existing metrics / structs

Add the field and run `make metrics`.

This will re-generate the constructor(s), which will lead to a broken build.
Just chase build failures until you've ensured that every code path has access
to the new data you wanted to add.

# To see what tags an existing metric has

Find the name string (e.g. grep for it), open it in an IDE, and just ask the
IDE to auto-complete a field access:

	yourTagsInstance.<ctrl-space to request autocomplete>

In Goland, VSCode, and likely elsewhere, this will give you a drop-down of all
fields inherited from all parents, for easy reading.

# Best practices

Use constant, in-line strings for metric names.  Prometheus requires that each
"name" must have a stable set of tags, so there is no safety benefit to using a
const - generally speaking it must NOT be shared.

Ad-hoc metrics are encouraged to use the convenience methods for simplicity.
When curious about something, just emit a metric and find out later (but watch
out for cardinality).

Avoid pointers, both for the ...Tags struct and its values, to prevent mutation.
This also implies you should generally use "simple" and minimal field types, as
they will be copied repeatedly - avoid e.g. complex thrift objects.  Hopefully
this will end up being nicer to the garbage collector than pointers everywhere.

For any metrics (or "events" which have multiple metrics) you consider "stable"
or have alerts or dashboards based on, strongly consider declaring a method on
your ...Tags struct and emitting in there.  This helps inform reviewers that
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
		tags := s.GetTags()                          // get all static tags
		tags["reserved"] = fmt.Sprint(rand.Intn(10)) // add the reserved one(s)
		s.Emitter.Count("it_happened", times, tags)  // use the lower-level Emitter
	}
*/
package structured
