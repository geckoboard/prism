package cmd

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/codegangsta/cli"
	"github.com/geckoboard/prism/profiler"
	"github.com/geckoboard/prism/util"
)

func PrintProfile(ctx *cli.Context) {
	var err error

	args := ctx.Args()
	if len(args) != 1 {
		util.ExitWithError("error: print requires a profile argument")
	}

	profile, err := util.LoadJsonProfile(args[0])
	if err != nil {
		util.ExitWithError(err.Error())
	}

	profTable := tabularizeProfile(profile)

	// If stdout is not a terminal we need to strip ANSI characters
	stripAnsiChars := !terminal.IsTerminal(int(os.Stdout.Fd()))
	profTable.Write(os.Stdout, stripAnsiChars)
}

// Create a table with profile details.
func tabularizeProfile(profile *profiler.Entry) *util.Table {
	t := &util.Table{
		Headers: make([]string, 6),
		Rows:    make([][]string, 0),
		Padding: 1,
	}

	// Setup alignment
	t.Alignment = make([]util.Alignment, len(t.Headers))
	t.Alignment[0] = util.AlignLeft
	t.Alignment[1] = util.AlignRight
	t.Alignment[2] = util.AlignRight
	t.Alignment[3] = util.AlignRight
	t.Alignment[4] = util.AlignRight
	t.Alignment[5] = util.AlignRight

	// Setup headers
	t.Headers[0] = "call stack"
	t.Headers[1] = "total (ms)"
	t.Headers[2] = "avg (ms)"
	t.Headers[3] = "min (ms)"
	t.Headers[4] = "max (ms)"
	t.Headers[5] = "invoc"

	// Populate rows
	populateProfileRows(profile, t)

	return t
}

// Populate table rows with profile entry metrics.
func populateProfileRows(pe *profiler.Entry, t *util.Table) {
	row := make([]string, 6)

	// Fill in call
	call := strings.Repeat("| ", pe.Depth)
	if len(pe.Children) == 0 {
		call += "- "
	} else {
		call += "+ "
	}
	row[0] = call + pe.Name

	// Populate measurement columns
	totalTime := float64(pe.TotalTime.Nanoseconds()) / 1.0e6
	avgTime := float64(pe.TotalTime.Nanoseconds()) / float64(pe.Invocations*1e6)
	minTime := float64(pe.MinTime.Nanoseconds()) / 1.0e6
	maxTime := float64(pe.MaxTime.Nanoseconds()) / 1.0e6

	row[1] = fmt.Sprintf("%1.2f", totalTime)
	row[2] = fmt.Sprintf("%1.2f", avgTime)
	row[3] = fmt.Sprintf("%1.2f", minTime)
	row[4] = fmt.Sprintf("%1.2f", maxTime)
	row[5] = fmt.Sprintf("%d", pe.Invocations)

	// Append row to table
	t.Rows = append(t.Rows, row)

	//  Process children
	for _, child := range pe.Children {
		populateProfileRows(child, t)
	}
}
