package cmd

import "testing"

func TestParseTableColumnList(t *testing.T) {
	colNamesToHeaderNames := map[string]string{
		"total":       "total (ms)",
		"avg":         "avg (ms)",
		"min":         "min (ms)",
		"max":         "max (ms)",
		"invocations": "invoc",
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
	expError := `unsupported column name "unknown"; supported column names are: total, avg, min, max, invocations`
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

	unknownType := tableColInvocations + 1000
	unknownType.Header()
}
