package cmd

import (
	"fmt"
	"regexp"
	"strings"
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

var (
	tableColSplitRegex = regexp.MustCompile(`\s*,\s*`)
	tableColTypeToName = map[tableColumnType]string{
		tableColTotal:       "total",
		tableColMin:         "min",
		tableColMax:         "max",
		tableColMean:        "mean",
		tableColMedian:      "median",
		tableColInvocations: "invocations",
		tableColP50:         "p50",
		tableColP75:         "p75",
		tableColP90:         "p90",
		tableColP99:         "p99",
		tableColStdDev:      "stddev",
	}
)

// Header returns the table header description for this column type.
func (dc tableColumnType) Header() string {
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
		return "invoc"
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
	panic("unsupported column type")
}

// Name returns a string representation of this column's type.
func (dc tableColumnType) Name() string {
	return tableColTypeToName[dc]
}

// Parse a comma delimited set of column types.
func parseTableColumList(list string) ([]tableColumnType, error) {
	cols := make([]tableColumnType, 0)
	for _, colName := range tableColSplitRegex.Split(list, -1) {
		found := false
		for colType, colTypeName := range tableColTypeToName {
			if colName == colTypeName {
				cols = append(cols, colType)
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("unsupported column name %q; supported column names are: %s", colName, SupportedColumnNames())
		}
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
