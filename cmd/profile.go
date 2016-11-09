package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/geckoboard/prism/profiler"
	"github.com/geckoboard/prism/tools"
	"gopkg.in/urfave/cli.v1"
)

var (
	errMissingPathToProject = errors.New("missing path_to_project argument")
	errNoProfileTargets     = errors.New("no profile targets specified")
	errMissingRunCmd        = errors.New("run-cmd not specified")

	tokenizeRegex = regexp.MustCompile("'.+?'|\".+?\"|\\S+")
)

// ProfileProject clones a go package, injects profile hooks, builds and runs
// the project to collect profiling information.
func ProfileProject(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return errMissingPathToProject
	}

	profileFuncs := ctx.StringSlice("profile-target")
	if len(profileFuncs) == 0 {
		return errNoProfileTargets
	}

	runCmd := ctx.String("run-cmd")
	if runCmd == "" {
		return errMissingRunCmd
	}

	if !strings.HasSuffix("/", args[0]) {
		args[0] += "/"
	}
	absProjPath, err := filepath.Abs(filepath.Dir(args[0]))
	if err != nil {
		return err
	}
	absProjPath += "/"

	// Clone project
	tmpDir, tmpAbsProjPath, err := cloneProject(absProjPath, ctx.String("output-dir"))
	if err != nil {
		return err
	}
	if !ctx.Bool("preserve-output") {
		defer deleteClonedProject(tmpDir)
	}

	// Analyze project
	goPackage, err := tools.NewGoPackage(tmpAbsProjPath)
	if err != nil {
		return err
	}

	// Select profile targets
	profileTargets, err := goPackage.Find(profileFuncs...)
	if err != nil {
		return err
	}

	// Inject profiler hooks and bootstrap code to main()
	bootstrapTargets := []tools.ProfileTarget{
		tools.ProfileTarget{
			QualifiedName: goPackage.PkgPrefix + "/main",
			PkgPrefix:     goPackage.PkgPrefix,
		},
	}
	updatedFiles, patchCount, err := goPackage.Patch(
		ctx.StringSlice("profile-vendored-pkg"),
		tools.PatchCmd{Targets: profileTargets, PatchFn: tools.InjectProfiler()},
		tools.PatchCmd{Targets: bootstrapTargets, PatchFn: tools.InjectProfilerBootstrap(ctx.String("profile-dir"), ctx.String("profile-label"))},
	)
	if err != nil {
		return err
	}
	fmt.Printf("profile: updated %d files and applied %d patches\n", updatedFiles, patchCount)

	// Handle build step if a build command is specified
	buildCmd := ctx.String("build-cmd")
	if buildCmd != "" {
		err = buildProject(goPackage.GOPATH, tmpAbsProjPath, buildCmd, ctx.Bool("no-ansi"))
		if err != nil {
			return err
		}
	}

	return runProject(goPackage.GOPATH, tmpAbsProjPath, runCmd, ctx.Bool("no-ansi"))
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

// Update GOPATH so that the workspace containing the cloned package is included
// first. This ensures that go will pick up subpackages from the cloned folder.
func overrideGoPath(adjustedGoPath string) []string {
	env := os.Environ()
	for index, envVar := range env {
		if strings.HasPrefix(envVar, "GOPATH=") {
			env[index] = "GOPATH=" + adjustedGoPath
			break
		}
	}

	return env
}

// Build patched project copy.
func buildProject(adjustedGoPath, tmpAbsProjPath, buildCmd string, stripAnsi bool) error {
	fmt.Printf("profile: building patched project (%s)\n", buildCmd)

	color := "\033[32m"
	if stripAnsi {
		color = ""
	}

	// Setup buffered output writers
	stdout := newPaddedWriter(os.Stdout, "profile: [build] > ", color)
	stderr := newPaddedWriter(os.Stderr, "profile: [build] > ", color)

	// Setup the build command and set up its cwd and env overrides
	var execCmd *exec.Cmd
	tokens := tokenizeArgs(buildCmd)
	if len(tokens) > 1 {
		execCmd = exec.Command(tokens[0], tokens[1:]...)
	} else {
		execCmd = exec.Command(tokens[0])
	}
	execCmd.Dir = tmpAbsProjPath
	execCmd.Env = overrideGoPath(adjustedGoPath)
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
func runProject(adjustedGoPath, tmpAbsProjPath, runCmd string, stripAnsi bool) error {
	fmt.Printf("profile: running patched project (%s)\n", runCmd)

	color := "\033[32m"
	if stripAnsi {
		color = ""
	}

	// Setup buffered output writers
	stdout := newPaddedWriter(os.Stdout, "profile: [run] > ", color)
	stderr := newPaddedWriter(os.Stderr, "profile: [run] > ", color)

	// Setup the run command and set up its cwd and env overrides
	var execCmd *exec.Cmd
	tokens := tokenizeArgs(runCmd)
	if len(tokens) > 1 {
		execCmd = exec.Command(tokens[0], tokens[1:]...)
	} else {
		execCmd = exec.Command(tokens[0])
	}
	execCmd.Dir = tmpAbsProjPath
	execCmd.Env = overrideGoPath(adjustedGoPath)
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
		if execCmd.Process != nil {
			execCmd.Process.Signal(s)
		}
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

// Split args into tokens using whitespace as the delimiter. This function
// behaves similar to strings.Fields but also preseves quoted sections.
func tokenizeArgs(args string) []string {
	return tokenizeRegex.FindAllString(args, -1)
}

// loadProfile reads a profile from disk.
func loadProfile(file string) (*profiler.Profile, error) {
	if !strings.HasSuffix(file, ".json") {
		return nil, fmt.Errorf(
			"unrecognized profile extension %q for %q; only json profiles are currently supported",
			filepath.Ext(file),
			file,
		)
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var profile *profiler.Profile
	err = json.Unmarshal(data, &profile)
	return profile, err
}
