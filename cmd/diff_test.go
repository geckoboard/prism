package cmd

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/geckoboard/prism/profiler"
	"gopkg.in/urfave/cli.v1"
)

func TestCorrelateEntries(t *testing.T) {
	p1 := &profiler.Entry{
		Name:  "main",
		Depth: 0,
		Children: []*profiler.Entry{
			{
				Name:  "foo",
				Depth: 1,
				Children: []*profiler.Entry{
					{
						Name:  "bar",
						Depth: 2,
					},
				},
			},
		},
	}

	p2 := &profiler.Entry{
		Name:  "main",
		Depth: 0,
		Children: []*profiler.Entry{
			{
				Name:  "bar",
				Depth: 1,
			},
		},
	}

	out := correlateEntries([]*profiler.Entry{p1, p2})
	expLen := 3
	if len(out) != expLen {
		t.Fatalf("expected correlation map to contain %d entries; got %d", expLen, len(out))
	}

	expLen = 2
	if len(out["main"]) != expLen {
		t.Fatalf("expected correlation entry for main to contain %d entries; got %d", expLen, len(out["main"]))
	}

	expLen = 1
	if len(out["foo"]) != expLen {
		t.Fatalf("expected correlation entry for foo to contain %d entries; got %d", expLen, len(out["foo"]))
	}

	expLen = 2
	if len(out["bar"]) != expLen {
		t.Fatalf("expected correlation entry for bar to contain %d entries; got %d", expLen, len(out["bar"]))
	}
}

func TestDiffWithProfileLabel(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, true)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("columns", "min,avg,  max, total,invocations", "")
	set.Float64("threshold", 10.0, "")
	set.Parse(profileFiles)
	ctx := cli.NewContext(nil, set, nil)

	// Redirect stdout
	stdOut := os.Stdout
	pRead, pWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = pWrite

	// Run diff and capture output
	DiffProfiles(ctx)

	// Drain pipe and restore stdout
	var buf bytes.Buffer
	pWrite.Close()
	io.Copy(&buf, pRead)
	pRead.Close()
	os.Stdout = stdOut

	output := buf.String()
	expOutput := `+------------+-------------------------------------------------------------------+-----------------------------------------------------------------------+
|            | With Label - baseline                                             | With Label                                                            |
+------------+-------------------------------------------------------------------+-----------------------------------------------------------------------+
| call stack |     min (ms) |     avg (ms) |     max (ms) |   total (ms) | invoc |      min (ms) |      avg (ms) |      max (ms) |    total (ms) | invoc |
+------------+--------------+--------------+--------------+--------------+-------+---------------+---------------+---------------+---------------+-------+
| + main     | 120.00 (---) | 120.00 (---) | 120.00 (---) | 120.00 (---) |     1 | 10.00 (12.0x) | 10.00 (12.0x) | 10.00 (12.0x) | 10.00 (12.0x) |     1 |
| | - foo    |  10.00 (---) |  60.00 (---) | 110.00 (---) | 120.00 (---) |     2 |     4.00 (--) |  5.00 (12.0x) |  6.00 (18.3x) | 10.00 (12.0x) |     2 |
+------------+--------------+--------------+--------------+--------------+-------+---------------+---------------+---------------+---------------+-------+
`

	if expOutput != output {
		t.Fatalf("tabularized diff output mismatch; expected:\n%s\n\ngot:\n%s", expOutput, output)
	}
}

func TestDiffWithoutProfileLabel(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, false)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("columns", "min,avg,  max, total,invocations", "")
	set.Float64("threshold", 10.0, "")
	set.Parse(profileFiles)
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
	err = DiffProfiles(ctx)
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
	expOutput := `+------------+-------------------------------------------------------------------+-----------------------------------------------------------------------+
|            | baseline                                                          | profile 1                                                             |
+------------+-------------------------------------------------------------------+-----------------------------------------------------------------------+
| call stack |     min (ms) |     avg (ms) |     max (ms) |   total (ms) | invoc |      min (ms) |      avg (ms) |      max (ms) |    total (ms) | invoc |
+------------+--------------+--------------+--------------+--------------+-------+---------------+---------------+---------------+---------------+-------+
| + main     | 120.00 (---) | 120.00 (---) | 120.00 (---) | 120.00 (---) |     1 | 10.00 (12.0x) | 10.00 (12.0x) | 10.00 (12.0x) | 10.00 (12.0x) |     1 |
| | - foo    |  10.00 (---) |  60.00 (---) | 110.00 (---) | 120.00 (---) |     2 |     4.00 (--) |  5.00 (12.0x) |  6.00 (18.3x) | 10.00 (12.0x) |     2 |
+------------+--------------+--------------+--------------+--------------+-------+---------------+---------------+---------------+---------------+-------+
`

	if expOutput != output {
		t.Fatalf("tabularized diff output mismatch; expected:\n%s\n\ngot:\n%s", expOutput, output)
	}
}

func mockProfiles(t *testing.T, useLabel bool) (profileDir string, profileFiles []string) {
	label := ""
	if useLabel {
		label = "With Label"
	}
	profiles := []*profiler.Entry{
		&profiler.Entry{
			Label:       label,
			Name:        "main",
			Depth:       0,
			TotalTime:   120 * time.Millisecond,
			MinTime:     120 * time.Millisecond,
			MaxTime:     120 * time.Millisecond,
			Invocations: 1,
			Children: []*profiler.Entry{
				{
					Name:        "foo",
					Depth:       1,
					TotalTime:   120 * time.Millisecond,
					MinTime:     10 * time.Millisecond,
					MaxTime:     110 * time.Millisecond,
					Invocations: 2,
				},
			},
		},
		&profiler.Entry{
			Label:       label,
			Name:        "main",
			Depth:       0,
			TotalTime:   10 * time.Millisecond,
			MinTime:     10 * time.Millisecond,
			MaxTime:     10 * time.Millisecond,
			Invocations: 1,
			Children: []*profiler.Entry{
				{
					Name:        "foo",
					Depth:       1,
					TotalTime:   10 * time.Millisecond,
					MinTime:     4 * time.Millisecond,
					MaxTime:     6 * time.Millisecond,
					Invocations: 2,
				},
			},
		},
	}

	var err error
	profileDir, err = ioutil.TempDir("", "prism-test")
	if err != nil {
		t.Fatal(err)
	}

	profileFiles = make([]string, 0)
	for index, pe := range profiles {
		data, err := json.Marshal(pe)
		if err != nil {
			os.RemoveAll(profileDir)
			t.Fatal(err)
		}

		file := fmt.Sprintf("%s/profile-%d.json", profileDir, index)
		err = ioutil.WriteFile(file, data, os.ModePerm)
		if err != nil {
			os.RemoveAll(profileDir)
			t.Fatal(err)
		}
		profileFiles = append(profileFiles, file)
	}

	return profileDir, profileFiles
}
