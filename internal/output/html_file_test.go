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
// the file is readable by its owner only.
func TestGenerateHTML_WritesOwnerOnlyFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	path := filepath.Join(t.TempDir(), "report.html")
	if err := GenerateHTML(testutil.SampleStats(), path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("report mode = %o, want 600", perm)
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
