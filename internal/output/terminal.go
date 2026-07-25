package output

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// defaultWidth is used when stdout is not a terminal, or its size cannot be
// determined.
const defaultWidth = 80

// terminalOut reports where to write, whether that destination is a terminal,
// and how wide it is.
//
// GH_FORCE_TTY is honoured for parity with the gh CLI: any non-empty value
// forces terminal mode, and the value may additionally be a column count
// ("100") or a percentage of the real terminal width ("80%").
func terminalOut() (w io.Writer, isTTY bool, width int) {
	fd := int(os.Stdout.Fd())
	isTTY = term.IsTerminal(fd)

	width = defaultWidth
	if realWidth, _, err := term.GetSize(fd); err == nil && realWidth > 0 {
		width = realWidth
	}

	if spec := os.Getenv("GH_FORCE_TTY"); spec != "" {
		isTTY = true
		if forced, ok := forcedWidth(spec, width); ok {
			width = forced
		}
	}

	return os.Stdout, isTTY, width
}

// forcedWidth parses a GH_FORCE_TTY value as either an absolute column count or
// a percentage of realWidth. It reports false when the value is neither.
func forcedWidth(spec string, realWidth int) (int, bool) {
	if pct, ok := strings.CutSuffix(spec, "%"); ok {
		if p, err := strconv.Atoi(pct); err == nil && p > 0 {
			return realWidth * p / 100, true
		}
		return 0, false
	}
	if w, err := strconv.Atoi(spec); err == nil && w > 0 {
		return w, true
	}
	return 0, false
}
