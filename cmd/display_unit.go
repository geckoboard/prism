package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

type displayUnit uint8

const (
	displayUnitAuto displayUnit = iota
	displayUnitMs
	displayUnitUs
	displayUnitNs
)

// Convert a time.Duration into a floating point value representing the
// duration value in the unit specified by du.
func (du displayUnit) Convert(t time.Duration) float64 {
	switch du {
	case displayUnitMs:
		return float64(t.Nanoseconds()) / 1.0e6
	case displayUnitUs:
		return float64(t.Nanoseconds()) / 1.0e3
	default:
		return float64(t.Nanoseconds())
	}
}

// Format a time unit as a string.
func (du displayUnit) Format(val float64) string {
	switch du {
	case displayUnitMs:
		return humanize.FormatFloat("#,###.##", val) + " ms"
	case displayUnitUs:
		return humanize.FormatFloat("#,###.##", val) + " us"
	default:
		return humanize.Comma(int64(val)) + " ns"
	}
}

// DetectTimeUnit returns the time unit best representing the given time.Duration.
func detectTimeUnit(t time.Duration) displayUnit {
	ns := t.Nanoseconds()
	if ns >= 1e6 {
		return displayUnitMs
	} else if ns >= 1e3 {
		return displayUnitUs
	}
	return displayUnitNs
}

func parseDisplayUnit(val string) (displayUnit, error) {
	trimmed := strings.TrimSpace(val)
	switch trimmed {
	case "auto":
		return displayUnitAuto, nil
	case "us":
		return displayUnitUs, nil
	case "ms":
		return displayUnitMs, nil
	case "ns":
		return displayUnitNs, nil
	}

	return 0, fmt.Errorf("unsupported display unit %q", trimmed)
}
