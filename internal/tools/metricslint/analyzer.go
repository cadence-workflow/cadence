package metricslint

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	emitterPkg      = "github.com/uber/cadence/common/metrics/structured"
	emitterTypeName = emitterPkg + ".Emitter"
)

// Analyzer reports all (inline string) metric names passed to Emitter methods or defined in metrics/defs.go.
//
// This is NOT intended to be used directly, it's just using the analysis framework to simplify type checking.
// The output needs to be checked for duplicates or error messages by a separate process (see cmd/main.go).
var Analyzer = &analysis.Analyzer{
	Name:     "metricnames",
	Doc:      "finds calls to metrics-emitting methods on Emitter, and metricDefinition names, and reports them for further analysis",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// method names on the emitter that we care about, and the suffixes that are required (if any).
// method names are checked against the actual methods on the emitter type,
// but we need the list ahead of time or passes might be racing with initializing it.
var emitterMethods = map[string]map[string]bool{
	"Histogram":    {"_ns": true},     // all our durations are in nanoseconds
	"IntHistogram": {"_counts": true}, // differentiates from durations, will likely have multiple suffixes
	"Count":        nil,
	"Gauge":        nil,
	"TagsFrom":     nil,
}

// TODO: does not yet ensure deep structure is valid, required by this reflection-branch to be safe at runtime
func run(pass *analysis.Pass) (interface{}, error) {
	i := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	if pass.Pkg.Path() == "github.com/uber/cadence/common/metrics" {
		// report all hard-coded metricDefinition names, so we can make sure
		// there are no duplicates between the integers and the const strings
		// passed to the emitter.
		//
		// if you want a duplicate: no, you really don't.
		// use a different name, and once verified just swap to the old name and
		// delete the integer def.  otherwise you risk double-counting events if
		// both are emitted, and can't tell if your tags are wrong.
		//
		// if you're swapping calls in one step, also delete the old definition.
		//
		// if you really do have an exception, modify this linter to allow it.
		reportMetricDefinitionNames(pass, i)
		// and DO NOT return, in case there are emitter calls in this package too,
		// though currently that would imply an import loop.
	} else if pass.Pkg.Path() == emitterPkg {
		// read the methods on the emitter and the target methods, and make sure they're in sync.
		checkTargetNamesMap(pass)
	}

	// print calls to metrics-emitting methods on Emitter, regardless of location
	reportMetricEmitterCalls(pass, i)

	return nil, nil
}

func reportMetricDefinitionNames(pass *analysis.Pass, i *inspector.Inspector) {
	nodeFilter := []ast.Node{
		(*ast.CompositeLit)(nil),
	}
	i.Preorder(nodeFilter, func(n ast.Node) {
		decl := n.(*ast.CompositeLit)
		var metricNameField *ast.KeyValueExpr
		for _, el := range decl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				return // not a struct initializer
			}
			ident, ok := kv.Key.(*ast.Ident)
			if !ok {
				return // dynamic key in a dict or something, ignore
			}
			if ident.Name == "metricName" {
				metricNameField = kv // currently this is exclusively used by metricDefinition
			}
			break
		}
		if metricNameField == nil {
			return
		}
		str, ok := getConstantString(metricNameField.Value)
		if !ok {
			pass.Reportf(metricNameField.Pos(), "metricDefinition{metricName: ...} value must be an inline string literal")
			return
		}
		pass.Reportf(metricNameField.Pos(), "success: %v", str)
	})
}

func checkTargetNamesMap(pass *analysis.Pass) {
	named, ok := pass.Pkg.Scope().Lookup("Emitter").Type().(*types.Named)
	if !ok {
		pass.Reportf(token.NoPos, "could not find Emitter type in %v", emitterPkg)
		return
	}
	ms := types.NewMethodSet(named)
	foundTargets := make(map[string]map[string]bool, len(emitterMethods))
	maps.Copy(foundTargets, emitterMethods)
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		if !m.Obj().Exported() {
			continue // private methods are irrelevant
		}
		_, ok := emitterMethods[m.Obj().Name()]
		if !ok {
			foundTargets[m.Obj().Name()] = nil // add any missing keys to the map
		} else {
			delete(foundTargets, m.Obj().Name()) // remove any found ones from the map
		}
	}
	if len(foundTargets) != 0 {
		pass.Reportf(named.Obj().Pos(), "target methods do not match Emitter methods, diff: %v", foundTargets)
	}
}

func reportMetricEmitterCalls(pass *analysis.Pass, i *inspector.Inspector) {
	nodeFilter := []ast.Node{
		(*ast.File)(nil),     // to skip test files
		(*ast.FuncDecl)(nil), // to skip test helper funcs
		(*ast.CallExpr)(nil),
	}
	i.Nodes(nodeFilter, func(n ast.Node, push bool) (proceed bool) {
		// always descend by default, in case the call contains a closure that emits metrics.
		// this covers the sync.Once case in the testdata, for example.
		proceed = true
		if !push {
			return // do nothing when ascending the ast tree
		}

		// check for test files, ignore their content if found
		file, ok := n.(*ast.File)
		if ok {
			filename := pass.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(filename, "_test.go") {
				return false // don't inspect test files
			}
			return
		}

		// check for test helper funcs anywhere, ignore their content if found.
		// these are identified by a *testing.T as their first arg.
		if fn, ok := n.(*ast.FuncDecl); ok {
			if len(fn.Type.Params.List) > 0 {
				firstArgType := pass.TypesInfo.TypeOf(fn.Type.Params.List[0].Type)
				asStr := types.TypeString(firstArgType, nil) // "full/path/to.Type"
				if asStr == "*testing.T" {
					return false // don't inspect test helpers
				}
			}
			return
		}

		call := n.(*ast.CallExpr) // only other possibility due to nodeFilter

		// check if this is a method call (receiver.Method() == X.Sel)
		// for one of the names we care about (as an early / efficient check)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		methodName := sel.Sel.Name
		requiredSuffixes, ok := emitterMethods[methodName]
		if !ok {
			return
		}

		// check if the receiver type is one we care about
		receiver := pass.TypesInfo.TypeOf(sel.X)
		// pointer receivers are not allowed, complain if found.
		// if we just ignore them here, we might miss a metric call with a duplicate name.
		isPointerReceiver := false
		ptr, ok := receiver.(*types.Pointer)
		for ; ok; ptr, ok = receiver.(*types.Pointer) {
			isPointerReceiver = true
			receiver = ptr.Elem()
		}
		named, ok := receiver.(*types.Named)
		if !ok {
			// allowed by the type system, but should be impossible in a CallExpr
			pass.Reportf(sel.Pos(), "anonymous receiver of a method call should be impossible?")
			return
		}
		fullName := getPkgAndName(named)
		if fullName != emitterTypeName {
			return // apparently not the emitter, it just has similarly-named methods (e.g. Tally).  ignore it.
		}

		// at this point we know that it's "a metrics method", make sure it's valid.

		if isPointerReceiver {
			// could be allowed with a bit more .Elem() dereferencing in this analyzer,
			// but currently blocked to intentionally force value types.
			pass.Reportf(sel.Pos(), "pointer receivers are not allowed on metrics emission calls: %v", types.ExprString(sel))
			return
		}
		if len(call.Args) < 2 {
			// 0 or 1 arg == not currently in use
			pass.Reportf(call.Pos(), "method call %v looks like a metrics method, but has too few args", call)
			return
		}

		// pull out the first arg, and make sure it's a constant inline string.
		nameArg := call.Args[0]
		metricName, isConstant := getConstantString(nameArg)
		if !isConstant {
			pass.Reportf(nameArg.Pos(), "metric names must be in-line strings, not consts or vars: %v", nameArg)
			return
		}
		if len(requiredSuffixes) > 0 {
			matched := false
			for suffix := range requiredSuffixes {
				if strings.HasSuffix(metricName, suffix) {
					matched = true
					break
				}
			}
			if !matched {
				suffixes := make([]string, 0, len(requiredSuffixes))
				for s := range requiredSuffixes {
					suffixes = append(suffixes, fmt.Sprintf("%q", s))
				}
				slices.Sort(suffixes)
				pass.Reportf(
					nameArg.Pos(),
					"metric name %q is not valid for method %v, it must have one of the following suffixes: %v",
					metricName, methodName, strings.Join(suffixes, ", "),
				)
				return
			}
		}

		// valid call!
		//
		// "report" the data we've found so it can be checked with a separate process (see cmd/main.go).
		// that will error if any non-"success" lines were found, else check that the list is unique.
		//
		// tests may lead to the same package being processed more than once, but
		// the reported lines for "path/to/file:line success: {name}" are still unique
		// and that won't count as a duplicate.
		pass.Reportf(call.Pos(), "success: %s", metricName)
		return
	})
}

func getPkgAndName(named *types.Named) (fullName string) {
	obj := named.Obj()
	if obj == nil {
		// afaik not possible, this would be "a named type without a type"
		return ""
	}
	pkg := obj.Pkg()
	if pkg == nil {
		// given where this is used, I believe this would currently imply
		// "a builtin type with a method", which does not exist.
		// but it's fine to return partial values, in case it's used in more places.
		return obj.Name()
	}
	return pkg.Path() + "." + obj.Name()
}

func getConstantString(expr ast.Expr) (str string, ok bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return "", false
	}
	if lit.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(lit.Value, `"`), true
}
