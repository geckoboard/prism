package cmd

import (
	"bytes"
	"io"
)

// padded writer wraps an io.Writer and inserts a customizable padding to the
// beginning of every line. It buffers incoming data and flushes it whenever
// a new line is encountered or the writer is manually flushed.
type paddedWriter struct {
	w         io.Writer
	buf       *bytes.Buffer
	padPrefix []byte
	padSuffix []byte
}

// Wrap a io.Writer with a writer that prepends pad to the beginning of each line.
// An optional color argument containing an ANSI escape code may be specified to
// colorize output for color terminals.
func NewPaddedWriter(w io.Writer, pad, color string) *paddedWriter {
	pw := &paddedWriter{
		w:         w,
		buf:       new(bytes.Buffer),
		padPrefix: []byte(pad),
	}

	if color != "" {
		pw.padPrefix = append(pw.padPrefix, []byte(color)...)
		pw.padSuffix = []byte("\033[0m")
	}

	return pw
}

// Implements io.Writer.
func (pw *paddedWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	var err error
	var lStart, lEnd int
	for _, b := range data {
		lEnd++

		if b != '\n' {
			continue
		}

		// We hit a line feed. Append data block to our buffer
		_, err = pw.buf.Write(data[lStart:lEnd])
		if err != nil {
			return 0, err
		}

		// Flush buffer
		pw.Flush()

		// Reset block indices for next block
		lStart = lEnd
		lEnd = lStart
	}

	// Append any pending bytes.
	if lEnd > lStart {
		_, err = pw.buf.Write(data[lStart:lEnd])
		if err != nil {
			return 0, err
		}
	}

	return len(data), nil
}

// Flush buffered line.
func (pw *paddedWriter) Flush() {
	if pw.buf.Len() == 0 {
		return
	}

	// If last character is not a line feed append one
	if pw.buf.Bytes()[pw.buf.Len()-1] != '\n' {
		pw.buf.WriteByte('\n')
	}

	// Write padding
	_, err := pw.w.Write(pw.padPrefix)
	if err != nil {
		return
	}

	// Write buffered data and suffix
	pw.w.Write(pw.buf.Bytes())
	if pw.padSuffix != nil {
		pw.w.Write(pw.padSuffix)
	}
	pw.buf.Reset()
}
