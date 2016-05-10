package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/geckoboard/prism/inject"
	"github.com/geckoboard/prism/util"
)

//
func ProfileProject(ctx *cli.Context) {
	args := ctx.Args()
	if len(args) != 1 {
		util.ExitWithError("error: missing path_to_project argument")
	}

	profileFuncs := ctx.StringSlice("target")
	if len(profileFuncs) == 0 {
		util.ExitWithError("error: no profile targets specified")
	}

	runCmd := ctx.String("run-cmd")
	if runCmd == "" {
		util.ExitWithError("error: run-cmd not specified")
	}

	if !strings.HasSuffix("/", args[0]) {
		args[0] += "/"
	}
	absProjPath, err := filepath.Abs(filepath.Dir(args[0]))
	if err != nil {
		util.ExitWithError(err.Error())
	}
	absProjPath += "/"

	// Clone project
	tmpDir, tmpAbsProjPath, err := cloneProject(absProjPath, ctx.String("output-folder"))
	if err != nil {
		util.ExitWithError(err.Error())
	}
	if !ctx.Bool("preserve-output") {
		defer deleteClonedProject(tmpDir)
	}

	// Analyze project
	analyzer, err := inject.NewAnalyzer(tmpAbsProjPath, absProjPath)
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}

	// Select profile targets
	profileTargets, err := analyzer.ProfileTargets(profileFuncs)
	fmt.Printf("profile: call graph analyzed %d target(s) and detected %d locations for injecting profiler hooks\n", len(profileFuncs), len(profileTargets))

	// Inject profiler
	injector := inject.NewInjector(tmpAbsProjPath, absProjPath)
	touchedFiles, err := injector.Hook(profileTargets)
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}

	fmt.Printf("profile: updated %d files\n", touchedFiles)

	// Handle build step if a build command is specified
	buildCmd := ctx.String("build-cmd")
	if buildCmd != "" {
		err = buildProject(tmpDir, tmpAbsProjPath, buildCmd)
		if err != nil {
			defer util.ExitWithError(err.Error())
			return
		}
	}

	// Run
	err = runProject(tmpDir, tmpAbsProjPath, runCmd)
	if err != nil {
		defer util.ExitWithError(err.Error())
		return
	}
}

// Clone project and return path to the cloned project.
func cloneProject(absProjPath, dest string) (tmpDir, tmpAbsProjPath string, err error) {
	skipLen := strings.Index(absProjPath, "/src/")

	tmpDir, err = ioutil.TempDir(dest, "prism-")
	if err != nil {
		return "", "", err
	}

	fmt.Printf("profile: copying project to %s\n", tmpDir)

	err = filepath.Walk(absProjPath, func(path string, info os.FileInfo, err error) error {
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
		tmpDir + absProjPath[skipLen:],
		nil
}

// Copy envvars for current process and prepend tmpDir into GOPATH.
func overrideGoPath(tmpDir string) []string {
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

	return env
}

// Build patched project copy.
func buildProject(tmpDir, tmpAbsProjPath, buildCmd string) error {
	fmt.Printf("profile: building project (%s)\n", buildCmd)

	// Setup buffered output writers
	stdout := util.NewPaddedWriter(os.Stdout, "profile: [build] > ", "\033[32m")
	stderr := util.NewPaddedWriter(os.Stderr, "profile: [build] > ", "\033[32m")

	// Setup the build command and set up its cwd and env overrides
	var execCmd *exec.Cmd
	tokens := util.TokenizeArgs(buildCmd)
	if len(tokens) > 1 {
		execCmd = exec.Command(tokens[0], tokens[1:]...)
	} else {
		execCmd = exec.Command(tokens[0])
	}
	execCmd.Dir = tmpAbsProjPath
	execCmd.Env = overrideGoPath(tmpDir)
	execCmd.Stdin = os.Stdin
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
func runProject(tmpDir, tmpAbsProjPath, runCmd string) error {
	fmt.Printf("profile: running patched project (%s)\n", runCmd)

	// Setup buffered output writers
	stdout := util.NewPaddedWriter(os.Stdout, "profile: [run] > ", "\033[32m")
	stderr := util.NewPaddedWriter(os.Stderr, "profile: [run] > ", "\033[32m")

	// Setup the run command and set up its cwd and env overrides
	var execCmd *exec.Cmd
	tokens := util.TokenizeArgs(runCmd)
	if len(tokens) > 1 {
		execCmd = exec.Command(tokens[0], tokens[1:]...)
	} else {
		execCmd = exec.Command(tokens[0])
	}
	execCmd.Dir = tmpAbsProjPath
	execCmd.Env = overrideGoPath(tmpDir)
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr
	// start a signal handler and forward signals to process:
	sigChan := make(chan os.Signal, 1)
	defer close(sigChan)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	gotSignal := false
	go func() {
		s := <-sigChan
		gotSignal = true
		execCmd.Process.Signal(s)
	}()
	err := execCmd.Run()

	// Flush writers
	stdout.Flush()
	stderr.Flush()

	if err != nil && !gotSignal {
		return fmt.Errorf("profile: run failed: %s", err.Error())
	}

	if gotSignal {
		fmt.Printf("profile: patched process execution interrupted by signal\n")
	}

	return nil
}

// Delete temp project copy.
func deleteClonedProject(path string) {
	os.RemoveAll(path)
}
