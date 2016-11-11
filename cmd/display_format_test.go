package cmd

import (
	"errors"
	"testing"
)

func TestParseDisplayFormat(t *testing.T) {
	specs := []struct {
		input     string
		expOutput displayFormat
		expError  error
	}{
		{"   time", displayTime, nil},
		{"percent   ", displayPercent, nil},
		{"something-else  ", displayFormat(0), errors.New(`unsupported display format "something-else"`)},
	}

	for specIndex, spec := range specs {
		out, err := parseDisplayFormat(spec.input)
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
