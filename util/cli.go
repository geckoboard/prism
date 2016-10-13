package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/geckoboard/prism/profiler"
)

var (
	tokenRegex         = regexp.MustCompile("'.+'|\".+\"|\\S+")
	tableColSplitRegex = regexp.MustCompile(`\s*,\s*`)
)

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

// Split args into tokens using whitespace as the delimiter. This function
// behaves similar to strings.Fields but also preseves quoted sections.
func TokenizeArgs(args string) []string {
	return tokenRegex.FindAllString(args, -1)
}

// Padded writer wraps an io.Writer and inserts a customizable padding to the
// beginning of every line. It buffers incoming data and flushes it whenever
// a new line is encountered or the writer is manually flushed.
type PaddedWriter struct {
	w         io.Writer
	buf       *bytes.Buffer
	padPrefix []byte
	padSuffix []byte
}

// Wrap a io.Writer with a writer that prepends pad to the beginning of each line.
// An optional color argument containing an ANSI escape codemay be specified to
// colorize output for color terminals.
func NewPaddedWriter(w io.Writer, pad, color string) *PaddedWriter {
	pw := &PaddedWriter{
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

// A typed value to indicate which table columns should be included in the output.
type TableColumnType int

const (
	TableColTotal TableColumnType = iota
	TableColAvg
	TableColMin
	TableColMax
	TableColInvocations
)

// Return the table header value for this column type.
func (dc TableColumnType) Header() string {
	switch dc {
	case TableColTotal:
		return "total (ms)"
	case TableColAvg:
		return "avg (ms)"
	case TableColMin:
		return "min (ms)"
	case TableColMax:
		return "max (ms)"
	case TableColInvocations:
		return "invoc"
	}

	panic("unsupported column type")
}

// Parse a comma delimited set of column types.
func ParseTableColumList(list string) ([]TableColumnType, error) {
	cols := make([]TableColumnType, 0)
	for _, colName := range tableColSplitRegex.Split(list, -1) {
		var col TableColumnType
		switch colName {
		case "total":
			col = TableColTotal
		case "avg":
			col = TableColAvg
		case "min":
			col = TableColMin
		case "max":
			col = TableColMax
		case "invocations":
			col = TableColInvocations
		default:
			return nil, fmt.Errorf("unsupported column name '%s'; supported column names are: total, avg, min, max, invocations", colName)
		}
		cols = append(cols, col)
	}

	return cols, nil
}
