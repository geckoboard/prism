package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"gopkg.in/urfave/cli.v1"
)

func TestProfile(t *testing.T) {
	wsDir, pkgDir, pkgName := mockPackageWithVendoredDeps(t, true)
	defer os.RemoveAll(wsDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("profile-dir", wsDir, "")
	set.String("build-cmd", "go build -o artifact", "")
	set.String("run-cmd", "./artifact", "")
	set.Bool("no-ansi", true, "")
	set.Parse([]string{pkgDir})
	targets := cli.StringSlice{pkgName + "/main"}
	targetFlag := &cli.StringSliceFlag{
		Name:  "profile-target",
		Value: &targets,
	}
	targetFlag.Apply(set)
	ctx := cli.NewContext(nil, set, nil)

	// Redirect stdout and stderr
	stdOut := os.Stdout
	stdErr := os.Stderr
	pRead, pWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pWrite
	os.Stderr = pWrite

	// Profile package and capture output
	err = ProfileProject(ctx)
	if err != nil {
		os.Stdout = stdOut
		os.Stderr = stdErr
		t.Fatal(err)
	}

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut
	os.Stderr = stdErr

	outputLines := strings.Split(strings.Trim(buf.String(), "\n"), "\n")
	expLines := 5
	if len(outputLines) != expLines {
		t.Fatalf("expected profile cmd output to emit %d output lines; got %d", expLines, len(outputLines))
	}

	specs := []struct {
		Line    int
		ExpText string
	}{
		{1, "profile: updated 1 files and applied 4 patches"},
		{2, "profile: building patched project (go build -o artifact)"},
		{3, "profile: running patched project (./artifact)"},
		{4, fmt.Sprintf("profile: [run] > profiler: saving profiles to %s", wsDir)},
	}

	for _, spec := range specs {
		if outputLines[spec.Line] != spec.ExpText {
			t.Errorf("[output line %d] expected text to match %q; got %q", spec.Line, spec.ExpText, outputLines[spec.Line])
		}
	}
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
