package tools

import (
	"os"
	"testing"
)

func TestCallgraphGeneration(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackage(t)
	defer os.RemoveAll(wsDir)

	candidates, err := ssaCandidates(pkgDir, pkgName, wsDir)
	if err != nil {
		t.Fatal(err)
	}

	mainFqName := pkgName + "/main"
	mainFn := candidates[mainFqName]
	if mainFn == nil {
		t.Fatal("error looking up SSA representation of main()")
	}

	target := &ProfileTarget{
		QualifiedName: mainFqName,
		PkgPrefix:     pkgName,
		ssaFunc:       mainFn,
	}

	graphNodes := target.CallGraph()
	expGraphNodeNames := []string{
		pkgName + "/main",
		pkgName + "/DoStuff",
		pkgName + "/A.DoStuff",
	}
	if len(graphNodes) != len(expGraphNodeNames) {
		t.Fatalf("expected callgraph from main() to have %d nodes; got %d", len(expGraphNodeNames), len(graphNodes))
	}

	for depth, node := range graphNodes {
		if node.Depth != depth {
			t.Errorf("node depth mismatch; expected %d; got %d", depth, node.Depth)
		}
		if node.Name != expGraphNodeNames[depth] {
			t.Errorf("expected node at depth %d to have name %q; got %q", depth, expGraphNodeNames[depth], node.Name)
		}
	}
}

func TestCallgraphGenerationWithNilSSA(t *testing.T) {
	target := &ProfileTarget{
		QualifiedName: "mock/main",
		PkgPrefix:     "mock",
	}

	graphNodes := target.CallGraph()
	expNodes := 1
	if len(graphNodes) != expNodes {
		t.Fatalf("expected callgraph from main() to have %d nodes; got %d", expNodes, len(graphNodes))
	}
}
