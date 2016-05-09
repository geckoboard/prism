package inject

import (
	"fmt"
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/ast/astutil"
)

const (
	profilerPackage = "github.com/geckoboard/prism/profiler"
)

type profilerInjector struct {
	parsedFile   *parsedFile
	injector     *Injector
	mainFunction string
}

// Inject profiler hooks to any targets present in this parsed file. If any
// hooks are applied, the injector will also add an import for the profiler package.
func injectProfiler(parsedFile *parsedFile, injector *Injector) {
	pkgBase, _ := qualifiedPkgName(injector.pkgRootPath)
	fqMainFunction := pkgBase + "/main"

	pi := &profilerInjector{
		parsedFile:   parsedFile,
		injector:     injector,
		mainFunction: fqMainFunction,
	}

	ast.Walk(pi, pi.parsedFile.astFile)
	if parsedFile.touched {
		astutil.AddImport(parsedFile.fset, parsedFile.astFile, profilerPackage)
	}
}

// Visit ast nodes and inject hooks to target function declarations.
// Implements ast.Visitor
func (in *profilerInjector) Visit(node ast.Node) ast.Visitor {
	fnDecl, isValid := node.(*ast.FuncDecl)
	if isValid {
		fqName := astFnName(fnDecl, in.parsedFile.pkgName)

		// If we hit the main() declaration inject our shutdown hooks.
		// Since hooks are prepended to the statement body we need to
		// use a defer block to ensure that the shutdown hooks are
		// always executed last if main is also one of our hook targets.
		if fqName == in.mainFunction {
			defer func() {
				in.hookMain(fnDecl.Body)
				in.parsedFile.touched = true
			}()
		}

		// Check if we need to hook this function. If the function does
		// not need to be hooked or we have already hooked it before
		// we do not need to visit it. We also ignore forward function
		// declarations.
		target, isTarget := in.injector.hookedFuncs[fqName]
		if !isTarget || target.Processed || fnDecl.Body == nil {
			return nil
		}

		in.hook(fqName, target, fnDecl.Body)

		// Flag this function as hooked
		target.Processed = true
		in.parsedFile.touched = true

		return nil
	}

	return in
}

// Register profiler init/shutdown hooks in main function declaration.
func (in *profilerInjector) hookMain(mainBody *ast.BlockStmt) {
	mainBody.List = append(
		[]ast.Stmt{
			&ast.ExprStmt{
				&ast.BasicLit{
					token.NoPos,
					token.STRING,
					`profiler.Init()`,
				},
			},
			&ast.ExprStmt{
				&ast.BasicLit{
					token.NoPos,
					token.STRING,
					`defer profiler.Shutdown()`,
				},
			},
		},
		mainBody.List...,
	)
}

// Inject profiler hook to target function body.
func (in *profilerInjector) hook(fqName string, target *Target, fnBody *ast.BlockStmt) {
	// Select the appropriate profiler methods to call depending on whether
	// this target is a root entry point (depth=0) or a direct/indirect
	// target of a root
	var enterFn = "Enter"
	var leaveFn = "Leave"
	if target.Depth == 0 {
		enterFn = "BeginProfile"
		leaveFn = "EndProfile"
	}

	fnBody.List = append(
		[]ast.Stmt{
			&ast.ExprStmt{
				&ast.BasicLit{
					token.NoPos,
					token.STRING,
					fmt.Sprintf(`profiler.%s("%s")`, enterFn, fqName),
				},
			},
			&ast.ExprStmt{
				&ast.BasicLit{
					token.NoPos,
					token.STRING,
					fmt.Sprintf(`defer profiler.%s()`, leaveFn),
				},
			},
		},
		fnBody.List...,
	)
}
