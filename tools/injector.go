package tools

import (
	"fmt"
	"go/ast"
	"go/token"
)

var (
	profilerImports = []string{"prismProfiler github.com/geckoboard/prism/profiler"}
	sinkImports     = []string{"prismSink github.com/geckoboard/prism/profiler/sink"}
)

// Return a PatchFunc that injects our profiler instrumentation code in all
// functions that are reachable from the profile targets that the user specified.
func InjectProfiler(pathToPackage, profileDir string) PatchFunc {
	basePkg, _ := qualifiedPkgName(pathToPackage)
	pkgEntryFn := basePkg + "/main"

	return func(cgNode *CallGraphNode, fnDeclNode *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
		enterFn, leaveFn := profileMethods(cgNode.Depth)
		imports := profilerImports

		// Append our instrumentation calls to the top of the function
		fnDeclNode.List = append(
			[]ast.Stmt{
				&ast.ExprStmt{
					&ast.BasicLit{
						token.NoPos,
						token.STRING,
						fmt.Sprintf(`prismProfiler.%s("%s")`, enterFn, cgNode.Name),
					},
				},
				&ast.ExprStmt{
					&ast.BasicLit{
						token.NoPos,
						token.STRING,
						fmt.Sprintf(`defer prismProfiler.%s()`, leaveFn),
					},
				},
			},
			fnDeclNode.List...,
		)

		// If this is the base package main() we also need to apply extra hooks
		// to initialize the profiler sinks. Since we are appending to the top
		// of the function declaration, these calls will be the first and
		// last calls that get executed by the patched main()
		if cgNode.Name == pkgEntryFn {
			fnDeclNode.List = append(
				[]ast.Stmt{
					&ast.ExprStmt{
						&ast.BasicLit{
							token.NoPos,
							token.STRING,
							fmt.Sprintf("prismProfiler.Init(prismSink.NewFileSink(%q))", profileDir),
						},
					},
					&ast.ExprStmt{
						&ast.BasicLit{
							token.NoPos,
							token.STRING,
							`defer prismProfiler.Shutdown()`,
						},
					},
				},
				fnDeclNode.List...,
			)

			imports = append(imports, sinkImports...)
		}

		return true, imports
	}
}

// Return the appropriate profiler enter/exit function names depending on whether
// a profile target is a user-specified target (depth=0) or a target discovered
// by analyzing the callgraph from a user-specified target.
func profileMethods(depth int) (enterFn, leaveFn string) {
	if depth == 0 {
		return "BeginProfile", "EndProfile"
	}

	return "Enter", "Leave"
}
