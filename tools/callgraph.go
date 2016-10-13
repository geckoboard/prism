package tools

import (
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

// Option flags that can be passed to the callgraph builder.
type CallGraphBuildOption uint8

const (
	// Exclude calls to vendored deps from callgraphs.
	IgnoreVendoredDeps CallGraphBuildOption = iota

	// Include calls to vendored deps in callgraphs.
	IncludeVendoredDeps
)

// A section of the callgraph that is reachable through a Target root node via
// one or more hops.
type CallGraphNode struct {
	// A fully qualified function name reachable through a ProfileTarget.
	Name string

	// Number of hops from the callgraph entrypoint (root).
	Depth int
}

// A callgraph consists of a slice of callgraph nodes discovered by performing
// Rapid Type Analysis (RTA) on a ProfileTarget.
type CallGraph []*CallGraphNode

// A ProfileTarget points to a function that serves as an entrypoint for applying
// profiler instrumentation code to itself and any function reachable through it.
type ProfileTarget struct {
	// The fully qualified function name for the target.
	QualifiedName string

	// The fully qualified package name for the analyzed go package.
	PkgPrefix string

	// The SSA representation of the target. We rely on this to perform
	// RTA analysis so we can discover any reachable functions from this endpoint
	ssaFunc *ssa.Function
}

// Construct a callgraph containing the list of qualified function names in
// the project package and its sub-packages that are reachable via a call to
// the profile target.
//
// The discovery of any functions reachable by the endpoint is facilitated by
// the use of Rapid Type Analysis (RTA).
//
// The discovery algorithm only considers functions whose FQN begins with the
// processed root package name. In addition, unless the callgraph was built with the
// IncludeVendoredDeps option, calls to vendored dependencies are also excluded
// from the callgraph.
func (pt *ProfileTarget) CallGraph(buildOption CallGraphBuildOption) CallGraph {
	cg := make(CallGraph, 0)
	if pt.ssaFunc == nil {
		return cg
	}

	var visitFn func(node *callgraph.Node, depth int)
	visitFn = func(node *callgraph.Node, depth int) {
		target := ssaQualifiedFuncName(node.Func)
		if !includeInGraph(target, pt.PkgPrefix, buildOption) {
			return
		}

		cg = append(cg, &CallGraphNode{
			Name:  target,
			Depth: depth,
		})

		// Visit edges
		for _, outEdge := range node.Out {
			visitFn(outEdge.Callee, depth+1)
		}
	}

	// Build and traverse RTA graph starting at entrypoint.
	rtaRes := rta.Analyze([]*ssa.Function{pt.ssaFunc}, true)
	visitFn(rtaRes.CallGraph.Root, 0)

	return cg
}

// Check if target can be include in callgraph.
func includeInGraph(target string, pkgPrefix string, opt CallGraphBuildOption) bool {
	if opt == IgnoreVendoredDeps && strings.Contains(target, "Godeps") {
		return false
	}
	return strings.HasPrefix(target, pkgPrefix)
}
