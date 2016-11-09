package cmd

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	tableColSplitRegex = regexp.MustCompile(`\s*,\s*`)
)

// A typed value to indicate which table columns should be included in the output.
type tableColumnType int

const (
	tableColTotal tableColumnType = iota
	tableColMin
	tableColMax
	tableColMean
	tableColMedian
	tableColInvocations
	tableColP50
	tableColP75
	tableColP90
	tableColP99
	tableColStdDev
	// a sentinel value allowing us to iterate all valid table column types
	numTableColumns
)

// Header returns the table header description for this column type.
func (dc tableColumnType) Header() string {
	switch dc {
	case tableColTotal:
		return "total (ms)"
	case tableColMin:
		return "min (ms)"
	case tableColMax:
		return "max (ms)"
	case tableColMean:
		return "mean (ms)"
	case tableColMedian:
		return "median (ms)"
	case tableColInvocations:
		return "invoc"
	case tableColP50:
		return "p50 (ms)"
	case tableColP75:
		return "p75 (ms)"
	case tableColP90:
		return "p90 (ms)"
	case tableColP99:
		return "p99 (ms)"
	case tableColStdDev:
		return "stddev"
	}
	panic("unsupported column type")
}

// Name returns a string representation of this column's type.
func (dc tableColumnType) Name() string {
	switch dc {
	case tableColTotal:
		return "total"
	case tableColMin:
		return "min"
	case tableColMax:
		return "max"
	case tableColMean:
		return "mean"
	case tableColMedian:
		return "median"
	case tableColInvocations:
		return "invocations"
	case tableColP50:
		return "p50"
	case tableColP75:
		return "p75"
	case tableColP90:
		return "p90"
	case tableColP99:
		return "p99"
	case tableColStdDev:
		return "stddev"
	}
	return ""
}

// Parse a comma delimited set of column types.
func parseTableColumList(list string) ([]tableColumnType, error) {
	cols := make([]tableColumnType, 0)
	for _, colName := range tableColSplitRegex.Split(list, -1) {
		var col tableColumnType
		switch colName {
		case "total":
			col = tableColTotal
		case "min":
			col = tableColMin
		case "max":
			col = tableColMax
		case "mean":
			col = tableColMean
		case "median":
			col = tableColMedian
		case "invocations":
			col = tableColInvocations
		case "p50":
			col = tableColP50
		case "p75":
			col = tableColP75
		case "p90":
			col = tableColP90
		case "p99":
			col = tableColP99
		case "stddev":
			col = tableColStdDev
		default:
			return nil, fmt.Errorf("unsupported column name %q; supported column names are: %s", colName, SupportedColumnNames())
		}
		cols = append(cols, col)
	}

	return cols, nil
}

// SupportedColumnNames returns back a string will all supported metric column names.
func SupportedColumnNames() string {
	set := make([]string, numTableColumns)
	for i := 0; i < int(numTableColumns); i++ {
		set[i] = tableColumnType(i).Name()
	}

	return strings.Join(set, ", ")
}
