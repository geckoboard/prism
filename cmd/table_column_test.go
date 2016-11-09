package cmd

import (
	"fmt"
	"testing"
)

func TestParseTableColumnList(t *testing.T) {
	colNamesToHeaderNames := map[string]string{
		"total":       "total (ms)",
		"min":         "min (ms)",
		"max":         "max (ms)",
		"mean":        "mean (ms)",
		"median":      "median (ms)",
		"invocations": "invoc",
		"p50":         "p50 (ms)",
		"p75":         "p75 (ms)",
		"p90":         "p90 (ms)",
		"p99":         "p99 (ms)",
		"stddev":      "stddev",
	}

	for colName, expHeader := range colNamesToHeaderNames {
		colTypes, err := parseTableColumList(colName)
		if err != nil {
			t.Errorf("error parsing col list %q: %v", colName, err)
			continue
		}

		if len(colTypes) != 1 {
			t.Errorf("expected parsed column type list %q to contain 1 entry; got %d", colName, len(colTypes))
			continue
		}

		if colTypes[0].Header() != expHeader {
			t.Errorf("expected header for column %q to be %q; got %q", colName, expHeader, colTypes[0].Header())
		}
	}
}

func TestParseTableColumnListError(t *testing.T) {
	_, err := parseTableColumList("total,     unknown")
	expError := fmt.Sprintf(`unsupported column name "unknown"; supported column names are: %s`, SupportedColumnNames())
	if err == nil || err.Error() != expError {
		t.Fatalf("expected to get error %q; got %v", expError, err)
	}
}

func TestColumnTypePanicForUnknownType(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Fatal("expected Header() to panic")
		}
	}()

	unknownType := numTableColumns
	unknownType.Header()
}
