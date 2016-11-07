package tools

import (
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestQualifiedPkgName(t *testing.T) {
	_, pathToTestFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not get the path of current test file")
	}

	specs := []string{
		pathToTestFile,
		"./", // relative path
	}

	expPkgName := "github.com/geckoboard/prism/tools"
	for specIndex, spec := range specs {
		pkgName, err := qualifiedPkgName(spec)
		if err != nil {
			t.Fatalf("[spec %d] %s", specIndex, err)
		}

		if pkgName != expPkgName {
			t.Fatalf("[spec %d] expected qualified package name to be %q; got %q", specIndex, expPkgName, pkgName)
		}
	}
}

func TestPackageWorkspace(t *testing.T) {
	_, pathToTestFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not get the path of current test file")
	}

	specs := []string{
		pathToTestFile,
		"./", // relative path
	}

	expWorkspace := pathToTestFile[:strings.Index(pathToTestFile, "src/github")-1]
	for specIndex, spec := range specs {
		workspace, err := packageWorkspace(spec)
		if err != nil {
			t.Fatalf("[spec %d] %s", specIndex, err)
		}

		if workspace != expWorkspace {
			t.Fatalf("[spec %d] expected workspace path to be %q; got %q", specIndex, expWorkspace, workspace)
		}
	}
}

func TestSSACandidates(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackage(t)
	defer os.RemoveAll(wsDir)

	candidates, err := ssaCandidates(pkgDir, pkgName, wsDir)
	if err != nil {
		t.Fatal(err)
	}

	// We expect one candidate for each defined function/method + 1 for the init() function
	expCandidates := 4
	if len(candidates) != expCandidates {
		t.Fatalf("expected to get back %d SSA function candidates; got %d", expCandidates, len(candidates))
	}

	validFqNames := map[string]struct{}{
		pkgName + "/A.DoStuff": struct{}{},
		pkgName + "/DoStuff":   struct{}{},
		pkgName + "/main":      struct{}{},
		pkgName + "/init":      struct{}{},
	}
	for fqName := range candidates {
		if _, valid := validFqNames[fqName]; !valid {
			t.Errorf("unexpected fq name %q", fqName)
		}
	}
}

func mockPackage(t *testing.T) (workspaceDir, pkgDir, pkgName string) {
	pkgName = "prism-mock"
	otherPkgName := "other"
	pkgData := map[string]string{
		otherPkgName: `
package other

func DoStuff(){
}
	`,
		pkgName: `
package main

import other "` + otherPkgName + `"

type A struct {
}

func(a *A) DoStuff(){
	// Call to external package; should not be returned as an SSA candidate
	other.DoStuff()
}

func DoStuff(){
	a := &A{}
	a.DoStuff()

	// The callgraph generator should not visit this function a second time
	a.DoStuff()
}

func main(){
	DoStuff()
}
`,
	}

	workspaceDir, err := ioutil.TempDir("", "prism-test")
	if err != nil {
		t.Fatal(err)
	}

	for name, src := range pkgData {
		dir := workspaceDir + "/src/" + name + "/"
		err = os.MkdirAll(dir, os.ModeDir|os.ModePerm)
		if err != nil {
			os.RemoveAll(workspaceDir)
			t.Fatalf("error creating workspace folder for package %q: %s", name, err)
		}

		err = ioutil.WriteFile(dir+"src.go", []byte(src), os.ModePerm)
		if err != nil {
			os.RemoveAll(workspaceDir)
			t.Fatalf("error creating package contents for package %q: %s", name, err)
		}
	}

	pkgDir = workspaceDir + "/src/" + pkgName + "/"
	return workspaceDir, pkgDir, pkgName
}

func mockPackageWithVendoredDeps(t *testing.T, useGodeps bool) (workspaceDir, pkgDir, pkgName string) {
	var otherPkgName string
	pkgName = "prism-mock"

	if useGodeps {
		otherPkgName = pkgName + "/Godeps/_workspace/other/pkg"
	} else {
		otherPkgName = pkgName + "/vendor/other/pkg"
	}

	pkgData := map[string]string{
		otherPkgName: `
package other

func DoStuff(){
}
	`,
		pkgName: `
package main

import other "` + otherPkgName + `"

type A struct {
}

func(a *A) DoStuff(){
	// Call to vendored dep
	other.DoStuff()
}

func DoStuff(){
	a := &A{}
	a.DoStuff()

	// The callgraph generator should not visit this function a second time
	a.DoStuff()
}

func main(){
	DoStuff()
}
`,
	}

	workspaceDir, err := ioutil.TempDir("", "prism-test")
	if err != nil {
		t.Fatal(err)
	}

	for name, src := range pkgData {
		dir := workspaceDir + "/src/" + name + "/"
		err = os.MkdirAll(dir, os.ModeDir|os.ModePerm)
		if err != nil {
			os.RemoveAll(workspaceDir)
			t.Fatalf("error creating workspace folder for package %q: %s", name, err)
		}

		err = ioutil.WriteFile(dir+"src.go", []byte(src), os.ModePerm)
		if err != nil {
			os.RemoveAll(workspaceDir)
			t.Fatalf("error creating package contents for package %q: %s", name, err)
		}
	}

	pkgDir = workspaceDir + "/src/" + pkgName + "/"
	return workspaceDir, pkgDir, pkgName
}
