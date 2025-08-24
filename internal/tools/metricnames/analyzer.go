package metricnames

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
		(*ast.File)(nil),
		(*ast.CallExpr)(nil),
	}
	inspect.Nodes(nodeFilter, func(n ast.Node, push bool) (proceed bool) {
		// always descend by default, in case the call contains a closure that emits metrics.
		// this covers the sync.Once case in the testdata, for example.
		proceed = true

		if !push {
			return // do nothing when ascending the ast tree
		}
		file, ok := n.(*ast.File)
		if ok {
			filename := pass.Fset.Position(file.Pos()).Filename
			if strings.HasSuffix(filename, "_test.go") {
				return false // don't descend into test files
			}
			return
		}

		call := n.(*ast.CallExpr)

		// check if this is a method call (receiver.method())
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}

		// check if the method name is one we care about
		methodName := sel.Sel.Name
		if !targetMethods[methodName] {
			return
		}
		receiverType := pass.TypesInfo.TypeOf(sel.X)
		if receiverType == nil {
			// packages-level funcs?
			pass.Reportf(call.Pos(), "method %s called with nil receiver type, unsure how this is possible", methodName)
			return
		}
		typeName, ok := relevantTypeName(receiverType)
		if !ok {
			return
		}

		metricNameIdx := 0 // first arg for ...Tags methods
		if typeName == "Emitter" {
			metricNameIdx = 1 // second arg for Emitter methods
		}

		if len(call.Args) <= metricNameIdx {
			pass.Reportf(call.Pos(), "method %s called with insufficient arguments, not a valid type? %v", methodName, receiverType)
			return
		}

		metricName, isConstant := getConstantString(pass, call.Args[metricNameIdx])

		if !isConstant {
			// already reported
			return
		}

		// "report" the data we've found so it can be checked with a bit of bash.
		// error if any non-"success" lines were found, else check that the list is unique.
		pass.Reportf(call.Pos(), "success: %s", metricName)
		return
	})

	return nil, nil
}

func relevantTypeName(t types.Type) (name string, relevant bool) {
	named, ok := t.(*types.Named)
	if !ok {
		return "", false
	}
	obj := named.Obj()
	if obj == nil {
		return "", false
	}
	if obj.Pkg() == nil {
		return "", false
	}
	pkg := obj.Pkg().Path()
	if pkg == structuredEmitterPath && obj.Name() == "Emitter" {
		return "Emitter", true
	}
	if strings.HasPrefix(pkg, "github.com/uber/cadence") && strings.HasSuffix(obj.Name(), "Tags") {
		return obj.Name(), true
	}
	// external package or not an Emitter/...Tags struct
	return "", false
}

func getConstantString(pass *analysis.Pass, expr ast.Expr) (str string, ok bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		pass.Reportf(expr.Pos(), "metric call with non-inline string argument: %v", expr)
		return "", false
	}
	if lit.Kind != token.STRING {
		pass.Reportf(expr.Pos(), "metric call with non-inline string argument: %v", expr)
		return "", false
	}
	return strings.Trim(lit.Value, `"`), true
}
