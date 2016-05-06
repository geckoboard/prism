package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/geckoboard/prism/profiler"
)

// Output message to stderr and exit with status 1.
func ExitWithError(msgFmt string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msgFmt+"\n", args...)
	os.Exit(1)
}

// Load json profile.
func LoadJsonProfile(file string) (*profiler.Entry, error) {
	if !strings.HasSuffix(file, ".json") {
		return nil, fmt.Errorf(
			"error: unrecognized profile extension %s for %s; only json profiles are currently supported",
			filepath.Ext(file),
			file,
		)
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	var pe *profiler.Entry
	err = json.Unmarshal(data, &pe)
	return pe, err
}

// Padded writer wraps an io.Writer and inserts a customizable padding to the
// beginning of every line. It buffers incoming data and flushes it whenever
// a new line is encountered or the writer is manually flushed.
type PaddedWriter struct {
	w   io.Writer
	buf *bytes.Buffer
	pad []byte
}

// Wrap a io.Writer with a writer that prepends pad to the beginning of each line.
func NewPaddedWriter(w io.Writer, pad string) *PaddedWriter {
	return &PaddedWriter{
		w:   w,
		buf: new(bytes.Buffer),
		pad: []byte(pad),
	}
}

// Implements io.Writer.
func (pw *PaddedWriter) Write(data []byte) (int, error) {
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
func (pw *PaddedWriter) Flush() {
	if pw.buf.Len() == 0 {
		return
	}

	// If last character is not a line feed append one
	if pw.buf.Bytes()[pw.buf.Len()-1] != '\n' {
		pw.buf.WriteByte('\n')
	}

	// Write padding
	_, err := pw.w.Write(pw.pad)
	if err != nil {
		return
	}

	// Write buffered data
	pw.w.Write(pw.buf.Bytes())
	pw.buf.Reset()
}
