package output

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// colDelim separates columns in terminal mode.
const colDelim = "  "

// minColWidth is the narrowest a column may be squeezed to before other columns
// are shrunk instead.
const minColWidth = 3

// table renders rows either as an aligned, width-bounded grid (terminal) or as
// tab-separated values (anything else, so output stays machine-readable when
// redirected). It replaces go-gh's tableprinter, of which this project used
// only the New/AddField/EndRow/Render subset — no headers, colours or per-field
// options.
type table struct {
	out      io.Writer
	isTTY    bool
	maxWidth int
	rows     [][]string
	current  []string
}

func newTable(out io.Writer, isTTY bool, maxWidth int) *table {
	return &table{out: out, isTTY: isTTY, maxWidth: maxWidth}
}

// AddField appends a cell to the row being built.
func (t *table) AddField(s string) {
	t.current = append(t.current, s)
}

// EndRow completes the row being built.
func (t *table) EndRow() {
	t.rows = append(t.rows, t.current)
	t.current = nil
}

// Render writes the table. In non-terminal mode fields are written
// tab-separated and never truncated.
func (t *table) Render() error {
	if len(t.rows) == 0 {
		return nil
	}

	if !t.isTTY {
		for _, row := range t.rows {
			if _, err := fmt.Fprintln(t.out, strings.Join(row, "\t")); err != nil {
				return err
			}
		}
		return nil
	}

	widths := t.columnWidths()
	for _, row := range t.rows {
		for col, cell := range row {
			if col > 0 {
				if _, err := fmt.Fprint(t.out, colDelim); err != nil {
					return err
				}
			}
			cell = truncate(widths[col], cell)
			// The final column is not padded: trailing whitespace serves no
			// purpose and shows up in copied output.
			if col < len(row)-1 {
				cell = padRight(widths[col], cell)
			}
			if _, err := fmt.Fprint(t.out, cell); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(t.out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// columnWidths sizes each column to its widest cell, then, if the row would
// overflow maxWidth, repeatedly shrinks the widest column until it fits.
func (t *table) columnWidths() []int {
	numCols := 0
	for _, row := range t.rows {
		numCols = max(numCols, len(row))
	}

	widths := make([]int, numCols)
	for _, row := range t.rows {
		for col, cell := range row {
			widths[col] = max(widths[col], utf8.RuneCountInString(cell))
		}
	}

	total := func() int {
		sum := len(colDelim) * (numCols - 1)
		for _, w := range widths {
			sum += w
		}
		return sum
	}

	for total() > t.maxWidth {
		widest, idx := 0, -1
		for i, w := range widths {
			if w > widest {
				widest, idx = w, i
			}
		}
		if idx < 0 || widest <= minColWidth {
			break // nothing left to give
		}
		widths[idx]--
	}

	return widths
}

// truncate shortens s to fit width, marking the cut with an ellipsis. It never
// pads.
func truncate(width int, s string) string {
	if width <= 0 || utf8.RuneCountInString(s) <= width {
		return s
	}
	r := []rune(s)
	if width <= 3 {
		return string(r[:width])
	}
	return string(r[:width-3]) + "..."
}
