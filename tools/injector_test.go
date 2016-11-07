package tools

import (
	"fmt"
	"go/ast"
	"testing"
)

func TestInjectProfilerBootstrap(t *testing.T) {
	profileDir := "/tmp/foo"
	profileLabel := "label"
	injectFn := InjectProfilerBootstrap(profileDir, profileLabel)

	cgNode := &CallGraphNode{
		Name:  "main",
		Depth: 0,
	}

	stmt := &ast.BlockStmt{
		List: make([]ast.Stmt, 0),
	}

	modifiedAST, extraImports := injectFn(cgNode, stmt)

	if !modifiedAST {
		t.Fatal("expected injector to modify the AST")
	}

	expImports := append(profilerImports, sinkImports...)
	if !importsMatch(extraImports, expImports) {
		t.Fatalf("injector did not return the expected imports; got %v", extraImports)
	}

	expStmtCount := 2
	if len(stmt.List) != expStmtCount {
		t.Fatalf("expected injector to append %d statements; got %d", expStmtCount, len(stmt.List))
	}

	expStmts := []string{
		fmt.Sprintf("prismProfiler.Init(prismSink.NewFileSink(%q), %q)", profileDir, profileLabel),
		"defer prismProfiler.Shutdown()",
	}
	for stmtIndex, expStmt := range expStmts {
		expr, err := extractExpr(stmt.List[stmtIndex])
		if err != nil {
			t.Errorf("[stmt %d] : %v", stmtIndex, err)
			continue
		}

		if expr != expStmt {
			t.Errorf("[stmt %d] expected expression to be %q; got %q", stmtIndex, expStmt, expr)
		}
	}
}

func TestInjectProfiler(t *testing.T) {
	injectFn := InjectProfiler()

	cgNode := &CallGraphNode{
		Name:  "DoStuff",
		Depth: 1,
	}

	stmt := &ast.BlockStmt{
		List: make([]ast.Stmt, 0),
	}

	modifiedAST, extraImports := injectFn(cgNode, stmt)

	if !modifiedAST {
		t.Fatal("expected injector to modify the AST")
	}

	expImports := profilerImports
	if !importsMatch(extraImports, expImports) {
		t.Fatalf("injector did not return the expected imports; got %v", extraImports)
	}

	expStmtCount := 2
	if len(stmt.List) != expStmtCount {
		t.Fatalf("expected injector to append %d statements; got %d", expStmtCount, len(stmt.List))
	}

	expStmts := []string{
		fmt.Sprintf("prismProfiler.Enter(%q)", cgNode.Name),
		"defer prismProfiler.Leave()",
	}
	for stmtIndex, expStmt := range expStmts {
		expr, err := extractExpr(stmt.List[stmtIndex])
		if err != nil {
			t.Errorf("[stmt %d] : %v", stmtIndex, err)
			continue
		}

		if expr != expStmt {
			t.Errorf("[stmt %d] expected expression to be %q; got %q", stmtIndex, expStmt, expr)
		}
	}
}

func TestProfileFnSelection(t *testing.T) {
	specs := []struct {
		Depth      int
		ExpEnterFn string
		ExpLeaveFn string
	}{
		{0, "BeginProfile", "EndProfile"},
		{1, "Enter", "Leave"},
		{2, "Enter", "Leave"},
	}

	for specIndex, spec := range specs {
		enterFn, leaveFn := profileFnName(spec.Depth)
		if enterFn != spec.ExpEnterFn {
			t.Errorf("[spec %d] expected enter fn to be %q; got %q", specIndex, spec.ExpEnterFn, enterFn)
			continue
		}
		if leaveFn != spec.ExpLeaveFn {
			t.Errorf("[spec %d] expected leave fn to be %q; got %q", specIndex, spec.ExpLeaveFn, leaveFn)
			continue
		}
	}
}

func extractExpr(stmt ast.Stmt) (string, error) {
	if exprStmt, isExpr := stmt.(*ast.ExprStmt); isExpr {
		if basicLitStmt, isLit := exprStmt.X.(*ast.BasicLit); isLit {
			return basicLitStmt.Value, nil
		}
	}

	return "", fmt.Errorf("statement does not contain a basic literal node")
}

func importsMatch(input, expected []string) bool {
	if len(input) != len(expected) {
		return false
	}

	for _, expImport := range expected {
		found := false
		for _, test := range input {
			if test == expImport {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}
