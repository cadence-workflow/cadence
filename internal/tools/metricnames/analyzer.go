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
		if _, ok := relevantTypeName(receiverType); !ok {
			return
		}

		// Determine if this is an Emitter-style method by checking if it comes from embedded Emitter
		// Emitter methods have Metadata as their first parameter
		metricNameIdx := 0 // default for direct ...Tags methods
		if isEmitterMethod(pass, sel, receiverType) {
			metricNameIdx = 1 // second arg for Emitter methods (after Metadata)
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

// isEmitterMethod checks if a method call is calling an Emitter method
// either directly on structured.Emitter or through embedding
func isEmitterMethod(pass *analysis.Pass, sel *ast.SelectorExpr, receiverType types.Type) bool {
	// First check if the receiver is directly structured.Emitter
	actualType := receiverType
	if ptr, ok := actualType.(*types.Pointer); ok {
		actualType = ptr.Elem()
	}

	if named, ok := actualType.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil &&
			obj.Pkg().Path() == structuredEmitterPath &&
			obj.Name() == "Emitter" {
			return true
		}
	}

	panic("TODO: dang this is complicated.  get rid of it, require direct calls?  why, go/types, why")
	// If not direct, check if it's embedded
	methodName := sel.Sel.Name
	methodSet := types.NewMethodSet(receiverType)

	for i := 0; i < methodSet.Len(); i++ {
		selection := methodSet.At(i)

		if selection.Obj().Name() != methodName {
			continue
		}
		// Check if this method is embedded (index length > 1 means it's accessed through embedding)
		indices := selection.Index()
		if len(indices) > 1 {
			// This is an embedded method - walk the embedding path to find the actual source
			currentType := receiverType

			// Follow the embedding path
			for _, index := range indices[:len(indices)-1] {
				if named, ok := currentType.(*types.Named); ok {
					underlying := named.Underlying()
					if structType, ok := underlying.(*types.Struct); ok {
						field := structType.Field(index)
						currentType = field.Type()
					}
				}
			}

			// Check if the final embedded type is structured.Emitter
			if named, ok := currentType.(*types.Named); ok {
				obj := named.Obj()
				if obj != nil && obj.Pkg() != nil &&
					obj.Pkg().Path() == structuredEmitterPath &&
					obj.Name() == "Emitter" {
					return true
				}
			}
		}
		break

	}

	return false
}
