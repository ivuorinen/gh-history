package output

import (
	"strings"
	"testing"
)

func render(t *testing.T, isTTY bool, maxWidth int, rows [][]string) string {
	t.Helper()
	var b strings.Builder
	tbl := newTable(&b, isTTY, maxWidth)
	for _, row := range rows {
		for _, cell := range row {
			tbl.AddField(cell)
		}
		tbl.EndRow()
	}
	if err := tbl.Render(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// Non-terminal output must stay machine-readable: tab-separated, never padded,
// never truncated, however narrow the nominal width.
func TestTable_NonTTYIsTabSeparated(t *testing.T) {
	got := render(t, false, 10, [][]string{
		{"Total Events", "1,234"},
		{"a-very-long-value-well-past-the-width", "9"},
	})
	want := "Total Events\t1,234\na-very-long-value-well-past-the-width\t9\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTable_TTYAlignsColumns(t *testing.T) {
	got := render(t, true, 80, [][]string{
		{"Total Events", "100"},
		{"PRs", "7"},
	})
	want := "Total Events  100\nPRs           7\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The last column is never padded — trailing whitespace is invisible noise that
// shows up when output is copied.
func TestTable_TTYDoesNotPadFinalColumn(t *testing.T) {
	out := render(t, true, 80, [][]string{{"a", "b"}, {"longer", "c"}})
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line has trailing whitespace: %q", line)
		}
	}
}

func TestTable_TTYTruncatesToFitWidth(t *testing.T) {
	got := render(t, true, 20, [][]string{
		{"1.", "some-org/an-extremely-long-repository-name", "5 events"},
	})
	line := strings.TrimSuffix(got, "\n")
	if len([]rune(line)) > 20 {
		t.Errorf("line is %d runes, exceeds max width 20: %q", len([]rune(line)), line)
	}
	if !strings.Contains(line, "...") {
		t.Errorf("expected an ellipsis marking the truncation, got %q", line)
	}
	// The narrow column keeps its content; the wide one absorbs the shrinking.
	if !strings.HasPrefix(line, "1.") {
		t.Errorf("expected the rank column intact, got %q", line)
	}
}

func TestTable_Empty(t *testing.T) {
	if got := render(t, true, 80, nil); got != "" {
		t.Errorf("expected no output for an empty table, got %q", got)
	}
}

func TestTable_RaggedRows(t *testing.T) {
	// Rows of differing lengths must not panic or misalign.
	got := render(t, true, 80, [][]string{
		{"a", "b", "c"},
		{"d"},
	})
	if !strings.Contains(got, "a") || !strings.Contains(got, "d") {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		width int
		in    string
		want  string
	}{
		{10, "short", "short"},
		{5, "exact", "exact"},
		{5, "toolong", "to..."},
		{3, "toolong", "too"},
		{1, "toolong", "t"},
		{0, "toolong", "toolong"}, // width 0 means "unbounded", not "empty"
		{4, "äöüßx", "ä..."},      // counted in runes
	}
	for _, tc := range tests {
		if got := truncate(tc.width, tc.in); got != tc.want {
			t.Errorf("truncate(%d, %q) = %q, want %q", tc.width, tc.in, got, tc.want)
		}
	}
}
