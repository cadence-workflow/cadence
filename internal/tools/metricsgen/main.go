package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/fatih/structtag"
	"golang.org/x/tools/go/ast/inspector"
)

// some ugly globals for easier error reporting.
// typed panics that get caught are probably the easiest way to avoid this, while also retaining failing-line reporting.
var (
	FSET *token.FileSet
	SRC  []byte
)

// very useful for understanding the AST: https://caixw.github.io/goast-viewer/index.html
// (forked from https://yuroyoro.github.io/goast-viewer/ but with support for generics)
func main() {
	// TODO: currently this finds all "...Tags" things and assumes they are StructTags for metrics.
	// If that becomes a problem, just optionally pass a list of args == names of types to generate,
	// and ignore the rest.  Easy peasy.

	log.SetFlags(log.Lshortfile) // TODO: I really can't stand this log package, replace?
	filename := os.Getenv("GOFILE")
	FSET = token.NewFileSet()
	var err error
	SRC, err = os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	f, err := parser.ParseFile(FSET, filename, SRC, parser.SkipObjectResolution|parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	// this handles a very limited set of types and syntax, because all this generator
	// cares about is top-level declared struct types.
	//
	// I would highly suggest exploring https://caixw.github.io/goast-viewer/index.html
	// a bit to get a feel for how the AST is structured, and to see if a newly-desired
	// syntax is recognizable at the AST level.
	// if it is not, this may need to be changed to use `packages.Load` to get type info,
	// but that is DRAMATICALLY slower and I would prefer to avoid it until necessary.
	// the only tolerable way to do that is to process the whole repository in a single
	// pass, and generate everything at once.

	type named struct {
		Name string
		Node *ast.StructType
		Skip map[string]bool
	}
	var interesting []named
	// ast inspector just makes it easy to handle a stack and limited set of types without caring about depth.
	// it's not much more code than ast.Inspect tbh, but I like it.
	ins := inspector.New([]*ast.File{f})
	ins.WithStack([]ast.Node{
		(*ast.FuncDecl)(nil),  // ignore temporary in-func types
		(*ast.ValueSpec)(nil), // ignore anonymous created-for-values types (will be unnamed or in a function)

		(*ast.StructType)(nil), // check structs IFF they have a name (immediate parent node is TypeSpec with name)
	}, func(n ast.Node, push bool, stack []ast.Node) (proceed bool) {
		var st *ast.StructType
		var ts *ast.TypeSpec
		skip := map[string]bool{}
		switch n.(type) {
		case *ast.FuncDecl, *ast.ValueSpec:
			return false // can only be a func-internal or anonymous type, ignore and do not descend
		case *ast.StructType:
			st = n.(*ast.StructType)
			parent := stack[len(stack)-2]
			var ok bool
			ts, ok = parent.(*ast.TypeSpec)
			if !ok {
				// no known place this happens, so warn about it.
				//
				// anonymous types used for values do this, e.g. var f = struct{...},
				// but those should be excluded by the ValueSpec filter.
				log.Println(notify(n, "warn: expected parent node of struct, must be a typespec: %T: %#v\n", parent, parent))
				return false
			}
			decl, ok := stack[len(stack)-3].(*ast.GenDecl) // afaict always true
			if ok {
				comment := decl.Doc.Text()
				words := strings.Fields(comment) // splits on any kind of whitespace
				for _, w := range words {
					if after, ok := strings.CutPrefix(w, "skip:"); ok {
						skip[after] = true
					}
				}
			}
		}

		if ts.Name == nil || ts.Name.Name == "" || !strings.HasSuffix(ts.Name.Name, "Tags") {
			return false // uninteresting struct
		}
		interesting = append(interesting, named{ts.Name.Name, st, skip})

		// no need to descend here either.
		// we don't want nested types, and struct nodes are easy to navigate without recursion.
		return false
	})

	if len(interesting) == 0 {
		log.Fatalf("no interesting structs found in %v, bad go generate or bad definition?", filename)
	}

	// Metric-related fields must be singular (no comma-separated lists), public, and either named or embedded.
	// These fields require struct tags:
	//  - `tag:"..."` to define the tag key name (so it's easily greppable)
	//  - optionally `convert:"..."` to change how they're stringified (else primitive ints are `Itoa` and any others are untouched)

	type genfield struct {
		Name     string
		Tagname  string
		Convert  string
		Imported string
		Type     string
	}
	type embedfield struct {
		Name     string
		Imported string
		Type     string
	}
	type genable struct {
		Name     string
		Self     string
		Fields   []genfield
		Emitter  bool
		Embeds   []embedfield
		Reserved int
		Skip     map[string]bool
		NewFunc  string // if the tags struct is private, this will be "newThing", to make the constructor private.  else it is "NewThing".
	}
	var gen []genable
	for _, s := range interesting {
		var newFunc string
		if s.Name[0:1] == strings.ToLower(s.Name[0:1]) {
			// private constructor, uppercase the name so it reads nicely
			newFunc = "new" + strings.ToUpper(s.Name[0:1]) + s.Name[1:]
		} else {
			// public struct, so the constructor should be public too
			newFunc = "New" + s.Name
		}
		g := genable{
			Name:    s.Name,
			Skip:    s.Skip,
			Self:    strings.ToLower(s.Name[0:1]),
			NewFunc: newFunc,
		}
		for _, f := range s.Node.Fields.List {
			switch len(f.Names) {
			case 0: // embed
				imported, id, ascode := sourcetype(f.Type)
				if !id.IsExported() {
					log.Fatalf("cannot embed private types") // TODO: add source
				}
				if id.Name == "Emitter" {
					g.Emitter = true
					continue // emitter has special handling
				}
				// TODO: arguably a custom name / non-embed makes sense, but it seems possibly bad as it'll be less consistent

				if !strings.HasSuffix(id.Name, "Tags") {
					log.Fatal(notify(f, `embedded types must end with "Tags", as they must conform to the metrics-interface`))
				}

				g.Embeds = append(g.Embeds, embedfield{
					Name:     id.Name,
					Imported: imported,
					Type:     ascode,
				})
			case 1: // named field
				fname := f.Names[0]
				if !fname.IsExported() {
					// private field, ignore.  might have a purpose, e.g. a cache or helper func
					continue
				}
				imported, _, ascode := sourcetype(f.Type) // type can be private, e.g. `string`
				tagname := nameTag(f)                     // the tag's name is always required
				if ascode == "struct{}" {
					g.Reserved++
					// special placeholder field, ignore it.
					// these serve to give type-hints that a field *will* exist,
					// but the needs for it are too dynamic to pre-populate.
					continue
				}

				convert := convertTag(f)
				if convert == "" && isInt(f.Type) {
					convert = `fmt.Sprintf("%d", {{.}})`
				}
				if strings.Contains(convert, "{{.}}") {
					convert = strings.ReplaceAll(convert, "{{.}}", g.Self+"."+fname.Name)
				}
				if convert == "" {
					convert = g.Self + "." + fname.Name
				}
				g.Fields = append(g.Fields, genfield{
					Name:     fname.Name,
					Tagname:  tagname,
					Convert:  convert,
					Imported: imported,
					Type:     ascode,
				})
			default:
				// comma-separated, illegal
				log.Fatalf(notify(f, "cannot have comma-separated fields as struct tags must be unique"))
			}
		}
		gen = append(gen, g)
	}

	basename := strings.TrimSuffix(filename, ".go")
	outfile, err := os.OpenFile(basename+"_metrics_gen.go", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	out := outfile

	// package declaration
	_, _ = fmt.Fprintln(out, "package", os.Getenv("GOPACKAGE"))
	_, _ = fmt.Fprintln(out) // and a blank line

	// magic generated-source comment
	_, _ = fmt.Fprintln(out, "// Code generated ./internal/tools/metricsgen; DO NOT EDIT")
	_, _ = fmt.Fprintln(out)

	// copy all imports, possibly duplicating, and add fmt if needed.
	// goimports will clean it up for us.
	_, _ = fmt.Fprintln(out, "import (")

	// add common ones used in the template.
	// goimports will deduplicate and remove them if they're unused.
	imports := append(f.Imports, []*ast.ImportSpec{
		{Path: &ast.BasicLit{Value: `"fmt"`}},                                               // for strconv
		{Path: &ast.BasicLit{Value: `"time"`}},                                              // for Histogram's time.Duration
		{Path: &ast.BasicLit{Value: `"github.com/uber-go/tally"`}},                          // for tally.Buckets
		{Path: &ast.BasicLit{Value: `"github.com/uber/cadence/common/metrics/structured"`}}, // for emitter
	}...)
	for _, i := range imports {
		// i.Path.Value already has quotes
		if i.Name != nil {
			_, _ = fmt.Fprintf(out, "\t%s %s\n", i.Name, i.Path.Value)
		} else {
			_, _ = fmt.Fprintf(out, "\t%s\n", i.Path.Value)
		}
	}
	_, _ = fmt.Fprintln(out, ")")

	// generate the template
	for _, t := range gen {
		err := tmpl.Execute(out, t)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func nameTag(f *ast.Field) string {
	value := getTag(f, "tag")
	if value == "" {
		// must have tag
		log.Fatalf(notify(f, "metric tags must have an explicit `tag:\"name\"`"))
	}
	return value
}

func convertTag(f *ast.Field) string {
	value := getTag(f, "convert") // currently a single string, could be complexified
	if strings.Contains(value, "{{.}}") {
		// call method and pass the field via interpolation
	} else if value != "" {
		// malformed convert
		log.Fatalf(notify(f, "convert tags must contain a `{{.}}` substring where the field will be interpolated"))
	}
	// valid format or empty string
	return value
}

func getTag(f *ast.Field, name string) string {
	if f.Tag == nil {
		return ""
	}
	// trim off the outer ` characters, as the AST's value keeps them
	st, err := structtag.Parse(strings.Trim(f.Tag.Value, "`"))
	if err != nil {
		log.Fatal("bad tag")
	}
	t, err := st.Get(name)
	_ = err // no defined type for "tag does not exist", just use t's presence
	if t == nil {
		return ""
	}
	return t.Value()
}

func isInt(fieldtype ast.Expr) bool {
	if _, ok := fieldtype.(*ast.StarExpr); ok {
		line, source := getsource(fieldtype)
		log.Fatalf("tag-struct fields cannot be pointers\n%v %v", line, source)
		return false // unreachable
	}
	if id, ok := fieldtype.(*ast.Ident); ok {
		return id.Name == "int" || id.Name == "int32" || id.Name == "int64"
	}
	if _, ok := fieldtype.(*ast.SelectorExpr); ok {
		return false // imported type, not assumed to be int
	}
	if st, ok := fieldtype.(*ast.StructType); ok {
		if st.Fields == nil || len(st.Fields.List) == 0 {
			return false
		}
	}
	log.Fatalf(notify(fieldtype, "unknown tag-struct type %T", fieldtype))
	return false // unreachable
}

func sourcetype(f ast.Expr) (imported string, name *ast.Ident, ascode string) {
	var ident *ast.Ident
	if id, ok := f.(*ast.Ident); ok {
		ident = id
		ascode = id.Name
	} else if sel, ok := f.(*ast.SelectorExpr); ok {
		// thing.Thing == X.Sel
		ident = sel.Sel
		imported = sel.X.(*ast.Ident).Name
		ascode = imported + "." + ident.Name
	} else if st, ok := f.(*ast.StructType); ok {
		if st.Fields == nil || len(st.Fields.List) == 0 {
			return "", nil, "struct{}"
		}
	} else {
		log.Fatal(notify(f, "unknown type types, must be `Type` or `import.Type` and must not be a pointer"))
	}
	return imported, ident, ascode
}

func notify(node ast.Node, msg string, args ...interface{}) string {
	path, source := getsource(node)
	msg = fmt.Sprintf(msg, args...)
	return fmt.Sprintf("%s\n%s: %s", msg, path, source)
}

func getsource(node ast.Node) (path string, source string) {
	// seems silly that getting the source code from the AST requires holding on to the source code.
	// but eh, I suppose it's more memory-efficient?
	pos := FSET.Position(node.Pos())
	l := pos.Line
	lines := strings.Split(string(SRC), "\n")
	if l > len(lines)-1 {
		log.Fatalf("source line too large (%d > %d)", l, len(lines)-1)
	}
	// pos only contains filename, not full path
	p, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/uber/cadence")
	cmd.Stderr = os.Stderr
	st, err := cmd.Output()
	if err == nil {
		// remove the path to the main module from the absolute path, it's large and annoying
		commonpath := strings.TrimSuffix(string(st), "\n")
		p = strings.TrimPrefix(p, commonpath+"/")
	}
	return p + "/" + pos.String(), lines[l-1]
}

// $self is used instead of .Self because it gets lost when ranging (changes `.` scope)
var tmpl = template.Must(template.New("metrics").Parse(`
{{- $self := .Self }}
{{- if not .Skip.New }}
	// {{.NewFunc}} constructs a new metric-tag-holding {{.Name}}, and it must be used
	// instead of custom initialization to ensure newly added tags can be detected
	// at compile time instead of missing them at run time.
	func {{.NewFunc}}(
		{{- if .Emitter }}
			emitter structured.Emitter,
		{{- end }}
		{{- range .Embeds }}
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
		num += {{ .Reserved }} // num of reserved fields
		{{- range .Embeds }}
			num += {{$self}}.{{ .Name }}.NumTags()
		{{- end }}
		return num
	}
{{- end }}

{{- if not .Skip.PutTags }}
	// PutTags writes this set of tags (and its embedded parents) to the passed map.
	func ({{$self}} {{.Name}}) PutTags(into map[string]string) {
		{{- range .Embeds }}
			{{$self}}.{{ .Name }}.PutTags(into)
		{{- end }}
		{{- range .Fields }}
			into["{{.Tagname}}"] = {{ .Convert }} 
		{{- end }}
	}
{{- end }}

{{- if not .Skip.GetTags }}
	// GetTags is a minor helper to get a pre-allocated-and-filled map with room
	// for reserved fields (i.e. 'struct{}' type fields, which only declare intent).
	func ({{$self}} {{.Name}}) GetTags() map[string]string {
		tags := make(map[string]string, {{$self}}.NumTags())
		{{$self}}.PutTags(tags)
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
			{{$self}}.Emitter.Count({{$self}}, name, num)
		}
	{{- end }}
	
	{{- if not .Skip.Histogram }}
		// Histogram records a histogram with the specified buckets.
		//
		// Buckets should essentially ALWAYS be one of the pre-defined values in [github.com/uber/cadence/common/metrics/structured],
		// or pass nil to choose the [github.com/uber/cadence/common/metrics/structured.Default1ms10m] default value.
		func ({{$self}} {{.Name}}) Histogram(name string, buckets tally.DurationBuckets, dur time.Duration) {
			{{$self}}.Emitter.Histogram({{$self}}, name, buckets, dur)
		}
	{{- end }}

{{- end }}{{/* convenience */}}
`))
