//go:build windows

package config

// Windows MoveFileEx/Rename provides the atomic replacement boundary; opening
// a directory for fsync is not supported through os.File.Sync.
func syncConfigDirectory(string) error { return nil }
