package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Collect the set of go files that comprise a package and its included
// sub-packages and create a static single-assignment representation
// of the source code.
//
// Process the list of SSA function representations exposed by the program and
// select the entries that can be used as injector targets. An entry is considered
// to be a valid target if its fully qualified name starts with the supplied
// package name.
//
// The function maps valid entries to their fully qualified names and returns them as a map.
func ssaCandidates(pathToPackage, fqPkgPrefix string) (map[string]*ssa.Function, error) {
	// Fetch all package-wide go files and pass them to a loader
	goFiles, err := filepath.Glob(fmt.Sprintf("%s*.go", pathToPackage))
	if err != nil {
		return nil, err
	}

	var conf loader.Config
	conf.CreateFromFilenames(fqPkgPrefix, goFiles...)
	loadedProg, err := conf.Load()
	if err != nil {
		return nil, err
	}

	// Convert to SSA format
	ssaProg := ssautil.CreateProgram(loadedProg, ssa.BuilderMode(0))
	ssaProg.Build()

	// Build candidate map
	candidates := make(map[string]*ssa.Function, 0)
	for ssaFn, _ := range ssautil.AllFunctions(ssaProg) {
		target := ssaQualifiedFuncName(ssaFn)
		if strings.HasPrefix(string(target), fqPkgPrefix) {
			candidates[target] = ssaFn
		}
	}
	return candidates, nil
}

// Generate fully qualified name for SSA function representation that includes
// the name of the package. This is achieved by invoking the String() method on
// the supplied SSA function and manipulating its output.
func ssaQualifiedFuncName(fn *ssa.Function) string {
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

// Construct fully qualified package name from a file path by stripping the
// go workspace location from its absolute path representation.
func qualifiedPkgName(pathToPackage string) (string, error) {
	absPackageDir, err := filepath.Abs(filepath.Dir(pathToPackage))
	if err != nil {
		return "", err
	}

	skipLen := strings.Index(absPackageDir, "/src/") + 5
	return absPackageDir[skipLen:], nil
}
