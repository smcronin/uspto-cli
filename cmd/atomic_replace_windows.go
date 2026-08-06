//go:build windows

package cmd

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func replaceFile(source, destination string, overwrite bool) error {
	if !overwrite {
		from, err := windows.UTF16PtrFromString(source)
		if err != nil {
			return fmt.Errorf("encoding temporary path: %w", err)
		}
		to, err := windows.UTF16PtrFromString(destination)
		if err != nil {
			return fmt.Errorf("encoding destination path: %w", err)
		}
		// CreateHardLink fails atomically when destination already exists. The
		// temporary file is in the destination directory, so both paths share a
		// volume and this closes the precheck/rename no-clobber race on Windows.
		if err := windows.CreateHardLink(to, from, 0); err != nil {
			return fmt.Errorf("atomically creating destination without overwrite: %w", err)
		}
		if err := os.Remove(source); err != nil {
			_ = os.Remove(destination)
			return fmt.Errorf("removing linked temporary file: %w", err)
		}
		return nil
	}
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return fmt.Errorf("encoding temporary path: %w", err)
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return fmt.Errorf("encoding destination path: %w", err)
	}
	if err := windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH); err != nil {
		return fmt.Errorf("atomically replacing destination: %w", err)
	}
	return nil
}
