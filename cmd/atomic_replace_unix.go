//go:build !windows

package cmd

import "os"

func replaceFile(source, destination string, overwrite bool) error {
	if !overwrite {
		// source and destination share a directory, so an atomic hard link gives
		// us no-clobber semantics even if another process creates destination
		// after the caller's existence check.
		if err := os.Link(source, destination); err != nil {
			return err
		}
		return os.Remove(source)
	}
	return os.Rename(source, destination)
}
