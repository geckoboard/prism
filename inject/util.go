package inject

import (
	"bytes"
	"go/ast"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/ssa"
)

var (
	stripCharRegex = regexp.MustCompile(`[()*]`)
)

// Generate fully qualified name for SSA function representation.
func ssaFnName(fn *ssa.Function) string {
	// Normalize fn.String() output by removing parenthesis and star operator
	normalized := stripCharRegex.ReplaceAllString(fn.String(), "")

	// The normalized string representation of the function concatenates
	// the package name and the function name (incl. receiver) with a dot.
	// We need to replace that with a '/'
	if fn.Pkg != nil {
		pkgName := strings.TrimPrefix(fn.Pkg.String(), "package ")
		pkgLen := len(pkgName)
		normalized = normalized[0:pkgLen] + "/" + normalized[pkgLen+1:]
	}
	return normalized
}

func astFnName(fnDecl *ast.FuncDecl, pkgName string) string {
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

// Construct fully qualified package name from file name
func qualifiedPkgName(file string) (string, error) {
	absPackageDir, err := filepath.Abs(filepath.Dir(file))
	if err != nil {
		return "", err
	}

	skipLen := strings.Index(absPackageDir, "/src/") + 5
	return absPackageDir[skipLen:], nil
}

// Check if we can visit a particular function by comparing its fully qualified
// name with the project's package prefix.
func canVisit(fnName, projectPkgPrefix string) bool {
	if strings.Contains(fnName, "Godeps") {
		return false
	}
	return strings.HasPrefix(fnName, projectPkgPrefix)
}
