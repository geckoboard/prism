package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/geckoboard/cli-table"
	"github.com/geckoboard/prism/profiler"
	"gopkg.in/urfave/cli.v1"
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

	df, err := parseDisplayFormat(ctx.String("display-format"))
	if err != nil {
		return err
	}

	tableCols, err := parseTableColumList(ctx.String("display-columns"))
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

	profTable := tabularizeProfile(profile, tableCols, threshold, df)

	// If stdout is not a terminal we need to strip ANSI characters
	filter := table.StripAnsi
	if terminal.IsTerminal(int(os.Stdout.Fd())) && !ctx.Bool("no-ansi") {
		filter = table.PreserveAnsi
	}
	profTable.Write(os.Stdout, filter)

	return nil
}

// Create a table with profile details.
func tabularizeProfile(profile *profiler.Profile, tableCols []tableColumnType, threshold float64, df displayFormat) *table.Table {
	t := table.New(len(tableCols) + 1)
	t.SetPadding(1)

	// Setup headers and alignment settings
	if profile.Label != "" {
		t.SetHeader(0, fmt.Sprintf("%s - call stack", profile.Label), table.AlignLeft)
	} else {
		t.SetHeader(0, "call stack", table.AlignLeft)
	}
	for dIndex, dType := range tableCols {
		t.SetHeader(dIndex+1, dType.Header(df), table.AlignRight)
	}

	// Populate rows
	populateMetricRow(0, profile.Target, profile.Target, t, tableCols, threshold, df)

	return t
}

// Populate table rows with call metrics.
func populateMetricRow(depth int, rootMetrics, rowMetrics *profiler.CallMetrics, t *table.Table, tableCols []tableColumnType, threshold float64, df displayFormat) {
	row := make([]string, len(tableCols)+1)

	// Fill in call
	call := strings.Repeat("| ", depth)
	if len(rowMetrics.NestedCalls) == 0 {
		call += "- "
	} else {
		call += "+ "
	}
	row[0] = call + rowMetrics.FnName

	baseIndex := 1
	for dIndex, dType := range tableCols {
		row[baseIndex+dIndex] = fmtEntry(rootMetrics, rowMetrics, dType, threshold, df)
	}
	t.Append(row)

	// Emit table rows for nested calls
	for _, childMetrics := range rowMetrics.NestedCalls {
		populateMetricRow(depth+1, rootMetrics, childMetrics, t, tableCols, threshold, df)
	}
}

// Format metric entry. An empty string will be returned if the entry is of
// time.Duration type and its value is less than the specified threshold. All
// time duration entries will be formatted as milliseconds.
func fmtEntry(rootMetrics, metrics *profiler.CallMetrics, metricType tableColumnType, threshold float64, df displayFormat) string {
	var val, rootVal time.Duration

	switch metricType {
	case tableColInvocations:
		return fmt.Sprintf("%d", metrics.Invocations)
	case tableColStdDev:
		return fmt.Sprintf("%3.3f", metrics.StdDev)
	case tableColTotal:
		val = metrics.TotalTime
		rootVal = rootMetrics.TotalTime
	case tableColMin:
		val = metrics.MinTime
		rootVal = rootMetrics.MinTime
	case tableColMax:
		val = metrics.MaxTime
		rootVal = rootMetrics.MaxTime
	case tableColMean:
		val = metrics.MeanTime
		rootVal = rootMetrics.MeanTime
	case tableColMedian:
		val = metrics.MedianTime
		rootVal = rootMetrics.MedianTime
	case tableColP50:
		val = metrics.P50Time
		rootVal = rootMetrics.P50Time
	case tableColP75:
		val = metrics.P75Time
		rootVal = rootMetrics.P75Time
	case tableColP90:
		val = metrics.P90Time
		rootVal = rootMetrics.P90Time
	case tableColP99:
		val = metrics.P99Time
		rootVal = rootMetrics.P99Time
	}

	// Convert value to ms
	rootMs := float64(rootVal.Nanoseconds()) / 1.0e6
	ms := float64(val.Nanoseconds()) / 1.0e6
	if ms < threshold {
		return ""
	}

	switch df {
	case displayTime:
		return fmt.Sprintf("%1.2f", ms)
	default:
		percent := 0.0
		if rootMs != 0.0 {
			percent = 100.0 * ms / rootMs
		}
		return fmt.Sprintf("%2.1f%%", percent)
	}
}
