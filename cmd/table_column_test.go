package cmd

import (
	"fmt"
	"testing"
)

func TestParseTableColumnList(t *testing.T) {
	colNamesToHeaderNames := map[string]string{
		"total":       "total",
		"min":         "min",
		"max":         "max",
		"mean":        "mean",
		"median":      "median",
		"invocations": "invoc",
		"p50":         "p50",
		"p75":         "p75",
		"p90":         "p90",
		"p99":         "p99",
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
	if unknownType.Name() != "" {
		t.Fatal("expected to get an empty string when calling Name() on an unknown table column type")
	}
	unknownType.Header()
}
