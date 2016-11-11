package cmd

import (
	"fmt"
	"strings"
)

type displayFormat uint8

const (
	displayTime displayFormat = iota
	displayPercent
)

func parseDisplayFormat(val string) (displayFormat, error) {
	trimmed := strings.TrimSpace(val)
	switch trimmed {
	case "time":
		return displayTime, nil
	case "percent":
		return displayPercent, nil
	}

	return 0, fmt.Errorf("unsupported display format %q", trimmed)
}
