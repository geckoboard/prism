package cmd

import (
	"bytes"
	"testing"
)

func TestPaddedWriterWithoutColor(t *testing.T) {
	var buf bytes.Buffer

	pw := newPaddedWriter(&buf, "> >", "")
	pw.Write([]byte("line 1\nline 2"))
	pw.Flush()

	expOutput := "> >line 1\n> >line 2\n"
	output := buf.String()
	if output != expOutput {
		t.Fatalf("expected padded writer output to be %q; got %q", expOutput, output)
	}
}

func TestPaddedWriterWithColor(t *testing.T) {
	var buf bytes.Buffer

	pw := newPaddedWriter(&buf, "> >", "\033[41m")
	pw.Write([]byte("line 1\nline 2"))
	pw.Flush()

	expOutput := "> >\033[41mline 1\n\033[0m> >\033[41mline 2\n\033[0m"
	output := buf.String()
	if output != expOutput {
		t.Fatalf("expected padded writer output to be %q; got %q", expOutput, output)
	}
}
