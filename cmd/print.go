package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/geckoboard/prism/profiler"
	"github.com/geckoboard/prism/util"
	"github.com/urfave/cli"
)

var (
	errNoProfile               = errors.New(`"print" requires a profile argument`)
	errNoPrintColumnsSpecified = errors.New("no table columns specified for printing profile")
)

func PrintProfile(ctx *cli.Context) error {
	var err error

	args := ctx.Args()
	if len(args) != 1 {
		return errNoProfile
	}

	tableCols, err := util.ParseTableColumList(ctx.String("columns"))
	if err != nil {
		return err
	}
	if len(tableCols) == 0 {
		return errNoPrintColumnsSpecified
	}

	threshold := ctx.Float64("threshold")

	profile, err := profiler.LoadProfile(args[0])
	if err != nil {
		return err
	}

	profTable := tabularizeProfile(profile, tableCols, threshold)

	// If stdout is not a terminal we need to strip ANSI characters
	stripAnsiChars := !terminal.IsTerminal(int(os.Stdout.Fd()))
	profTable.Write(os.Stdout, stripAnsiChars)

	return nil
}

// Create a table with profile details.
func tabularizeProfile(profile *profiler.Entry, tableCols []util.TableColumnType, threshold float64) *util.Table {
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
	populateProfileRows(profile, t, tableCols, threshold)

	return t
}

// Populate table rows with profile entry metrics.
func populateProfileRows(pe *profiler.Entry, t *util.Table, tableCols []util.TableColumnType, threshold float64) {
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
			row[baseIndex+dIndex] = fmtEntry(totalTime, threshold)
		case util.TableColAvg:
			row[baseIndex+dIndex] = fmtEntry(avgTime, threshold)
		case util.TableColMin:
			row[baseIndex+dIndex] = fmtEntry(minTime, threshold)
		case util.TableColMax:
			row[baseIndex+dIndex] = fmtEntry(maxTime, threshold)
		case util.TableColInvocations:
			row[baseIndex+dIndex] = fmt.Sprintf("%d", pe.Invocations)
		}
	}

	// Append row to table
	t.Rows = append(t.Rows, row)

	//  Process children
	for _, child := range pe.Children {
		populateProfileRows(child, t, tableCols, threshold)
	}
}

// Format profile entry
func fmtEntry(candidate float64, threshold float64) string {
	if candidate < threshold {
		return ""
	}

	return fmt.Sprintf("%1.2f", candidate)
}
