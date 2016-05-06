package inject

import (
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type Target struct {
	Name      string
	Processed bool
	Depth     int
}

// The Analyzer constructs the SSA representation of a program and provides
// tools for analyzing its call graph.
type Analyzer struct {
	projectPkgPrefix string
	ssaProg          *ssa.Program
	fnList           map[*ssa.Function]bool
}

// Create a new analyzer for a project. The projPathOverride
// parameter can be used to override the base project path when parsing the
// source code. Overriding the project path is useful when operating on a
// temp copy of the original project files.
func NewAnalyzer(absProjPath, projPathOverride string) (*Analyzer, error) {
	ssaProg, projectPkgPrefix, err := parseSSA(absProjPath, projPathOverride)
	if err != nil {
		return nil, err
	}

	return &Analyzer{
		projectPkgPrefix: projectPkgPrefix,
		ssaProg:          ssaProg,
		fnList:           ssautil.AllFunctions(ssaProg),
	}, nil
}

// Given a set of target function entrypoints, emit a set of profile targets.
// The method traverses the call graph of each target and collects all direct
// and indirect calls to functions inside the project's path.
func (an *Analyzer) ProfileTargets(targetFnList []string) ([]*Target, error) {
	targets := make([]*Target, 0)

	var root *ssa.Function
	for _, targetFn := range targetFnList {
		// Find SSA function for target
		root = nil
		for fn, _ := range an.fnList {
			fnName := ssaFnName(fn)
			if canVisit(fnName, an.projectPkgPrefix) && fnName == targetFn {
				root = fn
				break
			}
		}

		if root == nil {
			return nil, fmt.Errorf("graph analyzer: no match for profile target '%s'", targetFn)
		}

		targets = append(targets, collect(root, an.projectPkgPrefix)...)
	}

	return targets, nil
}

// Create callgraph rooted at fn and traverse it to collect direct and indirect calls.
func collect(fn *ssa.Function, projectPkgPrefix string) []*Target {
	// Build callgraph
	res := rta.Analyze([]*ssa.Function{fn}, true)

	// Scan callgraph
	targets := make([]*Target, 0)

	var visitFn func(node *callgraph.Node, depth int)
	visitFn = func(node *callgraph.Node, depth int) {
		fnName := ssaFnName(node.Func)
		if !canVisit(fnName, projectPkgPrefix) {
			return
		}

		targets = append(targets, genTarget(fnName, depth))

		// Visit edges
		for _, outEdge := range node.Out {
			visitFn(outEdge.Callee, depth+1)
		}
	}

	// Traverse call graph
	visitFn(res.CallGraph.Root, 0)

	return targets
}

// Setup a profile target for the supplied callgraph node.
func genTarget(fnName string, depth int) *Target {
	return &Target{
		Name:  fnName,
		Depth: depth,
	}
}

// Parse main file and all its dependencies and convert into SSA representation.
func parseSSA(absProjPath, projPathOverride string) (*ssa.Program, string, error) {
	// Build fully qualified package name for project
	projectPkgPrefix, err := qualifiedPkgName(projPathOverride)
	if err != nil {
		return nil, "", err
	}

	// Fetch all go files that this package defines
	goFiles, err := filepath.Glob(fmt.Sprintf("%s/*.go", projPathOverride))
	if err != nil {
		return nil, "", err
	}

	var conf loader.Config
	conf.CreateFromFilenames(projectPkgPrefix, goFiles...)
	loadedProg, err := conf.Load()
	if err != nil {
		return nil, "", err
	}

	// Convert to SSA format
	ssaProg := ssautil.CreateProgram(loadedProg, ssa.BuilderMode(0))
	ssaProg.Build()

	return ssaProg, projectPkgPrefix, nil
}
