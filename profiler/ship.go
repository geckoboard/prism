package profiler

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
)

var (
	profileOutputDir = defaultOutputDir()
	profilePrefix    = "profile-"
	badCharRegex     = regexp.MustCompile(`[\./\\]`)
)

// The null profile shipper drops all incoming profiles.
func nullProfileShipper() {
	fmt.Fprintf(os.Stderr, "profiler: using null profile shipper; all profiles will be dropped\n")

	// Signal that shipper has started
	shipSigChan <- struct{}{}
	defer func() {
		// Signal that we have stopped
		shipSigChan <- struct{}{}
	}()

	for {
		_, ok := <-shipChan
		if !ok {
			return
		}
	}
}

// Encode incoming profiles as json files in $HOME/prism/
func jsonProfileShipper() {
	// Ensure that ouptut folder exists
	err := os.MkdirAll(profileOutputDir, os.ModeDir|os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "profiler: could not create output dir: %s; switching to null shipper\n", err.Error())
		nullProfileShipper()
		return
	}
	fmt.Fprintf(os.Stderr, "profiler: saving profiles to %s\n", profileOutputDir)

	// Signal that shipper has started
	shipSigChan <- struct{}{}
	defer func() {
		// Signal that we have stopped
		shipSigChan <- struct{}{}
	}()
	for {
		profile, ok := <-shipChan
		if !ok {
			return
		}

		fpath := outputFile(profile, "json")
		f, err := os.Create(fpath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "profiler: could not create output file %s due to %s; dropping profile\n", fpath, err.Error())
			continue
		}

		data, _ := json.Marshal(profile)
		f.Write(data)
		f.Close()
	}
}

// Construct the path to a profile file for this entry. This function will
// also pass the path through filepath.Clean to ensure that the proper slashes
// are used depending on the host OS.
func outputFile(pe *Entry, extension string) string {
	return filepath.Clean(
		fmt.Sprintf(
			"%s/%s%s-%d-%d.%s",
			profileOutputDir,
			profilePrefix,
			badCharRegex.ReplaceAllString(pe.Name, "_"),
			pe.EnteredAt.UnixNano(),
			pe.ThreadId,
			extension,
		),
	)
}

// Get default output dir for profiles. This defaults to $HOME/prism.
func defaultOutputDir() string {
	usr, err := user.Current()
	if err != nil {
		panic(err)
	}
	return usr.HomeDir + "/prism"
}
