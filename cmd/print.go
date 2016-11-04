package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/geckoboard/cli-table"
	"github.com/geckoboard/prism/profiler"
	"github.com/urfave/cli"
)

var (
	errNoProfile               = errors.New(`"print" requires a profile argument`)
	errNoPrintColumnsSpecified = errors.New("no table columns specified for printing profile")
)

// PrintProfile displays a captured profile in tabular form.
func PrintProfile(ctx *cli.Context) error {
	var err error

	args := ctx.Args()
	if len(args) != 1 {
		return errNoProfile
	}

	tableCols, err := parseTableColumList(ctx.String("columns"))
	if err != nil {
		return err
	}
	if len(tableCols) == 0 {
		return errNoPrintColumnsSpecified
	}

	threshold := ctx.Float64("threshold")

	profile, err := loadProfile(args[0])
	if err != nil {
		return err
	}

	profTable := tabularizeProfile(profile, tableCols, threshold)

	// If stdout is not a terminal we need to strip ANSI characters
	filter := table.StripAnsi
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		filter = table.PreserveAnsi
	}
	profTable.Write(os.Stdout, filter)

	return nil
}

// Create a table with profile details.
func tabularizeProfile(profile *profiler.Entry, tableCols []tableColumnType, threshold float64) *table.Table {
	t := table.New(len(tableCols) + 1)
	t.SetPadding(1)

	// Setup headers and alignment settings
	if profile.Label != "" {
		t.SetHeader(0, fmt.Sprintf("%s - call stack", profile.Label), table.AlignLeft)
	} else {
		t.SetHeader(0, "call stack", table.AlignLeft)
	}
	for dIndex, dType := range tableCols {
		t.SetHeader(dIndex+1, dType.Header(), table.AlignRight)
	}

	// Populate rows
	populateProfileRows(profile, t, tableCols, threshold)

	return t
}

// Populate table rows with profile entry metrics.
func populateProfileRows(pe *profiler.Entry, t *table.Table, tableCols []tableColumnType, threshold float64) {
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
		case tableColTotal:
			row[baseIndex+dIndex] = fmtEntry(totalTime, threshold)
		case tableColAvg:
			row[baseIndex+dIndex] = fmtEntry(avgTime, threshold)
		case tableColMin:
			row[baseIndex+dIndex] = fmtEntry(minTime, threshold)
		case tableColMax:
			row[baseIndex+dIndex] = fmtEntry(maxTime, threshold)
		case tableColInvocations:
			row[baseIndex+dIndex] = fmt.Sprintf("%d", pe.Invocations)
		}
	}

	// Append row to table
	t.Append(row)

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
