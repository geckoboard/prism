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

	tableCols := util.ParseTableColumList(ctx.String("columns"))
	if len(tableCols) == 0 {
		util.ExitWithError("error: no table columns specified")
	}

	profile, err := util.LoadJsonProfile(args[0])
	if err != nil {
		util.ExitWithError(err.Error())
	}

	profTable := tabularizeProfile(profile, tableCols)

	// If stdout is not a terminal we need to strip ANSI characters
	stripAnsiChars := !terminal.IsTerminal(int(os.Stdout.Fd()))
	profTable.Write(os.Stdout, stripAnsiChars)
}

// Create a table with profile details.
func tabularizeProfile(profile *profiler.Entry, tableCols []util.TableColumnType) *util.Table {
	t := &util.Table{
		Headers:   make([]string, len(tableCols)+1),
		Alignment: make([]util.Alignment, len(tableCols)+1),
		Rows:      make([][]string, 0),
		Padding:   1,
	}

	// Setup headers and alignment settings
	t.Alignment[0] = util.AlignLeft
	t.Headers[0] = "call stack"
	for dIndex, dType := range tableCols {
		t.Alignment[dIndex+1] = util.AlignRight
		t.Headers[dIndex+1] = dType.Header()
	}

	// Populate rows
	populateProfileRows(profile, t, tableCols)

	return t
}

// Populate table rows with profile entry metrics.
func populateProfileRows(pe *profiler.Entry, t *util.Table, tableCols []util.TableColumnType) {
	row := make([]string, len(tableCols)+1)

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

	baseIndex := 1
	for dIndex, dType := range tableCols {
		switch dType {
		case util.TableColTotal:
			row[baseIndex+dIndex] = fmt.Sprintf("%1.2f", totalTime)
		case util.TableColAvg:
			row[baseIndex+dIndex] = fmt.Sprintf("%1.2f", avgTime)
		case util.TableColMin:
			row[baseIndex+dIndex] = fmt.Sprintf("%1.2f", minTime)
		case util.TableColMax:
			row[baseIndex+dIndex] = fmt.Sprintf("%1.2f", maxTime)
		case util.TableColInvocations:
			row[baseIndex+dIndex] = fmt.Sprintf("%d", pe.Invocations)
		}
	}

	// Append row to table
	t.Rows = append(t.Rows, row)

	//  Process children
	for _, child := range pe.Children {
		populateProfileRows(child, t, tableCols)
	}
}
