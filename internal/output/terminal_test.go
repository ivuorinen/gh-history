package output

import "testing"

func TestForcedWidth(t *testing.T) {
	const realWidth = 200

	tests := []struct {
		spec    string
		want    int
		wantOK  bool
		comment string
	}{
		{"100", 100, true, "absolute column count"},
		{"80%", 160, true, "percentage of the real width"},
		{"50%", 100, true, "percentage of the real width"},
		// Integer division floors small percentages to 0, which would leave the
		// renderer with no column budget at all.
		{"1%", minUsableWidth, true, "tiny percentage clamps to a usable width"},
		{"", 0, false, "empty is not a width"},
		{"true", 0, false, "non-numeric forces TTY but not a width"},
		{"abc%", 0, false, "non-numeric percentage"},
		{"0", 0, false, "zero is not a usable width"},
		{"-5", 0, false, "negative is not a usable width"},
		{"0%", 0, false, "zero percent is not a usable width"},
	}
	for _, tc := range tests {
		got, ok := forcedWidth(tc.spec, realWidth)
		if ok != tc.wantOK {
			t.Errorf("forcedWidth(%q): ok = %v, want %v (%s)", tc.spec, ok, tc.wantOK, tc.comment)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("forcedWidth(%q) = %d, want %d (%s)", tc.spec, got, tc.want, tc.comment)
		}
	}
}

// GH_FORCE_TTY forces terminal mode even when its value is not a width, so a
// rejected width must leave the detected width untouched rather than zeroing it.
func TestTerminalOut_ForceTTYWithoutWidth(t *testing.T) {
	t.Setenv("GH_FORCE_TTY", "true")
	_, isTTY, width := terminalOut()
	if !isTTY {
		t.Error("GH_FORCE_TTY should force terminal mode")
	}
	if width <= 0 {
		t.Errorf("width should stay positive, got %d", width)
	}
}

func TestTerminalOut_ForceTTYWithWidth(t *testing.T) {
	t.Setenv("GH_FORCE_TTY", "123")
	_, isTTY, width := terminalOut()
	if !isTTY {
		t.Error("GH_FORCE_TTY should force terminal mode")
	}
	if width != 123 {
		t.Errorf("width = %d, want 123", width)
	}
}
