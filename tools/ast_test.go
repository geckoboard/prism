package tools

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestQualifiedCodeName(t *testing.T) {
	parsedFile := mockParsedGoFile(t)

	specs := []struct {
		FnName           string
		ExpQualifiedName string
	}{
		{"NoReceiver", parsedFile.pkgName + "/NoReceiver"},
		{"Receiver", parsedFile.pkgName + "/MyFoo.Receiver"},
		{"PtrReceiver", parsedFile.pkgName + "/MyFoo.PtrReceiver"},
	}

	for specIndex, spec := range specs {
		var fnDecl *ast.FuncDecl = nil
		for _, decl := range parsedFile.astFile.Decls {
			if d, isFnDecl := decl.(*ast.FuncDecl); isFnDecl {
				if d.Name.Name == spec.FnName {
					fnDecl = d
					break
				}
			}
		}

		if fnDecl == nil {
			t.Errorf("[spec %d] could not lookup declaration of %q in test file", specIndex, spec.FnName)
			continue
		}

		qualifiedName := qualifiedNodeName(fnDecl, parsedFile.pkgName)
		if qualifiedName != spec.ExpQualifiedName {
			t.Errorf("[spec %d] expected to get qualified name %q; got %q", specIndex, spec.ExpQualifiedName, qualifiedName)
		}
	}
}

func TestFuncVisitorImport(t *testing.T) {
	parsedFile := mockParsedGoFile(t)
	targetMap := map[string]*CallGraphNode{
		parsedFile.pkgName + "/NoReceiver": &CallGraphNode{
			Name: "NoReceiver",
		},
	}
	visitor := newFuncVisitor(
		targetMap,
		func(_ *CallGraphNode, _ *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
			return true, []string{
				"github.com/foo/bar",
				"namedImport github.com/foo/baz",
			}
		},
	)

	modifiedAST, patchCount := visitor.Process(parsedFile)
	if !modifiedAST {
		t.Fatal("expected func visitor to modify the file AST")
	}

	expPatchCount := 1
	if patchCount != expPatchCount {
		t.Fatalf("expected patchCount to be %d; got %d", expPatchCount, patchCount)
	}

	expImportCount := 2
	if len(parsedFile.astFile.Imports) != expImportCount {
		t.Fatalf("expected import count to be %d; got %d", expImportCount, len(parsedFile.astFile.Imports))
	}

	for importIndex, importDecl := range parsedFile.astFile.Imports {
		switch importDecl.Path.Value {
		case `"github.com/foo/bar"`:
			if importDecl.Name != nil {
				t.Errorf("[import %d] expected import %q to have nil name", importIndex, importDecl.Path.Value)
			}
		case `"github.com/foo/baz"`:
			if importDecl.Name == nil {
				t.Errorf("[import %d] expected named import %q to have non-nil name", importIndex, importDecl.Path.Value)
			} else if importDecl.Name.Name != "namedImport" {
				t.Errorf("[import %d] expected named import %q to be named as %q; got %q", importIndex, importDecl.Path.Value, "namedImport", importDecl.Name.Name)
			}
		default:
			t.Errorf("[import %d] unexpected import %q", importIndex, importDecl.Path.Value)
		}
	}
}

func mockParsedGoFile(t *testing.T) *parsedGoFile {
	filePath := "test.go"
	fqPkgName := "github.com/geckoboard/test"

	src := `
package foo

type MyFoo struct{}

// Forward declaration
func NoReceiver()

func NoReceiver(){}
func (f MyFoo) Receiver(){}
func (f *MyFoo) PtrReceiver(arg int){}
`

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, filePath, src, 0)
	if err != nil {
		t.Fatal(err)
	}

	return &parsedGoFile{
		pkgName:  fqPkgName,
		filePath: filePath,
		fset:     fset,
		astFile:  astFile,
	}
}
