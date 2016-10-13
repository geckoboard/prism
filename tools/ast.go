package tools

import (
	"bytes"
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/ast/astutil"
)

// A visitor for AST nodes representing functions.
type funcVisitor struct {
	// The file we are currently visiting.
	parsedFile *parsedGoFile

	// Additional package imports that need to be injected to the file.
	// Modeled as a map to filter duplicates.
	extraImports map[string]struct{}

	// The patch function to apply to matching targets.
	patchFn PatchFunc

	// The unique list of functions that we need to hook indexed by FQN.
	uniqueTargetMap map[string]*CallGraphNode

	// Flag indicating whether the AST was modified.
	modifiedAST bool
}

// Create a new function node visitor.
func newFuncVisitor(uniqueTargetMap map[string]*CallGraphNode, patchFn PatchFunc) *funcVisitor {
	return &funcVisitor{
		patchFn:         patchFn,
		uniqueTargetMap: uniqueTargetMap,
	}
}

// Apply the visitor to a parsedFile and return a flag indicating whether the AST was modified.
func (v *funcVisitor) Process(parsedFile *parsedGoFile) bool {
	// Reset visitor state
	v.parsedFile = parsedFile
	v.extraImports = make(map[string]struct{}, 0)
	v.modifiedAST = false

	ast.Walk(v, parsedFile.astFile)

	if len(v.extraImports) != 0 {
		for pkgName := range v.extraImports {
			astutil.AddImport(parsedFile.fset, parsedFile.astFile, pkgName)
		}
		v.modifiedAST = true
	}

	return v.modifiedAST
}

// Implements ast.Visitor. Recursively looks for AST nodes that correspond to our
// targets and applies a PatchFunc.
func (v *funcVisitor) Visit(node ast.Node) ast.Visitor {
	// We are only interested in function nodes
	fnDecl, isFnNode := node.(*ast.FuncDecl)
	if !isFnNode {
		return v
	}

	// Ignore forward function declarations
	if fnDecl.Body == nil {
		return nil
	}

	// Check if we need to hook this function
	fqName := qualifiedNodeName(fnDecl, v.parsedFile.pkgName)
	fmt.Printf("FQ: %q\n", fqName)
	cgNode, isTarget := v.uniqueTargetMap[fqName]
	if !isTarget {
		return nil
	}

	modified, extraImports := v.patchFn(cgNode, fnDecl.Body)
	if modified {
		v.modifiedAST = true
	}
	if extraImports != nil {
		for _, name := range extraImports {
			v.extraImports[name] = struct{}{}
		}
	}

	return nil
}

// Returns the fully qualified name for function declaration given its AST node.
func qualifiedNodeName(fnDecl *ast.FuncDecl, pkgName string) string {
	buf := bytes.NewBufferString(pkgName)
	buf.WriteByte('/')

	// Examine receiver
	if fnDecl.Recv != nil {
		for _, rcvField := range fnDecl.Recv.List {
			// We only care for identifiers and star expressions
			switch rcvType := rcvField.Type.(type) {
			case *ast.StarExpr: // e.g (b *Bar)
				buf.WriteString(rcvType.X.(*ast.Ident).Name)
				buf.WriteByte('.')
			case *ast.Ident:
				buf.WriteString(rcvType.Name)
				buf.WriteByte('.')
			}
		}
	}

	// Finally append fn name
	buf.WriteString(fnDecl.Name.Name)
	return buf.String()
}
