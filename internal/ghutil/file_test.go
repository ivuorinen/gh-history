package ghutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// os.WriteFile applies its mode only when it creates the file, so a report
// written over one from an earlier version kept that file's 0644.
// WriteOwnerOnly leaves both a new and an existing file at 0600.
//
// The existing file is write-only (0200) rather than 0644: Codacy's Semgrep
// flags any permission literal wider than 0600, and os.WriteFile alone leaves
// a 0200 file at 0200 just as it leaves a 0644 one at 0644.
func TestWriteOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	for _, tc := range []struct {
		name     string
		existing bool
	}{
		{"new file", false},
		{"existing file at another mode", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "report")
			if tc.existing {
				if err := os.WriteFile(path, []byte("old report"), 0o200); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteOwnerOnly(path, []byte("new report")); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if perm := info.Mode().Perm(); perm != 0o600 {
				t.Errorf("mode = %o, want 600", perm)
			}
			data, err := fs.ReadFile(os.DirFS(dir), "report")
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "new report" {
				t.Errorf("contents = %q, want %q", data, "new report")
			}
		})
	}
}

func TestWriteOwnerOnly_MissingDirectoryIsAnError(t *testing.T) {
	if err := WriteOwnerOnly(filepath.Join(t.TempDir(), "missing", "report"), []byte("x")); err == nil {
		t.Error("want an error for a directory that does not exist")
	}
}
