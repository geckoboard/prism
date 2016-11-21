package cmd

import (
	"errors"
	"testing"
	"time"
)

func TestParseDisplayUnit(t *testing.T) {
	specs := []struct {
		input     string
		expOutput displayUnit
		expError  error
	}{
		{"   auto", displayUnitAuto, nil},
		{" ms   ", displayUnitMs, nil},
		{" us", displayUnitUs, nil},
		{"ns", displayUnitNs, nil},
		{"something-else  ", displayUnit(0), errors.New(`unsupported display unit "something-else"`)},
	}

	for specIndex, spec := range specs {
		out, err := parseDisplayUnit(spec.input)
		if spec.expError != nil || err != nil {
			if spec.expError != nil && err == nil || spec.expError == nil && err != nil || spec.expError.Error() != err.Error() {
				t.Errorf("[spec %d] expected error %v; got %v", specIndex, spec.expError, err)
				continue
			}
		}

		if out != spec.expOutput {
			t.Errorf("[spec %d] expected output %d; got %d", specIndex, spec.expOutput, out)
		}
	}
}

func TestFormatDisplayUnit(t *testing.T) {
	specs := []struct {
		in     float64
		unit   displayUnit
		expOut string
	}{
		{1234, displayUnitNs, "1,234 ns"},
		{1.1, displayUnitNs, "1 ns"},
		{123456, displayUnitUs, "123,456.00 us"},
		{1234567, displayUnitUs, "1,234,567.00 us"},
		{0.001, displayUnitUs, "0.00 us"},
		{1234567, displayUnitMs, "1,234,567.00 ms"},
	}

	for specIndex, spec := range specs {
		out := spec.unit.Format(spec.in)
		if out != spec.expOut {
			t.Errorf("[spec %d] expected formatted value to be %q; got %q", specIndex, spec.expOut, out)
		}
	}
}

func TestDetectDisplayUnit(t *testing.T) {
	specs := []struct {
		in      time.Duration
		expUnit displayUnit
	}{
		{10 * time.Nanosecond, displayUnitNs},
		{1000 * time.Nanosecond, displayUnitUs},
		{1000000 * time.Nanosecond, displayUnitMs},
	}

	for specIndex, spec := range specs {
		unit := detectTimeUnit(spec.in)
		if unit != spec.expUnit {
			t.Errorf("[spec %d] expected detected unit to be %v; got %v", specIndex, spec.expUnit, unit)
		}
	}
}
