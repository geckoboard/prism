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
	p1 := &profiler.Profile{
		Target: &profiler.CallMetrics{
			FnName: "main",
			NestedCalls: []*profiler.CallMetrics{
				{
					FnName: "foo",
					NestedCalls: []*profiler.CallMetrics{
						{
							FnName:      "bar",
							NestedCalls: []*profiler.CallMetrics{},
						},
					},
				},
			},
		},
	}

	p2 := &profiler.Profile{
		Target: &profiler.CallMetrics{
			FnName: "main",
			NestedCalls: []*profiler.CallMetrics{
				{
					FnName:      "bar",
					NestedCalls: []*profiler.CallMetrics{},
				},
			},
		},
	}

	profileList := []*profiler.Profile{p1, p2}
	correlations := prepareCorrelationData(profileList[0], len(profileList))
	for profileIndex := 1; profileIndex < len(profileList); profileIndex++ {
		correlations, _ = correlateMetric(profileIndex, profileList[profileIndex].Target, 0, correlations)
	}

	expCount := 3
	if len(correlations) != expCount {
		t.Fatalf("expected correlation table to contain %d entries; got %d", expCount, len(correlations))
	}

	specs := []struct {
		FnName      string
		LeftNotNil  bool
		RightNotNil bool
	}{
		{"main", true, true},
		{"foo", true, false},
		{"bar", true, true},
	}

	for specIndex, spec := range specs {
		row := correlations[specIndex]
		if len(row.metrics) != len(profileList) {
			t.Errorf("[spec %d] expected metric count for correlation row to be %d; got %d", specIndex, len(profileList), len(row.metrics))
			continue
		}

		if row.fnName != spec.FnName {
			t.Errorf("[spec %d] expected correlation row fnName to be %q; got %q", specIndex, spec.FnName, row.fnName)
			continue
		}

		if (spec.LeftNotNil && row.metrics[0] == nil) || (!spec.LeftNotNil && row.metrics[0] != nil) {
			t.Errorf("[spec %d] left correlation entry mismatch; expected it not to be nil? %t", specIndex, spec.LeftNotNil)
			continue
		}
		if (spec.RightNotNil && row.metrics[1] == nil) || (!spec.RightNotNil && row.metrics[1] != nil) {
			t.Errorf("[spec %d] right correlation entry mismatch; expected it not to be nil? %t", specIndex, spec.RightNotNil)
			continue
		}
	}
}

func TestDiffWithProfileLabel(t *testing.T) {
	profileDir, profileFiles := mockProfiles(t, true)
	defer os.RemoveAll(profileDir)

	// Mock args
	set := flag.NewFlagSet("test", 0)
	set.String("columns", SupportedColumnNames(), "")
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
	err = DiffProfiles(ctx)
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
	expOutput := `+------------+----------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
|            | With Label - baseline                                                                                                                        | With Label                                                                                                                                                                       |
+------------+----------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| call stack |  total (ms) |    min (ms) |    max (ms) |   mean (ms) | median (ms) | invoc |    p50 (ms) |    p75 (ms) |    p90 (ms) |    p99 (ms) | stddev |      total (ms) |        min (ms) |        max (ms) |       mean (ms) |     median (ms) | invoc |        p50 (ms) |        p75 (ms) |        p90 (ms) |        p99 (ms) | stddev |
+------------+-------------+-------------+-------------+-------------+-------------+-------+-------------+-------------+-------------+-------------+--------+-----------------+-----------------+-----------------+-----------------+-----------------+-------+-----------------+-----------------+-----------------+-----------------+--------+
| - main     | 120.00 (--) | 120.00 (--) | 120.00 (--) | 120.00 (--) | 120.00 (--) |     1 | 120.00 (--) | 120.00 (--) | 120.00 (--) | 120.00 (--) |  0.000 | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) |     1 | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) |  0.000 |
| | + foo    | 120.00 (--) |  10.00 (--) | 110.00 (--) |  60.00 (--) |  60.00 (--) |     2 |  10.00 (--) |  10.00 (--) |  10.00 (--) | 120.00 (--) | 70.711 | 10.00 (< 12.0x) |       4.00 (--) |  6.00 (< 18.3x) |  5.00 (< 12.0x) |  5.00 (< 12.0x) |     2 |       4.00 (--) |       4.00 (--) |       4.00 (--) |  6.00 (< 20.0x) |  1.414 |
+------------+-------------+-------------+-------------+-------------+-------------+-------+-------------+-------------+-------------+-------------+--------+-----------------+-----------------+-----------------+-----------------+-----------------+-------+-----------------+-----------------+-----------------+-----------------+--------+
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
	set.String("columns", SupportedColumnNames(), "")
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
	expOutput := `+------------+----------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
|            | baseline                                                                                                                                     | profile 1                                                                                                                                                                        |
+------------+----------------------------------------------------------------------------------------------------------------------------------------------+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| call stack |  total (ms) |    min (ms) |    max (ms) |   mean (ms) | median (ms) | invoc |    p50 (ms) |    p75 (ms) |    p90 (ms) |    p99 (ms) | stddev |      total (ms) |        min (ms) |        max (ms) |       mean (ms) |     median (ms) | invoc |        p50 (ms) |        p75 (ms) |        p90 (ms) |        p99 (ms) | stddev |
+------------+-------------+-------------+-------------+-------------+-------------+-------+-------------+-------------+-------------+-------------+--------+-----------------+-----------------+-----------------+-----------------+-----------------+-------+-----------------+-----------------+-----------------+-----------------+--------+
| - main     | 120.00 (--) | 120.00 (--) | 120.00 (--) | 120.00 (--) | 120.00 (--) |     1 | 120.00 (--) | 120.00 (--) | 120.00 (--) | 120.00 (--) |  0.000 | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) |     1 | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) | 10.00 (< 12.0x) |  0.000 |
| | + foo    | 120.00 (--) |  10.00 (--) | 110.00 (--) |  60.00 (--) |  60.00 (--) |     2 |  10.00 (--) |  10.00 (--) |  10.00 (--) | 120.00 (--) | 70.711 | 10.00 (< 12.0x) |       4.00 (--) |  6.00 (< 18.3x) |  5.00 (< 12.0x) |  5.00 (< 12.0x) |     2 |       4.00 (--) |       4.00 (--) |       4.00 (--) |  6.00 (< 20.0x) |  1.414 |
+------------+-------------+-------------+-------------+-------------+-------------+-------+-------------+-------------+-------------+-------------+--------+-----------------+-----------------+-----------------+-----------------+-----------------+-------+-----------------+-----------------+-----------------+-----------------+--------+
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
	profiles := []*profiler.Profile{
		&profiler.Profile{
			Label: label,
			Target: &profiler.CallMetrics{
				FnName:      "main",
				TotalTime:   120 * time.Millisecond,
				MinTime:     120 * time.Millisecond,
				MeanTime:    120 * time.Millisecond,
				MaxTime:     120 * time.Millisecond,
				MedianTime:  120 * time.Millisecond,
				P50Time:     120 * time.Millisecond,
				P75Time:     120 * time.Millisecond,
				P90Time:     120 * time.Millisecond,
				P99Time:     120 * time.Millisecond,
				StdDev:      0.0,
				Invocations: 1,
				NestedCalls: []*profiler.CallMetrics{
					{
						FnName:      "foo",
						TotalTime:   120 * time.Millisecond,
						MeanTime:    60 * time.Millisecond,
						MedianTime:  60 * time.Millisecond,
						MinTime:     10 * time.Millisecond,
						MaxTime:     110 * time.Millisecond,
						P50Time:     10 * time.Millisecond,
						P75Time:     10 * time.Millisecond,
						P90Time:     10 * time.Millisecond,
						P99Time:     120 * time.Millisecond,
						StdDev:      70.71068,
						Invocations: 2,
					},
				},
			},
		},
		&profiler.Profile{
			Label: label,
			Target: &profiler.CallMetrics{
				FnName:      "main",
				TotalTime:   10 * time.Millisecond,
				MinTime:     10 * time.Millisecond,
				MeanTime:    10 * time.Millisecond,
				MaxTime:     10 * time.Millisecond,
				MedianTime:  10 * time.Millisecond,
				P50Time:     10 * time.Millisecond,
				P75Time:     10 * time.Millisecond,
				P90Time:     10 * time.Millisecond,
				P99Time:     10 * time.Millisecond,
				StdDev:      0.0,
				Invocations: 1,
				NestedCalls: []*profiler.CallMetrics{
					{
						FnName:      "foo",
						TotalTime:   10 * time.Millisecond,
						MeanTime:    5 * time.Millisecond,
						MinTime:     4 * time.Millisecond,
						MaxTime:     6 * time.Millisecond,
						MedianTime:  5 * time.Millisecond,
						P50Time:     4 * time.Millisecond,
						P75Time:     4 * time.Millisecond,
						P90Time:     4 * time.Millisecond,
						P99Time:     6 * time.Millisecond,
						StdDev:      1.41421,
						Invocations: 2,
					},
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
