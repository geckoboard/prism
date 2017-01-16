package table

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Alignment represents the supported cell content alignment modes.
type Alignment uint8

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

// CharacterFilter defines the character filter modes supported by the table writer.
type CharacterFilter uint8

const (
	PreserveAnsi CharacterFilter = iota
	StripAnsi
)

var (
	ansiEscapeRegex    = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	tableColSplitRegex = regexp.MustCompile(`\s*,\s*`)
)

// Header groups are used to define headers that span multiple columns.
type headerGroup struct {
	header    string
	alignment Alignment
	colSpan   int
}

// A table that can be rendered in a terminal.
type Table struct {
	headers      []string
	headerGroups []headerGroup
	rows         [][]string
	alignments   []Alignment
	padding      int
}

// Create a new empty table with the specified number of columns.
func New(columns int) *Table {
	return &Table{
		headers:      make([]string, columns),
		headerGroups: make([]headerGroup, 0),
		rows:         make([][]string, 0),
		alignments:   make([]Alignment, columns),
	}
}

// Set cell padding for cell contents. If a negative padding is specified, a
// padding value of 0 will be forced.
func (t *Table) SetPadding(padding int) {
	if padding < 0 {
		padding = 0
	}

	t.padding = padding
}

// Set header title and column alignment settings. Column indices are 0-based.
func (t *Table) SetHeader(col int, title string, alignment Alignment) error {
	if col < 0 || col > len(t.headers)-1 {
		return fmt.Errorf("index out of range while attempting to set table header for column %d", col)
	}

	t.headers[col] = title
	t.alignments[col] = alignment
	return nil
}

// Add a super-group for a set of header columns. If the requested colSpan exceeds
// the number of available un-grouped header columns this method returns an error.
func (t *Table) AddHeaderGroup(colSpan int, title string, alignment Alignment) error {
	groupedCols := 0
	for _, hg := range t.headerGroups {
		groupedCols += hg.colSpan
	}

	colCount := len(t.headers)
	if groupedCols+colSpan > colCount {
		return fmt.Errorf("requested header group colspan %d exceeds the available columns for grouping %d/%d", colSpan, groupedCols, colCount)
	}

	t.headerGroups = append(t.headerGroups, headerGroup{
		header:    title,
		colSpan:   colSpan,
		alignment: alignment,
	})
	return nil
}

// Append one or more rows to the table.
func (t *Table) Append(rows ...[]string) error {
	colCount := len(t.headers)

	for rowIndex, row := range rows {
		if len(row) != colCount {
			return fmt.Errorf("inconsistent number of colums for row %d; expected %d but got %d", rowIndex, colCount, len(row))
		}
	}

	t.rows = append(t.rows, rows...)
	return nil
}

// Render table to an io.Writer. The charFilter parameter can be used to
// either preserve or strip ANSI characters from the output.
func (t *Table) Write(to io.Writer, charFilter CharacterFilter) {
	stripAnsiChars := charFilter == StripAnsi
	w := bufio.NewWriter(to)
	padding := strings.Repeat(" ", t.padding)

	// Calculate col widths and use them to calculate group heading widths
	colWidths := t.colWidths()

	// Render header groups if defined
	if len(t.headerGroups) > 0 {
		var groupWidths []int
		groupWidths, colWidths = t.groupWidths(colWidths)
		hLine := t.hLine(groupWidths)

		w.WriteString(hLine)
		w.WriteByte('|')
		for hgIndex, hg := range t.headerGroups {
			w.WriteString(padding)
			w.WriteString(t.align(hg.header, hg.alignment, groupWidths[hgIndex], stripAnsiChars))
			w.WriteString(padding)
			w.WriteByte('|')
		}
		w.WriteString("\n")
		w.WriteString(hLine)
	}

	// Render headers
	hLine := t.hLine(colWidths)
	if len(t.headerGroups) == 0 {
		w.WriteString(hLine)
	}
	w.WriteByte('|')
	for colIndex, h := range t.headers {
		w.WriteString(padding)
		w.WriteString(t.align(h, t.alignments[colIndex], colWidths[colIndex], stripAnsiChars))
		w.WriteString(padding)
		w.WriteByte('|')
	}
	w.WriteString("\n")
	w.WriteString(hLine)

	// Render rows
	for _, row := range t.rows {
		w.WriteByte('|')
		for colIndex, c := range row {
			w.WriteString(padding)
			w.WriteString(t.align(c, t.alignments[colIndex], colWidths[colIndex], stripAnsiChars))
			w.WriteString(padding)
			w.WriteByte('|')
		}
		w.WriteString("\n")
	}

	// Render footer line if the table is not empty
	if len(t.rows) > 0 {
		w.WriteString(hLine)
	}

	w.Flush()
}

// Generate horizontal line.
func (t *Table) hLine(colWidths []int) string {
	buf := bytes.NewBufferString("")

	buf.WriteByte('+')
	for _, colWidth := range colWidths {
		buf.WriteString(strings.Repeat("-", colWidth+2*t.padding))
		buf.WriteByte('+')
	}
	buf.WriteString("\n")

	return buf.String()
}

// Pad and align input string.
func (t *Table) align(val string, align Alignment, maxWidth int, stripAnsiChars bool) string {
	var vLen int

	if stripAnsiChars {
		val = ansiEscapeRegex.ReplaceAllString(val, "")
		vLen = utf8.RuneCountInString(val)
	} else {
		vLen = measure(val)
	}

	switch align {
	case AlignLeft:
		return val + strings.Repeat(" ", maxWidth-vLen)
	case AlignRight:
		return strings.Repeat(" ", maxWidth-vLen) + val
	default:
		lPad := (maxWidth - vLen) / 2
		return strings.Repeat(" ", lPad) + val + strings.Repeat(" ", maxWidth-lPad-vLen)
	}
}

// Calculate max width for each column.
func (t *Table) colWidths() []int {
	colWidths := make([]int, len(t.headers))
	for colIndex, h := range t.headers {
		maxWidth := utf8.RuneCountInString(h)
		for _, row := range t.rows {
			cellWidth := measure(row[colIndex])
			if cellWidth > maxWidth {
				maxWidth = cellWidth
			}
		}

		colWidths[colIndex] = maxWidth
	}
	return colWidths
}

// Calculate max width for each header group. If a group header's width exceeds
// the total width of the grouped columns, they will be automatically expanded
// to preserve alignment with the group header.
func (t *Table) groupWidths(colWidths []int) (groupWidths []int, adjustedColWidths []int) {
	adjustedColWidths = append([]int{}, colWidths...)
	groupWidths = make([]int, len(t.headerGroups))

	groupStartCol := 0
	for groupIndex, group := range t.headerGroups {
		// Calculate group width based on the grouped columns
		groupWidth := 0
		for ci := groupStartCol; ci < groupStartCol+group.colSpan; ci++ {
			groupWidth += colWidths[ci]
		}

		// Include separators and padding for inner columns to width
		if group.colSpan > 1 {
			groupWidth += (group.colSpan - 1) * (1 + 2*t.padding)
		}

		// Calculate group width based on padding and group title. If its
		// greater than the calculated groupWidth, append the extra space to the last group col
		contentWidth := 2*t.padding + utf8.RuneCountInString(group.header)
		if contentWidth > groupWidth {
			adjustedColWidths[groupStartCol+group.colSpan-1] += contentWidth - groupWidth
			groupWidth = contentWidth
		}

		groupWidths[groupIndex] = groupWidth
		groupStartCol += group.colSpan
	}
	return groupWidths, adjustedColWidths
}

// Measure string length excluding any Ansi color escape codes.
func measure(val string) int {
	return utf8.RuneCountInString(ansiEscapeRegex.ReplaceAllString(val, ""))
}
