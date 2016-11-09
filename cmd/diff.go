package cmd

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/geckoboard/cli-table"
	"github.com/geckoboard/prism/profiler"
	"golang.org/x/crypto/ssh/terminal"

	"gopkg.in/urfave/cli.v1"
)

const (
	diffEpsilon = 0.01
)

var (
	errNotEnoughProfiles      = errors.New(`"diff" requires at least 2 profiles`)
	errNoDiffColumnsSpecified = errors.New("no table columns specified for diff output")
)

// CorrelatedMetrics groups together captured metrics for the same function
// for a set of captured profiles.
type correlatedMetrics struct {
	fnName         string
	depth          int
	hasNestedCalls bool

	// Entry i will point to the call metric for this function from the i_th
	// profile. If the i_th profile does not contain a metric for this call
	// then metrics[i] wil be nil.
	metrics []*profiler.CallMetrics
}

// DiffProfiles pretty prints a n-way diff between two or more profiles.
func DiffProfiles(ctx *cli.Context) error {
	var err error

	args := ctx.Args()
	if len(args) < 2 {
		return errNotEnoughProfiles
	}

	diffCols, err := parseTableColumList(ctx.String("columns"))
	if err != nil {
		return err
	}
	if len(diffCols) == 0 {
		return errNoDiffColumnsSpecified
	}

	threshold := ctx.Float64("threshold")

	profiles := make([]*profiler.Profile, len(args))
	for index, arg := range args {
		profiles[index], err = loadProfile(arg)
		if err != nil {
			return err
		}
	}

	// Correlate metrics and build diff table
	correlations := prepareCorrelationData(profiles[0], len(profiles))
	for profileIndex := 1; profileIndex < len(profiles); profileIndex++ {
		correlations, _ = correlateMetric(profileIndex, profiles[profileIndex].Target, 0, correlations)
	}
	diffTable := tabularizeDiff(profiles, correlations, diffCols, threshold)

	// If stdout is not a terminal we need to strip ANSI characters
	filter := table.StripAnsi
	if terminal.IsTerminal(int(os.Stdout.Fd())) && !ctx.Bool("no-ansi") {
		filter = table.PreserveAnsi
	}
	diffTable.Write(os.Stdout, filter)

	return nil
}

// Prepare corelation structure for the baseline profile.
func prepareCorrelationData(baseline *profiler.Profile, numProfiles int) []*correlatedMetrics {
	return fillBaselineCorrelationData(0, baseline.Target, numProfiles)
}

// Recursively visit each call metric in the baseline profile and return back an
// initialized correlatedMetrics struct for each one.
func fillBaselineCorrelationData(depth int, baseMetric *profiler.CallMetrics, numProfiles int) []*correlatedMetrics {
	cm := &correlatedMetrics{
		fnName:         baseMetric.FnName,
		depth:          depth,
		hasNestedCalls: len(baseMetric.NestedCalls) > 0,
		metrics:        make([]*profiler.CallMetrics, numProfiles),
	}
	cm.metrics[0] = baseMetric

	cmList := []*correlatedMetrics{cm}
	for _, nestedCallMetric := range baseMetric.NestedCalls {
		cmList = append(cmList, fillBaselineCorrelationData(depth+1, nestedCallMetric, numProfiles)...)
	}

	return cmList
}

// Visit call metric on a non-baseline profile and try to correlate it
// with an entry from the correlated metrics slice generated by the baseline
// profile. We attempt to find a candidate to correlate with by beginning our
// search at index minDepth trying to match the metric name with one of the
// correlated metrics slice entries. This function returns the index of the
// entry that was matched or minDepth if no match could be found.
func correlateMetric(profileIndex int, metric *profiler.CallMetrics, minDepth int, correlations []*correlatedMetrics) ([]*correlatedMetrics, int) {
	for scanIndex := minDepth; scanIndex < len(correlations); scanIndex++ {
		if correlations[scanIndex].fnName == metric.FnName {
			correlations[scanIndex].metrics[profileIndex] = metric
			minDepth = scanIndex
			break
		}
	}

	// Try to match children
	for _, nestedCallMetric := range metric.NestedCalls {
		correlations, minDepth = correlateMetric(profileIndex, nestedCallMetric, minDepth, correlations)
	}

	return correlations, minDepth
}

// Generate a table with that summarizes all profiles and includes a speedup
// factor for each metric compared to the first (baseline) profile.
func tabularizeDiff(profiles []*profiler.Profile, correlations []*correlatedMetrics, diffCols []tableColumnType, threshold float64) *table.Table {
	t := table.New(len(profiles)*len(diffCols) + 1)
	t.SetPadding(1)

	// Populate headers
	t.SetHeader(0, "call stack", table.AlignLeft)
	t.AddHeaderGroup(1, "", table.AlignLeft)

	startOffset := 1
	for index, profile := range profiles {
		baseIndex := startOffset + index*len(diffCols)
		var groupTitle string
		switch profile.Label {
		case "":
			switch index {
			case 0:
				groupTitle = "baseline"
			default:
				groupTitle = fmt.Sprintf("profile %d", index)
			}
		default:
			switch index {
			case 0:
				groupTitle = fmt.Sprintf("%s - baseline", profile.Label)
			default:
				groupTitle = profile.Label
			}
		}
		t.AddHeaderGroup(len(diffCols), groupTitle, table.AlignLeft)

		for dIndex, dType := range diffCols {
			t.SetHeader(baseIndex+dIndex, dType.Header(), table.AlignRight)
		}
	}

	// Populate rows
	for _, correlation := range correlations {
		populateDiffRow(correlation, t, diffCols, threshold)
	}

	return t
}

// Populate table row with comparisons between correlated metrics.
func populateDiffRow(correlation *correlatedMetrics, t *table.Table, diffCols []tableColumnType, threshold float64) {
	numProfiles := len(correlation.metrics)
	row := make([]string, numProfiles*len(diffCols)+1)

	// Fill in call
	call := strings.Repeat("| ", correlation.depth)
	if correlation.hasNestedCalls {
		call += "- "
	} else {
		call += "+ "
	}
	row[0] = call + correlation.fnName

	// Populate measurement columns
	for profileIndex, metrics := range correlation.metrics {
		baseIndex := profileIndex*len(diffCols) + 1
		for dIndex, dType := range diffCols {
			row[baseIndex+dIndex] = fmtDiff(correlation.metrics[0], metrics, dType, threshold)
		}
	}
	t.Append(row)
}

// Colorize and format candidate including a comparison to the baseline value.
// This method treats lower values as better. If the abs delta difference
// of the two values is less than the threshold then no comparison will be performed.
func fmtDiff(baseLine, candidate *profiler.CallMetrics, metricType tableColumnType, threshold float64) string {
	var baseVal time.Duration
	var candVal time.Duration

	if candidate == nil {
		return ""
	}

	switch metricType {
	case tableColInvocations:
		return fmt.Sprintf("%d", candidate.Invocations)
	case tableColStdDev:
		return fmt.Sprintf("%3.3f", candidate.StdDev)
	case tableColTotal:
		baseVal = baseLine.TotalTime
		candVal = candidate.TotalTime
	case tableColMin:
		baseVal = baseLine.MinTime
		candVal = candidate.MinTime
	case tableColMax:
		baseVal = baseLine.MaxTime
		candVal = candidate.MaxTime
	case tableColMean:
		baseVal = baseLine.MeanTime
		candVal = candidate.MeanTime
	case tableColMedian:
		baseVal = baseLine.MedianTime
		candVal = candidate.MedianTime
	case tableColP50:
		baseVal = baseLine.P50Time
		candVal = candidate.P50Time
	case tableColP75:
		baseVal = baseLine.P75Time
		candVal = candidate.P75Time
	case tableColP90:
		baseVal = baseLine.P90Time
		candVal = candidate.P90Time
	case tableColP99:
		baseVal = baseLine.P99Time
		candVal = candidate.P99Time
	}

	// Convert value to ms
	baseMs := float64(baseVal.Nanoseconds()) / 1.0e6
	candMs := float64(candVal.Nanoseconds()) / 1.0e6
	absDelta := math.Abs(baseMs - candMs)

	if candidate == baseLine || absDelta < threshold {
		return fmt.Sprintf("%1.2f (--)", candMs)
	}

	var speedup float64
	if candMs != 0 {
		speedup = baseMs / candMs
	}
	if absDelta < diffEpsilon {
		speedup = 1.0
	}

	var symbol rune
	var color string
	if speedup == 0.0 || speedup == 1.0 {
		color = "\033[33m" // yellow
		symbol = '='
	} else if speedup >= 1.0 {
		color = "\033[32m" // green
		symbol = '<'
	} else {
		color = "\033[31m" // red
		symbol = '>'
	}

	return fmt.Sprintf("%1.2f (%s%c %2.1fx\033[0m)", candMs, color, symbol, speedup)
}
