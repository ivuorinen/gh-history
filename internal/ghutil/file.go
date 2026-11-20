package ghutil

import (
	"errors"
	"io/fs"
	"os"
)

// WriteOwnerOnly writes data to path, leaving the file readable and writable by
// its owner only. A report can name private repositories, so every report file
// goes through here.
//
// os.WriteFile applies its mode only when it creates the file, so a report
// written over one from an earlier version, mode 0644, stayed world-readable.
// An existing file is therefore made owner-only before the new contents are
// written, and a failure to do so is returned rather than writing anyway.
func WriteOwnerOnly(path string, data []byte) error {
	if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
