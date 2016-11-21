package cmd

import (
	"bytes"
	"flag"
	"io"
	"os"
	"testing"

	"gopkg.in/urfave/cli.v1"
)

func TestLoadProfileErrors(t *testing.T) {
	expErr := `unrecognized profile extension ".yml" for "foo.yml"; only json profiles are currently supported`
	_, err := loadProfile("foo.yml")
	if err == nil || err.Error() != expErr {
		t.Fatalf("expected to get error %q; got %v", expErr, err)
	}

	_, err = loadProfile("no-such-file.json")
	if err == nil {
		t.Fatal("expected to get an error")
	}
}

func TestPrintWithProfileLabel(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, true)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("display-columns", SupportedColumnNames(), "")
	set.String("display-format", "time", "")
	set.String("display-unit", "ms", "")
	set.Float64("display-threshold", 11.0, "")
	set.Parse(profileFiles[0:1])
	ctx := cli.NewContext(nil, set, nil)

	// Redirect stdout
	stdOut := os.Stdout
	pRead, pWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pWrite

	// Restore stdout incase of a panic
	defer func() {
		os.Stdout = stdOut
	}()

	// Run diff and capture output
	err = PrintProfile(ctx)
	if err != nil {
		os.Stdout = stdOut
		t.Fatal(err)
	}

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut

	output := buf.String()
	expOutput := `+-------------------------+-----------+-----------+-----------+-----------+-----------+-------+-----------+-----------+-----------+-----------+--------+
| With Label - call stack |     total |       min |       max |      mean |    median | invoc |       p50 |       p75 |       p90 |       p99 | stddev |
+-------------------------+-----------+-----------+-----------+-----------+-----------+-------+-----------+-----------+-----------+-----------+--------+
| + main                  | 120.00 ms | 120.00 ms | 120.00 ms | 120.00 ms | 120.00 ms |     1 | 120.00 ms | 120.00 ms | 120.00 ms | 120.00 ms |  0.000 |
| | - foo                 | 120.00 ms |           | 110.00 ms |  60.00 ms |  60.00 ms |     2 |           |           |           | 120.00 ms | 70.711 |
+-------------------------+-----------+-----------+-----------+-----------+-----------+-------+-----------+-----------+-----------+-----------+--------+
`

	if expOutput != output {
		t.Fatalf("tabularized print output mismatch; expected:\n%s\n\ngot:\n%s", expOutput, output)
	}
}

func TestPrintWithoutProfileLabel(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, false)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("display-columns", SupportedColumnNames(), "")
	set.String("display-format", "time", "")
	set.String("display-unit", "ms", "")
	set.Float64("display-threshold", 0.0, "")
	set.Parse(profileFiles[1:])
	ctx := cli.NewContext(nil, set, nil)

	// Redirect stdout
	stdOut := os.Stdout
	pRead, pWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pWrite

	// Restore stdout incase of a panic
	defer func() {
		os.Stdout = stdOut
	}()

	// Run diff and capture output
	err = PrintProfile(ctx)
	if err != nil {
		os.Stdout = stdOut
		t.Fatal(err)
	}

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut

	output := buf.String()
	expOutput := `+------------+----------+----------+----------+----------+----------+-------+----------+----------+----------+----------+--------+
| call stack |    total |      min |      max |     mean |   median | invoc |      p50 |      p75 |      p90 |      p99 | stddev |
+------------+----------+----------+----------+----------+----------+-------+----------+----------+----------+----------+--------+
| + main     | 10.00 ms | 10.00 ms | 10.00 ms | 10.00 ms | 10.00 ms |     1 | 10.00 ms | 10.00 ms | 10.00 ms | 10.00 ms |  0.000 |
| | - foo    | 10.00 ms |  4.00 ms |  6.00 ms |  5.00 ms |  5.00 ms |     2 |  4.00 ms |  4.00 ms |  4.00 ms |  6.00 ms |  1.414 |
+------------+----------+----------+----------+----------+----------+-------+----------+----------+----------+----------+--------+
`

	if expOutput != output {
		t.Fatalf("tabularized print output mismatch; expected:\n%s\n\ngot:\n%s", expOutput, output)
	}
}

func TestPrintWithoutProfileLabelAndPercentOutput(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, false)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("display-columns", SupportedColumnNames(), "")
	set.String("display-format", "percent", "")
	set.String("display-unit", "auto", "")
	set.Float64("display-threshold", 41.0, "")
	set.Parse(profileFiles[1:])
	ctx := cli.NewContext(nil, set, nil)

	// Redirect stdout
	stdOut := os.Stdout
	pRead, pWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pWrite

	// Restore stdout incase of a panic
	defer func() {
		os.Stdout = stdOut
	}()

	// Run diff and capture output
	err = PrintProfile(ctx)
	if err != nil {
		os.Stdout = stdOut
		t.Fatal(err)
	}

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut

	output := buf.String()
	expOutput := `+------------+--------+--------+--------+--------+--------+-------+--------+--------+--------+--------+--------+
| call stack |  total |    min |    max |   mean | median | invoc |    p50 |    p75 |    p90 |    p99 | stddev |
+------------+--------+--------+--------+--------+--------+-------+--------+--------+--------+--------+--------+
| + main     | 100.0% | 100.0% | 100.0% | 100.0% | 100.0% |     1 | 100.0% | 100.0% | 100.0% | 100.0% |  0.000 |
| | - foo    | 100.0% |        |  60.0% |  50.0% |  50.0% |     2 |        |        |        |  60.0% |  1.414 |
+------------+--------+--------+--------+--------+--------+-------+--------+--------+--------+--------+--------+
`

	if expOutput != output {
		t.Fatalf("tabularized print output mismatch; expected:\n%s\n\ngot:\n%s", expOutput, output)
	}
}
