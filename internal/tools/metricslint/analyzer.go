package metricslint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	scopePkg      = "github.com/uber/cadence/common/metrics"
	scopeTypeName = scopePkg + ".Scope"
)

var scopeMethods = map[string]bool{
	"ExponentialHistogram":    true,
	"IntExponentialHistogram": true,
	"IncCounter":              true,
	"AddCounter":              true,
	"UpdateGauge":             true,
	"StartTimer":              true,
	"RecordTimer":             true,
	"RecordHistogramDuration": true,
	"RecordHistogramValue":    true,
}

// Analyzer reports all (inline string) metric names passed to Emitter methods or defined in metrics/defs.go.
//
// This is NOT intended to be used directly, it's just using the analysis framework to simplify type checking.
// The output needs to be checked for duplicates or error messages by a separate process (see cmd/main.go).
var Analyzer = &analysis.Analyzer{
	Name:     "metricnames",
	Doc:      "finds metricDefinition defs and uses, and reports them so they can be post-processed for uniqueness",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	i := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	if pass.Pkg.Path() == scopePkg {
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
	}

	// print calls to metrics-emitting methods on Emitter, regardless of location
	reportScopeCalls(pass, i)

	return nil, nil
}

func reportMetricDefinitionNames(pass *analysis.Pass, i *inspector.Inspector) {
	nodeFilter := []ast.Node{
		(*ast.CompositeLit)(nil),
	}
	i.Preorder(nodeFilter, func(n ast.Node) {
		decl := n.(*ast.CompositeLit)
		var metricNameField *ast.KeyValueExpr
		var histogramType string
		for _, el := range decl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				return // not a struct initializer
			}
			ident, ok := kv.Key.(*ast.Ident)
			if !ok {
				return // dynamic key in a dict or something, ignore
			}
			switch ident.Name {
			case "metricName":
				metricNameField = kv // currently this is exclusively used by metricDefinition
			case "exponentialHistogram":
				if histogramType != "" {
					pass.Reportf(kv.Pos(), "only one of exponentialHistogram or intExponentialHistogram can be set")
				}
				histogramType = "duration"
			case "intExponentialHistogram":
				if histogramType != "" {
					pass.Reportf(kv.Pos(), "only one of exponentialHistogram or intExponentialHistogram can be set")
				}
				histogramType = "int"
			}
		}
		if metricNameField == nil {
			return
		}
		str, ok := getConstantString(metricNameField.Value)
		if !ok {
			pass.Reportf(metricNameField.Pos(), "metricDefinition{metricName: ...} value must be an inline string literal")
			return
		}
		switch histogramType {
		case "duration":
			if !strings.HasSuffix(str, "_ns") {
				pass.Reportf(metricNameField.Pos(), "exponential histogram metric names must end with _ns: %q", str)
				return
			}
		case "int":
			if !strings.HasSuffix(str, "_counts") {
				pass.Reportf(metricNameField.Pos(), "int-exponential histogram metric names must end with _counts: %q", str)
				return
			}
		}
		pass.Reportf(metricNameField.Pos(), "success: %v", str)
	})
}

func reportScopeCalls(pass *analysis.Pass, i *inspector.Inspector) {
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
		if !scopeMethods[methodName] {
			return
		}

		// check if the receiver type is one we care about
		receiver := pass.TypesInfo.TypeOf(sel.X)
		// pointer receivers are not allowed, complain if found.
		// if we just ignore them here, we might miss a metric call with a duplicate name.
		named, ok := receiver.(*types.Named)
		if !ok {
			if _, isPtr := receiver.(*types.Pointer); isPtr {
				// ignore pointer use, e.g. this occurs on the impl of scope itself
				return
			}
			// not named and not pointer == allowed by the type system, but should be impossible in a CallExpr
			pass.Reportf(sel.Pos(), "anonymous receiver of a method call should be impossible?")
			return
		}
		fullName := getPkgAndName(named)
		if fullName != scopeTypeName {
			return // apparently not the scope, it just has similarly-named methods.  ignore it.
		}

		// at this point we know that it's "a metrics method", make sure it's valid.

		if len(call.Args) == 0 {
			// 0 args == not currently in use
			pass.Reportf(call.Pos(), "method call %v looks like a metrics method, but has too few args", call)
			return
		}

		filename := pass.Fset.Position(call.Pos()).Filename
		if strings.HasSuffix(filename, "common/util.go") {
			// util function that checks key lengths
			return
		} else if strings.HasSuffix(filename, "service/worker/archiver/replay_metrics_client.go") {
			// a mock, ignore the metric-ID variable use
			return
		} else if strings.HasSuffix(filename, "service/history/workflowcache/cache.go") {
			// reasonable usage, internal+external passed in.  maybe worth changing later.
			return
		} else if strings.HasSuffix(filename, "service/history/workflowcache/metrics.go") {
			// reasonable usage, internal+external passed in.  maybe worth changing later.
			return
		}

		// pull out the first arg's text and report it for duplicate-checking
		nameArg := call.Args[0]
		nameArgText := types.ExprString(nameArg) // reconstructs the expression, but should be fine
		if !strings.HasPrefix(nameArgText, "metrics.") {
			pass.Reportf(nameArg.Pos(), "first argument to %v must be a metrics constant, not %v", methodName, nameArgText)
			return
		}

		// valid call!
		//
		// "report" the data we've found so it can be checked with a separate process (see cmd/main.go).
		// that will error if any non-"success" lines were found, else check that the list is unique.
		//
		// tests may lead to the same package being processed more than once, but
		// the reported lines for "path/to/file:line success: {name}" are still unique
		// and that won't count as a duplicate.
		pass.Reportf(call.Pos(), "success: %s", nameArgText)
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
