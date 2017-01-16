# cli-table 
[![CircleCI](https://circleci.com/gh/geckoboard/cli-table.svg?style=shield)](https://circleci.com/gh/geckoboard/cli-table)
[![Coverage Status](https://coveralls.io/repos/github/geckoboard/cli-table/badge.svg?branch=master)](https://coveralls.io/github/geckoboard/cli-table?branch=master)
[![GoDoc](https://godoc.org/github.com/geckoboard/cli-table?status.svg)](https://godoc.org/github.com/geckoboard/cli-table)

Fancy ASCII tables for your CLI with full ANSI support.

The cli-table package provides a simple API for rendering ASCII tables.
Its main features are:
- headers and header groups; header groups may span multiple header columns.
- user-selectable cell padding.
- per-column and per header group alignment selection (left, center, right).
- ANSI support. ANSI characters are properly handled and do not mess up width calculations.
- tables are rendered to any output sink implementing [io.Writer](https://golang.org/pkg/io/#Writer)

## Getting started

```go
package main

import (
	"os"

	"github.com/geckoboard/cli-table"
)

func main() {
	t := table.New(3)

	// Set headers
	t.SetHeader(0, "left", table.AlignLeft)
	t.SetHeader(1, "center", table.AlignCenter)
	t.SetHeader(2, "right", table.AlignRight)

	// Optionally define header groups
	t.AddHeaderGroup(2, "left group", table.AlignCenter)
	t.AddHeaderGroup(1, "right group", table.AlignRight)

	// Append single row
	t.Append([]string{"1", "2", "3"})

	// Or append a bunch of rows which may contain ANSI characters
	t.Append(
		[]string{"1", "2", "3"},
		[]string{"\033[33mfour\033[0m", "five", "six"},
	)

	// Render table and strip out ANSI characters
	t.Write(os.Stdout, table.StripAnsi)

	// If your terminal does support ANSI characters you can use:
	// t.Write(os.Stdout, table.PreserveAnsi)
}
```

Running this example would render the following output:

```
+---------------+---------------+
|  left group   |   right group |
+---------------+---------------+
| left | center |         right |
+------+--------+---------------+
| 1    |   2    |             3 |
| 1    |   2    |             3 |
| four |  five  |           six |
+------+--------+---------------+
```

