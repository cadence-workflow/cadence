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
	log.SetFlags(log.Lshortfile) // TODO: I really can't stand this log package
	filename := os.Getenv("GOFILE")
	FSET = token.NewFileSet()
	var err error
	SRC, err = os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	f, err := parser.ParseFile(FSET, filename, SRC, parser.SkipObjectResolution)
	if err != nil {
		log.Fatal(err)
	}

	var needsFmt bool

	// handles a very limited set of types and syntax, because all this generator
	// cares about is top-level declared struct types.
	//
	// I would highly suggest exploring https://caixw.github.io/goast-viewer/index.html
	// a bit to get a feel for how the AST is structured, and to see if a newly-desired
	// syntax is recognizable at the AST level.
	// if it is not, this may need to be changed to use `packages.Load` to get type info,
	// but that is DRAMATICALLY slower and I would prefer to avoid it until necessary.

	type named struct {
		Name string
		Node *ast.StructType
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
		}

		if ts.Name == nil || ts.Name.Name == "" || !strings.HasSuffix(ts.Name.Name, "Tags") {
			return false // uninteresting struct
		}
		interesting = append(interesting, named{ts.Name.Name, st})

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
		Fields   []genfield
		Embeds   []embedfield
		Reserved int
	}
	var gen []genable
	for _, s := range interesting {
		g := genable{
			Name: s.Name,
		}
		for _, f := range s.Node.Fields.List {
			switch len(f.Names) {
			case 0: // embed
				imported, id, ascode := sourcetype(f.Type)
				if !id.IsExported() {
					log.Fatalf("cannot embed private types") // TODO: add source
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
				if strings.HasPrefix(convert, ".") {
					// append it to the field access
					convert = "self." + fname.Name + convert
				}
				if convert == "" && isInt(f.Type) {
					needsFmt = true
					convert = `fmt.Sprintf("%d", {{.}})`
				}
				if strings.Contains(convert, "{{.}}") {
					convert = strings.ReplaceAll(convert, "{{.}}", "self."+fname.Name)
				}
				if convert == "" {
					convert = "self." + fname.Name
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
		if len(g.Fields) > 0 {
			gen = append(gen, g)
		} else {
			log.Fatalf("nothing to gen? %#v", g)
		}
	}

	basename := strings.TrimSuffix(filename, ".go")
	outfile, err := os.OpenFile(basename+"_gen.go", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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
	for _, i := range f.Imports {
		// i.Path.Value already has quotes
		if i.Name != nil {
			_, _ = fmt.Fprintf(out, "\t%s %s\n", i.Name, i.Path.Value)
		} else {
			_, _ = fmt.Fprintf(out, "\t%s\n", i.Path.Value)
		}
	}
	if needsFmt {
		_, _ = fmt.Fprintf(out, "\t%q\n", "fmt")
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
	if strings.HasPrefix(value, ".") {
		// call method on the thing
	} else if strings.Contains(value, "{{.}}") {
		// call method and pass the field via interpolation
	} else if value != "" {
		// malformed convert
		log.Fatalf(notify(f, "convert tags must be either `.Method()` (leading `.`, appended to the field access) "+
			"or `someFunc({{.}})` (acts like a template, but not fully executed as one) to define how to stringify"))
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

/*
Sample:

	func NewShardTasklistTags(ServiceTags ServiceTags, Shard int, Type TasklistType, ) ShardTasklistTags {
	        res := ShardTasklistTags{
	                Shard: Shard,
	                Type: Type,
	        }
	        return res
	}

	func (self ShardTasklistTags) NumTags() int {
	        num := 2 // num of self fields
	        num += self.ServiceTags.NumTags()
	        return num
	}

	func (self ShardTasklistTags) Tags(into map[string]string) {
	        self.ServiceTags.Tags(into)
	        into["shard"] = fmt.Sprintf("%d", self.Shard)
	        into["tasklist_type"] = self.Type.String()
	}
*/
var tmpl = template.Must(template.New("metrics").Parse(`
func New{{.Name}}(
	{{- range .Embeds }}
		{{ .Name }} {{ .Type }},
	{{- end }}
	{{- range .Fields }}
		{{ .Name }} {{ .Type }},
	{{- end }}
) {{ .Name }} {
	res := {{ .Name }}{
		{{- range .Embeds }}
			{{ .Name }}: {{ .Name }},
		{{- end }}
		{{- range .Fields }}
			{{ .Name }}: {{ .Name }},
		{{- end }}
	}
	return res
}

func (self {{.Name}}) NumTags() int {
	num := {{ .Fields | len }} // num of self fields 
	num += {{ .Reserved }} // num of reserved fields
	{{- range .Embeds }}
		num += self.{{ .Name }}.NumTags()
	{{- end }}
	return num
}

func (self {{.Name}}) Tags(into map[string]string) {
	{{- range .Embeds }}
	self.{{ .Name }}.Tags(into)
	{{- end }}
	{{- range .Fields }}
	into["{{.Tagname}}"] = {{ .Convert }} 
	{{- end }}
}
`))
