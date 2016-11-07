package tools

import (
	"fmt"
	"go/ast"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestNewGoPackage(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackage(t)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	if pkg.pathToPackage != pkgDir {
		t.Fatalf("expected pathToPackage to be %q; got %q", pkgDir, pkg.pathToPackage)
	}

	if pkg.PkgPrefix != pkgName {
		t.Fatalf("expected PkgPrefix to be %q; got %q", pkgName, pkg.PkgPrefix)
	}

	separator := ':'
	if runtime.GOOS == "windows" {
		separator = ';'
	}

	expGOPATH := fmt.Sprintf("%s%c%s", wsDir, separator, os.Getenv("GOPATH"))
	if pkg.GOPATH != expGOPATH {
		t.Fatalf("expected adjusted GOPATH to be %q; got %q", expGOPATH, pkg.GOPATH)
	}
}

func TestFindTarget(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackage(t)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	patchTargets := []string{
		pkgName + "/main",
		pkgName + "/DoStuff",
		pkgName + "/A.DoStuff",
	}
	targetList, err := pkg.Find(patchTargets...)
	if err != nil {
		t.Fatal(err)
	}

	if len(targetList) != len(patchTargets) {
		t.Fatalf("expected to get back %d targets; got %d", len(patchTargets), len(targetList))
	}
}

func TestFindMissingTarget(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackage(t)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	invalidTarget := pkgName + "/missing.target"
	expError := fmt.Sprintf("GoPackage.Find: no match for profile target %q", invalidTarget)
	_, err = pkg.Find(invalidTarget)
	if err == nil || err.Error() != expError {
		t.Fatalf("expected to get error %q; got: %v", expError, err)
	}
}

func TestPatchPackageExcludingDeps(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackage(t)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	targetList, err := pkg.Find(pkgName + "/main")
	if err != nil {
		t.Fatal(err)
	}

	if len(targetList) != 1 {
		t.Fatal("error looking up patch target for main()")
	}

	vendorPkgRegex := []string{}
	dummyPatchCmd := PatchCmd{
		Targets: targetList,
		PatchFn: func(_ *CallGraphNode, _ *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
			return true, nil
		},
	}
	updatedFiles, patchCount, err := pkg.Patch(vendorPkgRegex, dummyPatchCmd)
	if err != nil {
		t.Fatal(err)
	}

	expUpdatedFiles := 1
	if updatedFiles != expUpdatedFiles {
		t.Fatalf("expected Patch() to update %d files; got %d", expUpdatedFiles, updatedFiles)
	}

	// pkg.Patch will modify main and the 2 call targets in its callgraph
	expPatchCount := 3
	if patchCount != expPatchCount {
		t.Fatalf("expected Patch() to apply %d patches; got %d", expPatchCount, patchCount)
	}
}

func TestPatchPackageIncludingGodeps(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackageWithVendoredDeps(t, true)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	targetList, err := pkg.Find(pkgName + "/main")
	if err != nil {
		t.Fatal(err)
	}

	if len(targetList) != 1 {
		t.Fatal("error looking up patch target for main()")
	}

	vendorPkgRegex := []string{"other/pkg"}
	dummyPatchCmd := PatchCmd{
		Targets: targetList,
		PatchFn: func(_ *CallGraphNode, _ *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
			return true, nil
		},
	}
	updatedFiles, patchCount, err := pkg.Patch(vendorPkgRegex, dummyPatchCmd)
	if err != nil {
		t.Fatal(err)
	}

	expUpdatedFiles := 2
	if updatedFiles != expUpdatedFiles {
		t.Fatalf("expected Patch() to update %d files; got %d", expUpdatedFiles, updatedFiles)
	}

	// pkg.Patch will modify main and the 2 call targets in its callgraph + 1 function in the vendored package
	expPatchCount := 4
	if patchCount != expPatchCount {
		t.Fatalf("expected Patch() to apply %d patches; got %d", expPatchCount, patchCount)
	}
}

func TestPatchPackageIncludingVendorDeps(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackageWithVendoredDeps(t, false)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	targetList, err := pkg.Find(pkgName + "/main")
	if err != nil {
		t.Fatal(err)
	}

	if len(targetList) != 1 {
		t.Fatal("error looking up patch target for main()")
	}

	vendorPkgRegex := []string{"other/pkg"}
	dummyPatchCmd := PatchCmd{
		Targets: targetList,
		PatchFn: func(_ *CallGraphNode, _ *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
			return true, nil
		},
	}
	updatedFiles, patchCount, err := pkg.Patch(vendorPkgRegex, dummyPatchCmd)
	if err != nil {
		t.Fatal(err)
	}

	expUpdatedFiles := 2
	if updatedFiles != expUpdatedFiles {
		t.Fatalf("expected Patch() to update %d files; got %d", expUpdatedFiles, updatedFiles)
	}

	// pkg.Patch will modify main and the 2 call targets in its callgraph + 1 function in the vendored package
	expPatchCount := 4
	if patchCount != expPatchCount {
		t.Fatalf("expected Patch() to apply %d patches; got %d", expPatchCount, patchCount)
	}
}

func TestPatchPackageWithInvalidVendorRegex(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackageWithVendoredDeps(t, true)
	defer os.RemoveAll(wsDir)

	pkg, err := NewGoPackage(pkgDir)
	if err != nil {
		t.Fatal(err)
	}

	targetList, err := pkg.Find(pkgName + "/main")
	if err != nil {
		t.Fatal(err)
	}

	if len(targetList) != 1 {
		t.Fatal("error looking up patch target for main()")
	}

	vendorPkgRegex := []string{"other/pkg *****"}
	dummyPatchCmd := PatchCmd{
		Targets: targetList,
		PatchFn: func(_ *CallGraphNode, _ *ast.BlockStmt) (modifiedAST bool, extraImports []string) {
			return true, nil
		},
	}
	_, _, err = pkg.Patch(vendorPkgRegex, dummyPatchCmd)
	if err == nil || !strings.Contains(err.Error(), "could not compile regex") {
		t.Fatalf("expected to get a regex compilation error; got %v", err)
	}
}
