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

var Analyzer = &analysis.Analyzer{
	Name:     "metricnames",
	Doc:      "finds calls to metrics-emitting methods on Emitter and ...Tags types, and reports the inline metric names for further analysis",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// Target method names we're looking for
var targetMethods = map[string]bool{
	"Histogram":    true,
	"IntHistogram": true,
	"Count":        true,
	"Gauge":        true,
}

const structuredEmitterPath = "github.com/uber/cadence/common/metrics/structured"

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.File)(nil),     // to skip test files
		(*ast.FuncDecl)(nil), // to skip test helper funcs
		(*ast.CallExpr)(nil),
	}
	inspect.Nodes(nodeFilter, func(n ast.Node, push bool) (proceed bool) {
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
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		// check if the method name is one we care about
		methodName := sel.Sel.Name
		if !targetMethods[methodName] {
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
			pass.Reportf(sel.Pos(), "non-pointer anonymous receiver of a method call should be impossible?")
			return
		}
		if !isTypeMetricsRelated(named) {
			return
		}

		// at this point we know that it's "a metrics method", make sure it's valid.

		if isPointerReceiver {
			// could be allowed with a bit more .Elem() dereferencing in this analyzer,
			// but currently blocked to intentionally force value types.
			pass.Reportf(sel.Pos(), "pointer receivers are not allowed on metrics emission calls: %v", types.ExprString(sel))
			return
		}
		if len(call.Args) == 0 {
			// no args == method-name conflict that we shouldn't allow
			pass.Reportf(call.Pos(), "method call %v looks like a metrics method, but has no args.  this is not currently allowed", call)
			return
		}

		// pull out the first arg, and make sure it's a constant inline string.
		//
		// both emitters and generated convenience methods use the metric name
		// as the first argument, because this means we can avoid having to
		// deal with resolving embedded methods to figure out a different index.
		//
		// embedded methods require traversing a MethodSet's result, re-building
		// the resolution chain to find the actual receiver type.
		// this is possible, but not worth the amount of code it requires
		// (around 100 lines with error handling and basic docs for our very
		// simple "only values, only structs" requirements).
		nameArg := call.Args[0]
		metricName, isConstant := getConstantString(nameArg)
		if !isConstant {
			pass.Reportf(nameArg.Pos(), "metric names must be in-line strings, not consts or vars: %v", nameArg)
			return
		}

		// valid call!
		//
		// "report" the data we've found so it can be checked with a bit of bash.
		// error if any non-"success" lines were found, else check that the list is unique.
		//
		// tests may lead to the same package being processed more than once, but
		// the reported lines for "path/to/file:line success: {name}" are still unique.
		pass.Reportf(call.Pos(), "success: %s", metricName)
		return
	})

	return nil, nil
}

func isTypeMetricsRelated(t *types.Named) bool {
	pkg, name := getPkgAndName(t)
	if pkg == structuredEmitterPath && name == "Emitter" {
		return true
	}
	if (pkg == "github.com/uber/cadence" || strings.HasPrefix(pkg, "github.com/uber/cadence/")) && strings.HasSuffix(name, "Tags") {
		return true
	}
	// some uninteresting type
	return false
}

func getPkgAndName(named *types.Named) (pkgPath, name string) {
	obj := named.Obj()
	if obj == nil {
		// afaik not possible, this would be "a named type without a type"
		return "", ""
	}
	pkg := obj.Pkg()
	if pkg == nil {
		// given where this is used, I believe this would currently imply
		// "a builtin type with a method", which does not exist.
		// but it's fine to return partial values, in case it's used in more places.
		return "", obj.Name()
	}
	return pkg.Path(), obj.Name()
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
