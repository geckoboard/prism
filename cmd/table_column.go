package cmd

import (
	"fmt"
	"regexp"
)

var (
	tableColSplitRegex = regexp.MustCompile(`\s*,\s*`)
)

// A typed value to indicate which table columns should be included in the output.
type tableColumnType int

const (
	tableColTotal tableColumnType = iota
	tableColAvg
	tableColMin
	tableColMax
	tableColInvocations
)

// Return the table header value for this column type.
func (dc tableColumnType) Header() string {
	switch dc {
	case tableColTotal:
		return "total (ms)"
	case tableColAvg:
		return "avg (ms)"
	case tableColMin:
		return "min (ms)"
	case tableColMax:
		return "max (ms)"
	case tableColInvocations:
		return "invoc"
	}
	panic("unsupported column type")
}

// Parse a comma delimited set of column types.
func parseTableColumList(list string) ([]tableColumnType, error) {
	cols := make([]tableColumnType, 0)
	for _, colName := range tableColSplitRegex.Split(list, -1) {
		var col tableColumnType
		switch colName {
		case "total":
			col = tableColTotal
		case "avg":
			col = tableColAvg
		case "min":
			col = tableColMin
		case "max":
			col = tableColMax
		case "invocations":
			col = tableColInvocations
		default:
			return nil, fmt.Errorf("unsupported column name '%s'; supported column names are: total, avg, min, max, invocations", colName)
		}
		cols = append(cols, col)
	}

	return cols, nil
}
