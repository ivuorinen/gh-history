package ghutil

import (
	"os"
)

// WriteOwnerOnly writes data to path, leaving a regular file readable and
// writable by its owner only. A report can name private repositories, so every
// report file goes through here.
//
// os.WriteFile applies its mode only when it creates the file, so a report
// written over one from an earlier version, mode 0644, stayed world-readable.
// Here the file is opened first and, when it is a regular file, made 0600
// through the open descriptor before any data is written; a failure to do so
// is returned rather than writing anyway.
//
// Opening for writing fails for a directory, so an --output naming one is an
// error and the directory keeps its mode. A device or pipe (/dev/null,
// /dev/stdout) is written as before, with no permission change: the report is
// not stored in it, and its mode belongs to the system.
func WriteOwnerOnly(path string, data []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	// A failed close can mean the data never reached the file.
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		if err := f.Chmod(0o600); err != nil {
			return err
		}
	}
	_, err = f.Write(data)
	return err
}
