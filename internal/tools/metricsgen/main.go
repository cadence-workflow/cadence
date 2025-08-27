package main

import (
	"context"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"maps"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/fatih/structtag"
)

// some ugly globals for easier error reporting
var (
	FSET    *token.FileSet
	SRC     []byte
	VERBOSE bool
)

// list of all skippable things, so mis-spellings and whatnot can be caught.
var skipNames = map[string]bool{
	"New":          true,
	"NumTags":      true,
	"PutTags":      true,
	"GetTags":      true,
	"Convenience":  true,
	"Inc":          true,
	"Count":        true,
	"Gauge":        true,
	"Histogram":    true,
	"IntHistogram": true,
}

// intermediate product, structs that should be inspected further
type namedStruct struct {
	Name string
	Node *ast.StructType
	Skip map[string]bool
}

type genField struct {
	Name      string
	MetricTag string
	Convert   string
	Imported  string
	Type      string
}
type constructorField struct {
	Name string
	Type string
}
type genEmbed struct {
	Name     string
	Imported string
	Type     string
}

// struct to generate
type gen struct {
	Name                  string
	Self                  string
	Fields                []genField
	Embeds                []genEmbed
	ConstructorOnlyFields []constructorField // private non-tag fields that need to go in the constructor
	DynamicOperationTags  bool               // requires special handling
	Emitter               bool               // requires special handling
	NumReserved           int
	Skip                  map[string]bool
	NewFunc               string // if the tags struct is private, this will be "newThing", to make the constructor private.  else it is "NewThing".
	StructuredPkg         string // "structured." anywhere outside the structured package
}

// Metricsgen is a code generator to simplify creating structured metrics based on
// the patterns in [github.com/uber/cadence/common/metrics/structured].
//
// To use, just add a `//go:generate metricsgen` in the source file with any new
// `...Tags` structs, and run `make metrics`.
// `make go-generate` will also run this implicitly, but it is far slower.
func main() {
	// I would highly suggest exploring https://caixw.github.io/goast-viewer/index.html a bit
	// (forked from https://yuroyoro.github.io/goast-viewer/ but with support for generics)
	// to get a feel for how the AST is structured, and to see if a newly-desired
	// syntax/feature is recognizable at the AST level.
	//
	// if it is not, this may need to be changed to use `packages.Load` to get type info,
	// but that is DRAMATICALLY slower and I would prefer to avoid it until necessary.
	// the only tolerable way to do that is to process the whole repository in a single
	// pass, and generate everything at once.

	// Currently this finds all "...Tags" things and assumes they are StructTags for metrics.
	// If that becomes a problem, just optionally pass a list of args == names of types to generate,
	// and ignore the rest.  Should be very easy.

	flag.BoolVar(&VERBOSE, "v", false, "verbose output, e.g. print all types found")

	log.SetFlags(log.Lshortfile) // TODO: I really can't stand this log package, replace?
	filename := os.Getenv("GOFILE")
	FSET = token.NewFileSet()
	var err error
	SRC, err = os.ReadFile(filename) // hold onto the source code so errors can show the line of text that's wrong
	if err != nil {
		log.Fatal(err)
	}
	f, err := parser.ParseFile(FSET, filename, SRC, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	interesting := findStructs(f)
	if len(interesting) == 0 {
		log.Fatalf("no interesting structs (named '...Tags') found in %v, bad go generate or bad definition?", filename)
	}

	// Metric-related fields must be singular (no comma-separated lists), public, and either named or embedded.
	// These fields require struct tags:
	//  - `tag:"..."` to define the tag key name (so it's easily greppable)
	//  - optionally `convert:"..."` to change how they're stringified (else primitive ints are `Itoa` and any others are untouched)
	structuredPkg := "structured."
	if os.Getenv("GOPACKAGE") == "structured" {
		structuredPkg = ""
	}
	toGenerate := getGenStructs(interesting, structuredPkg)

	basename := strings.TrimSuffix(filename, ".go")
	var out io.Writer // smaller interface so it can be replaced with os.Stdout for manual testing
	out, err = os.OpenFile(basename+"_metrics_gen.go", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// package declaration
	write(out, "package %v", os.Getenv("GOPACKAGE"))
	write(out, "") // and a blank line

	// magic generated-source comment
	write(out, "// Code generated ./internal/tools/metricsgen; DO NOT EDIT")
	write(out, "")

	write(out, "import (")

	// copy all imports, possibly duplicating, and add fmt if needed.
	// goimports will deduplicate and remove any as needed.
	imports := append(f.Imports, []*ast.ImportSpec{
		{Path: &ast.BasicLit{Value: `"fmt"`}},  // for strconv
		{Path: &ast.BasicLit{Value: `"time"`}}, // for Histogram's time.Duration
		// {Path: &ast.BasicLit{Value: `"github.com/uber/cadence/common/metrics"`}}, // for PutOperationTags
	}...)
	if structuredPkg != "" {
		// add the import for emitter and histogram types
		imports = append(imports, &ast.ImportSpec{Path: &ast.BasicLit{Value: `"github.com/uber/cadence/common/metrics/structured"`}})
	}
	for _, i := range imports {
		// i.Path.Value already has quotes
		if i.Name != nil {
			write(out, "\t%s %s", i.Name, i.Path.Value)
		} else {
			write(out, "\t%s", i.Path.Value)
		}
	}
	write(out, ")")

	// generate the template
	for i, t := range toGenerate {
		if i > 0 {
			write(out, "") // blank line between things
		}
		err := tmpl.Execute(out, t)
		if err != nil {
			log.Fatalf("error generating code for %v: %v", t.Name, err)
		}
	}
}

func findStructs(f *ast.File) []namedStruct {
	// ast's Doc and Comment fields are somewhat confusing and inconsistent, at
	// least as far as how humans think of it, and there are many empty and/or
	// verbose-to-gather ways to get comments.
	//
	// so this uses the CommentMap, which acts like humans think of comments:
	// all text above or on the same line are considered "comments" on a thing.
	cm := ast.NewCommentMap(FSET, f, f.Comments)

	var results []namedStruct
	// range over the top-level declarations in the file, find *Tags to generate.
	// this conveniently excludes any in-func types.
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue // only care about type declarations
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name == nil || ts.Name.Name == "" {
				continue // only care about named types
			}
			if !strings.HasSuffix(ts.Name.Name, "Tags") {
				continue // only care about type names ending in "Tags"
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok || st.Fields == nil || len(st.Fields.List) == 0 {
				// could be allowed, but seems more likely to be a mistake
				log.Fatal(invalidCodeMsg(ts, "struct %v looks like a structured ...Tags type, but it has no fields", ts.Name.Name))
				panic("unreachable")
			}

			verbosef("found struct %v", ts.Name.Name)
			found := namedStruct{
				Name: ts.Name.Name,
				Node: st,
				Skip: map[string]bool{},
			}

			// check struct comments for skip tags.
			// use `gd`, not `ts/st`, to gather all comments regardless of how
			// they are technically placed in the AST.
			for _, cg := range cm.Filter(gd).Comments() {
				for _, word := range strings.Fields(cg.Text()) { // splits on whitespace
					if toskip, ok := strings.CutPrefix(word, "skip:"); ok {
						// validate the word
						if !skipNames[toskip] {
							// it's possible to point to the exact chars in the comment that are wrong, but calculating
							// that is a bit fiddly because there isn't a convenient way to strip off comment decorator
							// chars line by line.  Text() does that for us, and the struct decl is close enough to the comment.
							log.Fatal(invalidCodeMsg(gd, "unknown skip marker %q, must be one of: %v", toskip, maps.Keys(skipNames)))
						}
						verbosef("found skip %q (comment %q) for %v", toskip, word, found.Name)
						found.Skip[toskip] = true
					}
				}
			}
			results = append(results, found)
		}
	}
	return results
}

func getGenStructs(interesting []namedStruct, structuredPkg string) []gen {
	var results []gen
	for _, s := range interesting {
		var newFunc string
		if s.Name[0:1] == strings.ToLower(s.Name[0:1]) {
			// private constructor, uppercase the name so it reads nicely
			newFunc = "new" + strings.ToUpper(s.Name[0:1]) + s.Name[1:]
		} else {
			// public struct, so the constructor should be public too
			newFunc = "New" + s.Name
		}
		g := gen{
			Name:          s.Name,
			Skip:          s.Skip,
			Self:          strings.ToLower(s.Name[0:1]),
			NewFunc:       newFunc,
			StructuredPkg: structuredPkg,
		}
		for _, f := range s.Node.Fields.List {
			switch len(f.Names) {
			case 0: // embed
				imported, id, ascode := sourceType(f.Type)
				if id.Name == "Emitter" {
					g.Emitter = true
					continue // emitter always has special handling
				}
				if id.Name == "DynamicOperationTags" {
					if ascode == "structured.DynamicOperationTags" {
						verbosef("found embedded %v, customizing codegen", ascode)
						g.DynamicOperationTags = true
						continue
					} else {
						// could be allowed, but more likely a mistake until it proves needed.
						log.Fatal(invalidCodeMsg(f, "DynamicOperationTags must be embedded as structured.DynamicOperationTags"))
					}
				}
				if !strings.HasSuffix(id.Name, "Tags") {
					log.Fatal(invalidCodeMsg(f, `embedded types must end with "Tags", as they must conform to the metrics-interface`))
				}

				g.Embeds = append(g.Embeds, genEmbed{
					Name:     id.Name,
					Imported: imported,
					Type:     ascode,
				})
			case 1: // named field
				fname := f.Names[0]
				if !fname.IsExported() {
					// private field, see if it needs to be added to the constructor but don't otherwise handle it
					_, _, ascode := sourceType(f.Type) // type can be private, e.g. `string`
					g.ConstructorOnlyFields = append(g.ConstructorOnlyFields, constructorField{
						Name: fname.Name,
						Type: ascode,
					})
					continue
				}

				// ...Tags types cannot be pointers, but others are allowed
				if strings.HasSuffix(fname.Name, "Tags") {
					if _, ok := f.Type.(*ast.StarExpr); ok {
						log.Fatal(invalidCodeMsg(f, "embedded Tags structs cannot be pointers"))
					}
				}
				imported, _, ascode := sourceType(f.Type) // type can be private, e.g. `string`
				metricTag := getNameTag(f)                // the tag's name is always required
				if ascode == "struct{}" {
					g.NumReserved++
					// special placeholder field, ignore it.
					// these serve to give type-hints that a field *will* exist,
					// but the needs for it are too dynamic to pre-populate.
					continue
				}

				convertTag := getConvertTag(f)
				if convertTag == "" {
					if isInt(f.Type) {
						convertTag = `fmt.Sprintf("%d", {{.}})`
					} else {
						convertTag = "{{.}}" // defaults to assuming the field is a string, included verbatim
					}
				}
				// run the template to get the source code to stringify this field
				convertTmpl := template.Must(template.New("").Parse(convertTag))
				var out strings.Builder
				err := convertTmpl.Execute(&out, g.Self+"."+fname.Name)
				if err != nil {
					// use the original tag, not the empty-replaced one
					log.Fatal(invalidCodeMsg(f, "bad convert tag, must be a valid text template or empty: convert:%q\nerror: %v", getConvertTag(f), err))
				}
				g.Fields = append(g.Fields, genField{
					Name:      fname.Name,
					MetricTag: metricTag,
					Convert:   out.String(),
					Imported:  imported,
					Type:      ascode,
				})
			default:
				// comma-separated, never allowed
				log.Fatal(invalidCodeMsg(f, "cannot have comma-separated fields as struct tags must be unique"))
			}
		}
		results = append(results, g)
	}
	return results
}

func verbosef(format string, args ...any) {
	if VERBOSE {
		fmt.Printf(format+"\n", args...)
	}
}

// --- tag helpers ---

func getNameTag(f *ast.Field) string {
	value := getTag(f, "tag")
	if value == "" {
		// must have tag
		log.Fatal(invalidCodeMsg(f, "metric tags must have an explicit `tag:\"name\"`"))
	}
	return value
}

func getConvertTag(f *ast.Field) string {
	value := getTag(f, "convert") // currently a single string, could be complexified
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "{{") {
		// malformed convert.
		// technically possible to use e.g. a static value or computed from other fields, but that's less likely
		// and would probably be better done with a reserved field / custom PutTags instead.
		log.Fatal(invalidCodeMsg(f, "convert tags must contain a `{{ . }}` template where the field will be interpolated"))
	}
	// valid format
	return value
}

func getTag(f *ast.Field, name string) string {
	if f.Tag == nil {
		return ""
	}
	// trim off the outer ` characters, as the AST's value keeps them
	st, err := structtag.Parse(strings.Trim(f.Tag.Value, "`"))
	if err != nil {
		log.Fatal(invalidCodeMsg(f, "bad tag format: %q, err: %v", f.Tag.Value, err))
	}
	t, err := st.Get(name)
	_ = err // no defined type for "tag does not exist", just use t's presence
	if t == nil {
		return ""
	}
	return t.Value()
}

// --- type checking helpers ---

func isInt(fieldType ast.Expr) bool {
	// sourceType already restricted to these types,
	// no need to check others exhaustively.
	switch t := fieldType.(type) {
	case *ast.StarExpr:
		return false // no reliable fallback, force custom handling
	case *ast.Ident:
		return t.Name == "int" || t.Name == "int32" || t.Name == "int64"
	case *ast.SelectorExpr:
		return false // imported types cannot be builtin ints
	case *ast.StructType:
		if t.Fields == nil || len(t.Fields.List) == 0 {
			return false // `struct{}` is allowed and not an int
		}
	}
	log.Fatal(invalidCodeMsg(fieldType, "unknown field type %T: %#v", fieldType, fieldType))
	panic("unreachable")
}

// given a field type node, returns:
//   - pkg.Type: "pkg", Type, "pkg.Type"
//   - Type: "", Type, "Type"
func sourceType(fieldType ast.Expr) (imported string, name *ast.Ident, ascode string) {
	if t, ok := fieldType.(*ast.StarExpr); ok {
		imported, name, ascode = sourceType(t.X) // recurse to get the inner type
		return imported, name, "*" + ascode      // restore the pointer
	}

	switch t := fieldType.(type) {
	case *ast.Ident: // bare identifier, no package.  local type or builtin.
		return "", t, t.Name
	case *ast.SelectorExpr: // thing.Thing == X.Sel
		if x, ok := t.X.(*ast.Ident); ok {
			imported = x.Name
			return imported, t.Sel, imported + "." + t.Sel.Name
		}
	case *ast.StructType:
		if t.Fields == nil || len(t.Fields.List) == 0 {
			return "", nil, "struct{}"
		}
	}
	log.Fatal(invalidCodeMsg(fieldType, "unknown field type %T, must be `Type` or `pkg.Type`: %#v", fieldType, fieldType))
	panic("unreachable")
}

// --- error reporting funcs, to show file:line and the contents of the line for easier troubleshooting ---

func invalidCodeMsg(node ast.Node, msg string, args ...interface{}) string {
	path, source := getSourceCodeLine(node)
	msg = fmt.Sprintf(msg, args...)
	return fmt.Sprintf("%s\n%s: %s", msg, path, source)
}

func getSourceCodeLine(node ast.Node) (path string, source string) {
	// seems silly that getting the source code from the AST requires holding on to the source code.
	// but eh, I suppose it's more memory-efficient?
	pos := FSET.Position(node.Pos())
	l := pos.Line
	lines := strings.Split(string(SRC), "\n")
	if l > len(lines)-1 {
		// should not be possible, at least without `//line` directives
		log.Fatalf("source line too large (%d > %d)", l, len(lines)-1)
	}
	// pos only contains filename, not full path
	p, err := os.Getwd()
	if err != nil {
		// bad shell location, basically
		log.Fatal("could not os.Getwd(), does your current folder still exist?", err)
	}
	// figure out the shared part of the absolute path, if possible.
	// 1s timeout is far more than is necessary locally, unless it needs to download,
	// and we don't want to wait for that because it won't show any output.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", "github.com/uber/cadence")
	cmd.Stderr = os.Stderr
	st, err := cmd.Output()
	if err == nil {
		// remove the path to the main module from the absolute path, it's large and annoying
		moduleRoot := strings.TrimSpace(string(st)) // remove the newline
		p = strings.TrimPrefix(p, moduleRoot+"/")
	} // else just give up and use the absolute path, it's long but not wrong.
	return p + "/" + pos.String(), lines[l-1]
}

// --- template helpers ---

func write(out io.Writer, format string, args ...any) {
	_, err := fmt.Fprintf(out, format+"\n", args...)
	if err != nil {
		log.Fatal("failed to write generated code to output:", err)
	}
}

// $self is used instead of .Self because it gets lost when ranging (changes `.` scope)
var tmpl = template.Must(template.New("").Parse(`
{{- $self := .Self }}
{{- if not .Skip.New }}
	// {{.NewFunc}} constructs a new metric-tag-holding {{.Name}},
	// and it must be used, instead of custom initialization to ensure newly added
	// tags can be detected at compile time instead of missing them at run time.
	func {{.NewFunc}}(
		{{- if .Emitter }}
			emitter {{.StructuredPkg}}Emitter,
		{{- end }}
		{{- range .Embeds }}
			{{ .Name }} {{ .Type }},
		{{- end }}
		{{- range .ConstructorOnlyFields }}
			{{ .Name }} {{ .Type }},
		{{- end }}
		{{- range .Fields }}
			{{ .Name }} {{ .Type }},
		{{- end }}
	) {{ .Name }} {
		{{$self}} := {{ .Name }}{
			{{- if .Emitter }}
				Emitter: emitter,
			{{- end }}
			{{- range .Embeds }}
				{{ .Name }}: {{ .Name }},
			{{- end }}
			{{- range .ConstructorOnlyFields }}
				{{ .Name }}: {{ .Name }},
			{{- end }}
			{{- range .Fields }}
				{{ .Name }}: {{ .Name }},
			{{- end }}
		}
		return {{$self}}
	}
{{- end }}

{{- if not .Skip.NumTags }}
	// NumTags returns the number of tags that are intended to be written in all
	// cases.  This will include all embedded parent tags and all reserved tags,
	// and is intended to be used to pre-allocate maps of tags.
	func ({{$self}} {{.Name}}) NumTags() int {
		num := {{ .Fields | len }} // num of self fields 
		num += {{ .NumReserved }} // num of reserved fields
		{{- range .Embeds }}
			num += {{$self}}.{{ .Name }}.NumTags()
		{{- end }}
		{{- if .DynamicOperationTags }}
			num += {{$self}}.DynamicOperationTags.NumTags()
		{{- end }}
		return num
	}
{{- end }}

{{- if not .Skip.PutTags }}
	// PutTags writes this set of tags (and its embedded parents) to the passed map.
	func ({{$self}} {{.Name}}) PutTags({{ if .DynamicOperationTags }}operation int, {{ end }}into {{.StructuredPkg}}DynamicTags) {
		{{- range .Embeds }}
			{{$self}}.{{ .Name }}.PutTags(into)
		{{- end }}
		{{- if .DynamicOperationTags }}
			{{$self}}.DynamicOperationTags.PutTags(operation, into)
		{{- end }}
		{{- range .Fields }}
			into["{{.MetricTag}}"] = {{ .Convert }} 
		{{- end }}
	}
{{- end }}

{{- if not .Skip.GetTags }}
	// GetTags is a minor helper to get a pre-allocated-and-filled map with room
	// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
	func ({{$self}} {{.Name}}) GetTags({{ if .DynamicOperationTags }}operation int, {{ end }}) {{.StructuredPkg}}DynamicTags {
		tags := make({{.StructuredPkg}}DynamicTags, {{$self}}.NumTags())
		{{$self}}.PutTags({{ if .DynamicOperationTags }}operation, {{ end }}tags)
		return tags
	}
{{- end }}

{{- if and (or .Emitter .Embeds) (not .Skip.Convenience) }}
	// convenience methods
	{{/* ---- forcing a blank line */}}
	{{- if not .Skip.Inc }}
		// Inc increments a named counter with the tags on this struct.
		func ({{$self}} {{.Name}}) Inc(name string) {
			{{$self}}.Count(name, 1)
		}
	{{- end }}
	
	{{- if not .Skip.Count }}
		// Count adds to a named counter with the tags on this struct.
		func ({{$self}} {{.Name}}) Count(name string, num int) {
			{{$self}}.Emitter.Count(name, num, {{$self}})
		}
	{{- end }}

	{{- if not .Skip.Gauge }}
		// Gauge adds to a named gauge with the tags on this struct.
		func ({{$self}} {{.Name}}) Gauge(name string, num float64) {
			{{$self}}.Emitter.Gauge(name, num, {{$self}})
		}
	{{- end }}
	
	{{- if not .Skip.Histogram }}
		// Histogram records a histogram with the specified buckets.
		//
		// Buckets should essentially ALWAYS be one of the pre-defined values in [github.com/uber/cadence/common/metrics/structured].
		func ({{$self}} {{.Name}}) Histogram(name string, buckets {{.StructuredPkg}}SubsettableBuckets, dur time.Duration) {
			{{$self}}.Emitter.Histogram(name, buckets, dur, {{$self}})
		}
	{{- end }}

	{{- if not .Skip.IntHistogram }}
		// IntHistogram records an integer-based histogram with the specified buckets.
		//
		// Buckets should essentially ALWAYS be one of the pre-defined values in [github.com/uber/cadence/common/metrics/structured].
		func ({{$self}} {{.Name}}) Histogram(name string, buckets {{.StructuredPkg}}IntSubsettableBuckets, value int) {
			{{$self}}.Emitter.IntHistogram(name, buckets, value, {{$self}})
		}
	{{- end }}
{{- end }}
`))
