package cmd

import (
	"bytes"
	"flag"
	"io"
	"os"
	"testing"

	"gopkg.in/urfave/cli.v1"
)

func TestPrintWithProfileLabel(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, true)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("columns", "min,avg,  max, total,invocations", "")
	set.Float64("threshold", 10.0, "")
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
		t.Fatal(err)
	}

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut

	output := buf.String()
	expOutput := `+-------------------------+----------+----------+----------+------------+-------+
| With Label - call stack | min (ms) | avg (ms) | max (ms) | total (ms) | invoc |
+-------------------------+----------+----------+----------+------------+-------+
| + main                  |   120.00 |   120.00 |   120.00 |     120.00 |     1 |
| | - foo                 |    10.00 |    60.00 |   110.00 |     120.00 |     2 |
+-------------------------+----------+----------+----------+------------+-------+
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
	set.String("columns", "min,avg,  max, total,invocations", "")
	set.Float64("threshold", 10.0, "")
	set.Parse(profileFiles[1:])
	ctx := cli.NewContext(nil, set, nil)

	// Redirect stdout
	stdOut := os.Stdout
	pRead, pWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pWrite

	// Run diff and capture output
	PrintProfile(ctx)

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut

	output := buf.String()
	expOutput := `+------------+----------+----------+----------+------------+-------+
| call stack | min (ms) | avg (ms) | max (ms) | total (ms) | invoc |
+------------+----------+----------+----------+------------+-------+
| + main     |    10.00 |    10.00 |    10.00 |      10.00 |     1 |
| | - foo    |          |          |          |      10.00 |     2 |
+------------+----------+----------+----------+------------+-------+
`

	if expOutput != output {
		t.Fatalf("tabularized print output mismatch; expected:\n%s\n\ngot:\n%s", expOutput, output)
	}
}
