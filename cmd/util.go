package cmd

import (
	"fmt"
	"os"
)

// Output message to stderr and exit with status 1.
func exitWithError(msgFmt string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msgFmt+"\n", args...)
	os.Exit(1)
}
