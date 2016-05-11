package util

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strings"
)

type Alignment uint8

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

var (
	ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// This struct models a header group.
type TableHeaderGroup struct {
	Header  string
	ColSpan int
}

// This struct models a Table.
type Table struct {
	Headers      []string
	HeaderGroups []TableHeaderGroup
	Rows         [][]string
	Alignment    []Alignment
	Padding      int
}

// Write Table to stdout optionally striping ANSI escape characters.
func (t *Table) Write(to io.Writer, stripAnsiChars bool) {
	w := bufio.NewWriter(to)
	padding := strings.Repeat(" ", t.Padding)

	// Calculate col widths and use them to calculate group heading widths
	colWidths := t.colWidths()

	// Write header groups if defined
	if len(t.HeaderGroups) > 0 {
		groupWidths := t.groupWidths(colWidths)
		hLine := t.hLine(groupWidths)

		w.WriteString(hLine)
		w.WriteByte('|')
		for gIndex, g := range t.HeaderGroups {
			w.WriteString(padding)
			w.WriteString(t.align(g.Header, AlignCenter, groupWidths[gIndex], stripAnsiChars))
			w.WriteString(padding)
			w.WriteByte('|')
		}
		w.WriteString("\n")
		w.WriteString(hLine)
	}

	// Write headers
	hLine := t.hLine(colWidths)
	if len(t.HeaderGroups) == 0 {
		w.WriteString(hLine)
	}
	w.WriteByte('|')
	for colIndex, h := range t.Headers {
		w.WriteString(padding)
		w.WriteString(t.align(h, t.Alignment[colIndex], colWidths[colIndex], stripAnsiChars))
		w.WriteString(padding)
		w.WriteByte('|')
	}
	w.WriteString("\n")
	w.WriteString(hLine)

	// Write rows
	for _, row := range t.Rows {
		w.WriteByte('|')
		for colIndex, c := range row {
			w.WriteString(padding)
			w.WriteString(t.align(c, t.Alignment[colIndex], colWidths[colIndex], stripAnsiChars))
			w.WriteString(padding)
			w.WriteByte('|')
		}
		w.WriteString("\n")
	}

	// Write footer
	w.WriteString(hLine)

	w.Flush()
}

// Generate horizontal line.
func (t *Table) hLine(colWidths []int) string {
	buf := bytes.NewBufferString("")

	buf.WriteByte('+')
	for _, colWidth := range colWidths {
		buf.WriteString(strings.Repeat("-", colWidth+2*t.Padding))
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
		vLen = len(val)
	} else {
		vLen = measure(val)
	}

	switch align {
	case AlignLeft:
		return val + strings.Repeat(" ", maxWidth-vLen)
	case AlignRight:
		return strings.Repeat(" ", maxWidth-vLen) + val
	case AlignCenter:
		lPad := (maxWidth - vLen) / 2
		return strings.Repeat(" ", lPad) + val + strings.Repeat(" ", maxWidth-lPad-vLen)
	}

	panic("unknown alignment type")
}

// Calculate max width for each column.
func (t *Table) colWidths() []int {
	colWidths := make([]int, len(t.Headers))
	for colIndex, h := range t.Headers {
		maxWidth := len(h)
		for _, row := range t.Rows {
			cellWidth := measure(row[colIndex])
			if cellWidth > maxWidth {
				maxWidth = cellWidth
			}
		}

		colWidths[colIndex] = maxWidth
	}
	return colWidths
}

// Calculate max width for each header group.
func (t *Table) groupWidths(colWidths []int) []int {
	groupWidths := make([]int, len(t.HeaderGroups))

	groupStartCol := 0
	for groupIndex, group := range t.HeaderGroups {
		groupWidth := 0
		for ci := groupStartCol; ci < groupStartCol+group.ColSpan; ci++ {
			groupWidth += colWidths[ci]
		}

		// Include separators and padding for inner columns to width
		if group.ColSpan > 1 {
			groupWidth += (group.ColSpan - 1) * (1 + 2*t.Padding)
		}

		groupWidths[groupIndex] = groupWidth
		groupStartCol += group.ColSpan
	}
	return groupWidths
}

// Measure string length excluding any ANSI color escape codes.
func measure(val string) int {
	return len(ansiEscapeRegex.ReplaceAllString(val, ""))
}
