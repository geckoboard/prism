package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/geckoboard/prism/inject"
	"github.com/geckoboard/prism/util"
)

//
func ProfileProject(ctx *cli.Context) {
	args := ctx.Args()
	if len(args) != 1 {
		util.ExitWithError("error: missing path_to_main_file argument")
	}

	profileFuncs := ctx.StringSlice("target")
	if len(profileFuncs) == 0 {
		util.ExitWithError("error: no profile targets specified")
	}

	origPathToMain := args[0]
	origProjectPath, err := filepath.Abs(filepath.Dir(origPathToMain))
	if err != nil {
		util.ExitWithError(err.Error())
	}
	origProjectPath += "/"

	// Clone project
	tmpDir, tmpPathToMain, err := cloneProject(origPathToMain, ctx.String("output-folder"))
	if err != nil {
		util.ExitWithError(err.Error())
	}
	if !ctx.Bool("preserve-output") {
		defer deleteClonedProject(tmpDir)
	}

	// Analyze project
	analyzer, err := inject.NewAnalyzer(tmpPathToMain, origProjectPath)
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}

	// Select profile targets
	profileTargets, err := analyzer.ProfileTargets(profileFuncs)
	fmt.Printf("profile: call graph analyzed %d target(s) and detected %d locations for injecting profiler hooks\n", len(profileFuncs), len(profileTargets))

	// Inject profiler
	injector := inject.NewInjector(filepath.Dir(tmpPathToMain), origProjectPath)
	touchedFiles, err := injector.Hook(profileTargets)
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}

	fmt.Printf("profile: updated %d files\n", touchedFiles)

	// Build
	err = buildProject(tmpDir, tmpPathToMain, ctx.String("build-cmd"))
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}

	// Run
	err = runProject(tmpPathToMain, ctx.String("run-cmd"))
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}
}

// Clone project and return updated path to main
func cloneProject(pathToMain, dest string) (tmpDir, tmpPathToMain string, err error) {
	mainFile := filepath.Base(pathToMain)

	// Get absolute project path and trim everything before the first "src/"
	// path segment which indicates the root of the GOPATH where the project resides in.
	absProjectPath, err := filepath.Abs(filepath.Dir(pathToMain))
	if err != nil {
		return "", "", err
	}
	skipLen := strings.Index(absProjectPath, "/src/")

	tmpDir, err = ioutil.TempDir(dest, "prism-")
	if err != nil {
		return "", "", err
	}

	fmt.Printf("profile: copying project to %s\n", tmpDir)

	err = filepath.Walk(absProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		dstPath := tmpDir + path[skipLen:]

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		} else if !info.Mode().IsRegular() {
			fmt.Printf("profile: [WARNING] skipping non-regular file %s\n", path)
			return nil
		}

		// Copy file
		fSrc, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fSrc.Close()
		fDst, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer fDst.Close()
		_, err = io.Copy(fDst, fSrc)
		return err
	})

	if err != nil {
		deleteClonedProject(tmpDir)
		return "", "", err
	}

	return tmpDir,
		tmpDir + absProjectPath[skipLen:] + "/" + mainFile,
		nil
}

// Build patched project copy.
func buildProject(tmpDir, tmpPathToMain, buildCmd string) error {
	fmt.Printf("profile: building project (%s)\n", buildCmd)

	// In order for go to correctly pick up any patched nested packages
	// instead of the original ones we need to prepend the tmp dir to the
	// build command's GOPATH envvar.
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}
	gopath := "GOPATH=" + tmpDir + separator + os.Getenv("GOPATH")

	env := os.Environ()
	for index, envVar := range env {
		if strings.HasPrefix(envVar, "GOPATH=") {
			env[index] = gopath
			break
		}
	}

	// Setup buffered output writers
	stdout := util.NewPaddedWriter(os.Stdout, "profile: [build] > ")
	stderr := util.NewPaddedWriter(os.Stderr, "profile: [build] > ")

	// Setup the build command and set up its cwd and env overrides
	var execCmd *exec.Cmd
	tokens := strings.Fields(buildCmd)
	if len(tokens) > 1 {
		execCmd = exec.Command(tokens[0], tokens[1:]...)
	} else {
		execCmd = exec.Command(tokens[0])
	}
	execCmd.Dir = filepath.Dir(tmpPathToMain)
	execCmd.Env = env
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr
	err := execCmd.Run()

	// Flush writers
	stdout.Flush()
	stderr.Flush()

	if err != nil {
		return fmt.Errorf("profile: build failed: %s", err.Error())
	}

	return nil
}

// Run patched project to collect profiler data.
func runProject(tmpPathToMain, runCmd string) error {
	fmt.Printf("profile: running patched project (%s)\n", runCmd)

	// Setup buffered output writers
	stdout := util.NewPaddedWriter(os.Stdout, "profile: [run] > ")
	stderr := util.NewPaddedWriter(os.Stderr, "profile: [run] > ")

	// Setup the run command and set up its cwd and env overrides
	var execCmd *exec.Cmd
	tokens := strings.Fields(runCmd)
	if len(tokens) > 1 {
		execCmd = exec.Command(tokens[0], tokens[1:]...)
	} else {
		execCmd = exec.Command(tokens[0])
	}
	execCmd.Dir = filepath.Dir(tmpPathToMain)
	execCmd.Env = os.Environ()
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr
	err := execCmd.Run()

	// Flush writers
	stdout.Flush()
	stderr.Flush()

	if err != nil {
		return fmt.Errorf("profile: run failed: %s", err.Error())
	}

	return nil
}

// Delete temp project copy.
func deleteClonedProject(path string) {
	os.RemoveAll(path)
}
