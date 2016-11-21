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

	pp := &profilePrinter{}

	pp.format, err = parseDisplayFormat(ctx.String("display-format"))
	if err != nil {
		return err
	}

	pp.unit, err = parseDisplayUnit(ctx.String("display-unit"))
	if err != nil {
		return err
	}

	pp.columns, err = parseTableColumList(ctx.String("display-columns"))
	if err != nil {
		return err
	}
	if len(pp.columns) == 0 {
		return errNoPrintColumnsSpecified
	}

	pp.clipThreshold = ctx.Float64("display-threshold")

	profile, err := loadProfile(args[0])
	if err != nil {
		return err
	}

	profTable := pp.Tabularize(profile)

	// If stdout is not a terminal we need to strip ANSI characters
	filter := table.StripAnsi
	if terminal.IsTerminal(int(os.Stdout.Fd())) && !ctx.Bool("no-ansi") {
		filter = table.PreserveAnsi
	}
	profTable.Write(os.Stdout, filter)

	return nil
}

// profilePrinter generates a tabulated output of the captured profile details.
type profilePrinter struct {
	format        displayFormat
	unit          displayUnit
	columns       []tableColumnType
	clipThreshold float64
}

// Create a table with profile details.
func (pp *profilePrinter) Tabularize(profile *profiler.Profile) *table.Table {
	if pp.unit == displayUnitAuto {
		pp.unit = pp.detectTimeUnit(profile.Target)
	}

	t := table.New(len(pp.columns) + 1)
	t.SetPadding(1)

	// Setup headers and alignment settings
	if profile.Label != "" {
		t.SetHeader(0, fmt.Sprintf("%s - call stack", profile.Label), table.AlignLeft)
	} else {
		t.SetHeader(0, "call stack", table.AlignLeft)
	}
	for dIndex, dType := range pp.columns {
		t.SetHeader(dIndex+1, dType.Header(), table.AlignRight)
	}

	// Populate rows
	pp.appendRow(0, profile.Target, profile.Target, t)

	return t
}

// Append a row with call metrics and recursively process nested profile entries.
func (pp *profilePrinter) appendRow(depth int, rootMetrics, rowMetrics *profiler.CallMetrics, t *table.Table) {
	row := make([]string, len(pp.columns)+1)

	// Fill in call
	call := strings.Repeat("| ", depth)
	if len(rowMetrics.NestedCalls) == 0 {
		call += "- "
	} else {
		call += "+ "
	}
	row[0] = call + rowMetrics.FnName

	baseIndex := 1
	for dIndex, dType := range pp.columns {
		row[baseIndex+dIndex] = pp.fmtEntry(rootMetrics, rowMetrics, dType)
	}
	t.Append(row)

	// Emit table rows for nested calls
	for _, childMetrics := range rowMetrics.NestedCalls {
		pp.appendRow(depth+1, rootMetrics, childMetrics, t)
	}
}

// detectTimeUnit iterates through the list of displayable metrics and tries to
// figure out best displayUnit that can represent all displayable values.
func (pp *profilePrinter) detectTimeUnit(metrics *profiler.CallMetrics) displayUnit {
	var val time.Duration
	var unit displayUnit = displayUnitMs
	for _, dType := range pp.columns {
		switch dType {
		case tableColTotal:
			val = metrics.TotalTime
		case tableColMin:
			val = metrics.MinTime
		case tableColMax:
			val = metrics.MaxTime
		case tableColMean:
			val = metrics.MeanTime
		case tableColMedian:
			val = metrics.MedianTime
		case tableColP50:
			val = metrics.P50Time
		case tableColP75:
			val = metrics.P75Time
		case tableColP90:
			val = metrics.P90Time
		case tableColP99:
			val = metrics.P99Time
		default:
			continue
		}

		dUnit := detectTimeUnit(val)
		if dUnit > unit {
			unit = dUnit
		}
	}

	// Process nested measurements
	for _, childMetrics := range metrics.NestedCalls {
		dUnit := pp.detectTimeUnit(childMetrics)
		if dUnit > unit {
			unit = dUnit
		}
	}

	return unit
}

// Format metric entry. An empty string will be returned if the entry is of
// time.Duration type and its value is less than the specified threshold.
func (pp *profilePrinter) fmtEntry(rootMetrics, metrics *profiler.CallMetrics, metricType tableColumnType) string {
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

	// Convert value to the proper unit
	rootTime := pp.unit.Convert(rootVal)
	entryTime := pp.unit.Convert(val)

	switch pp.format {
	case displayTime:
		if entryTime < pp.clipThreshold {
			return ""
		}
		return pp.unit.Format(entryTime)
	default:
		percent := 0.0
		if rootTime != 0.0 {
			percent = 100.0 * entryTime / rootTime
		}
		if percent < pp.clipThreshold {
			return ""
		}
		return fmt.Sprintf("%2.1f%%", percent)
	}
}
