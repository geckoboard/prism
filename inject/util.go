package inject

import (
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
	return strings.HasPrefix(fnName, projectPkgPrefix)
}
