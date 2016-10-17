package tools

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/ssa"
)

var (
	stripCharRegex = regexp.MustCompile(`[()*]`)
)

// A function used to modify the AST for a go function matching a profile target. The
// function should return a flag to indicate whether the AST of the function was modified
// and a list of additional package imports to be injected into the file where the target
// is defined.
//
// The method is passed a callgraph node instance and the AST node that corresponds to its body.
type PatchFunc func(cgNode *CallGraphNode, fnDeclNode *ast.BlockStmt) (modifiedAST bool, extraImports []string)

// A patch command groups together a list of targets and a patch function to apply to them.
type PatchCmd struct {
	// The slice of targets to hook.
	Targets []ProfileTarget

	// The patch function to apply to functions matching the target.
	PatchFn PatchFunc
}

// Represents the contents of a parsed go file.
type parsedGoFile struct {
	// Path to the file.
	filePath string

	// The package name for this file.
	pkgName string

	// The set of tokens and AST for the parsed file.
	fset    *token.FileSet
	astFile *ast.File
}

// The output of an SSA analysis performed on a package and any packages it
// references.
type GoPackage struct {
	// The path to the project.
	pathToPackage string

	// The fully qualified package name for the base package.
	PkgPrefix string

	// A map of FQ function names to their SSA representation. This map
	// only contains functions that can be used as profile injection points.
	ssaFuncCandidates map[string]*ssa.Function
}

// Analyze all go files in pathToPackage as well as any other packages that are
// referenced by them and construct a static single-assignment representation of
// the underlying code.
func NewGoPackage(pathToPackage string) (*GoPackage, error) {
	// Detect FQN for project base package
	fqPkgPrefix, err := qualifiedPkgName(pathToPackage)
	if err != nil {
		return nil, err
	}

	candidates, err := ssaCandidates(pathToPackage, fqPkgPrefix)
	if err != nil {
		return nil, err
	}

	return &GoPackage{
		pathToPackage:     pathToPackage,
		PkgPrefix:         fqPkgPrefix,
		ssaFuncCandidates: candidates,
	}, nil
}

// Lookup a list of fully qualified profile targets inside the parsed package sources.
// These targets serve as the entrypoint for injecting profiler hooks to any
// function reachable by that entrypoint.
//
// Injector targets for functions without receivers are constructed by
// concatenating the fully qualified package name where the function is defined,
// a '/' character and the function name.
//
// If the function uses a receiver, the target name is constructed by
// concatenating the fully qualified package name, a '/' character, the
// function receiver's name, a '.' character and finally the function name.
//
// Here is an example of constructing injector targets for a set of functions
// defined in package: "github.com/geckoboard/foo"
//
//  package foo
//
//  func MoreStuff(){}
//
//  type Foo struct{}
//  func (f *Foo) DoStuff(){}
//
// The injector targets for the two functions are defined as:
//  github.com/geckoboard/foo/MoreStuff
//  github.com/geckoboard/foo/Foo.DoStuff
func (pkg *GoPackage) Find(targetList ...string) ([]ProfileTarget, error) {
	profileTargets := make([]ProfileTarget, len(targetList))
	var entrypointSSA *ssa.Function = nil
	for targetIndex, target := range targetList {
		entrypointSSA = nil
		for candidate, ssaFn := range pkg.ssaFuncCandidates {
			if candidate == target {
				entrypointSSA = ssaFn
				break
			}
		}

		if entrypointSSA == nil {
			return nil, fmt.Errorf("GoPackage.Find: no match for profile target %q", target)
		}

		profileTargets[targetIndex] = ProfileTarget{
			QualifiedName: target,
			PkgPrefix:     pkg.PkgPrefix,
			ssaFunc:       entrypointSSA,
		}
	}

	return profileTargets, nil
}

// Iterate the list of go source files that comprise this package and any folder
// defined inside it and apply the patch function to AST entries matching the given
// list of targets.
//
// This function will automatically overwrite any files that are modified by the
// given patch function.
func (pkg *GoPackage) Patch(vendorPkgRegex []string, patchCmds ...PatchCmd) (updatedFiles int, err error) {
	// Parse package sources
	parsedFiles, err := parsePackageSources(pkg.pathToPackage, vendorPkgRegex)
	if err != nil {
		return 0, err
	}

	// Expand the callgraph of hook targets and generate a visitor for each patch cmd
	visitors := make([]*funcVisitor, len(patchCmds))
	for cmdIndex, cmd := range patchCmds {
		visitors[cmdIndex] = newFuncVisitor(uniqueTargetMap(cmd.Targets), cmd.PatchFn)
	}

	for _, parsedFile := range parsedFiles {
		modifiedAST := false
		for _, visitor := range visitors {
			modifiedAST = visitor.Process(parsedFile) || modifiedAST
		}

		// If the file was updated write it back to disk
		if modifiedAST {
			f, err := os.Create(parsedFile.filePath)
			if err != nil {
				return 0, err
			}
			printer.Fprint(f, parsedFile.fset, parsedFile.astFile)
			f.Close()
			updatedFiles++
		}
	}

	return updatedFiles, err
}

// For each profile target, discover all reachable functions in its callgraph and
// generate a map where keys are the FQ name of each callgraph node and values
// are the callgraph nodes.
func uniqueTargetMap(targets []ProfileTarget) map[string]*CallGraphNode {
	uniqueTargets := make(map[string]*CallGraphNode, 0)
	for _, target := range targets {
		cg := target.CallGraph()
		for _, cgNode := range cg {
			uniqueTargets[cgNode.Name] = cgNode
		}
	}

	return uniqueTargets
}

// Recursively scan pathToPackage and create an AST for any non-test go files
// that are found.
func parsePackageSources(pathToPackage string, vendorPkgRegex []string) ([]*parsedGoFile, error) {
	var err error
	pkgRegexes := make([]*regexp.Regexp, len(vendorPkgRegex))
	for index, regex := range vendorPkgRegex {
		pkgRegexes[index], err = regexp.Compile(regex)
		if err != nil {
			return nil, fmt.Errorf("GoPackage.Patch: could not compile regex for profile-vendored-pkg arg %q: %s", regex, err)
		}
	}

	parsedFiles := make([]*parsedGoFile, 0)
	buildAST := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip dirs, non-go and go test files
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// If this is a vendored dependency we should skip it unless it matches one of
		// the user-defined vendor package regexes
		if isVendoredDep(path) {
			keep := false
			for _, r := range pkgRegexes {
				if r.MatchString(path) {
					keep = true
					break
				}
			}
			if !keep {
				return nil
			}
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("in: could not parse %s; %v", path, err)
		}

		pkgName, err := qualifiedPkgName(path)
		if err != nil {
			return err
		}

		parsedFiles = append(parsedFiles, &parsedGoFile{
			pkgName:  pkgName,
			filePath: path,
			fset:     fset,
			astFile:  f,
		})

		return nil
	}

	// Parse package files
	err = filepath.Walk(pathToPackage, buildAST)
	if err != nil {
		return nil, err
	}

	return parsedFiles, nil
}

// Check if the given path points to a vendored dependency.
func isVendoredDep(path string) bool {
	return strings.Contains(path, "/Godeps/") || strings.Contains(path, "/vendor/")
}
