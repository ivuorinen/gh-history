package output

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ivuorinen/gh-history/internal/testutil"
)

// A report can name private repositories and their pull request titles, so
// the file is readable by its owner only, including one written over an
// existing file at another mode (write-only here; see TestWriteOwnerOnly).
func TestGenerateHTML_WritesOwnerOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "report.html")
		if existing {
			if err := os.WriteFile(path, []byte("old report"), 0o200); err != nil {
				t.Fatal(err)
			}
		}
		if err := GenerateHTML(testutil.SampleStats(), path); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("existing file %v: report mode = %o, want 600", existing, perm)
		}
	}
}

// GenerateHTML writes where it is told and creates no directory, as --output
// does for every other format.
func TestGenerateHTML_MissingDirectoryIsAnError(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	if err := GenerateHTML(testutil.SampleStats(), filepath.Join(dir, "report.html")); err == nil {
		t.Fatal("want an error for a directory that does not exist")
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the missing directory was created (stat error: %v)", err)
	}
}
