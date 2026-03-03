package cmd

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpectedArchiveName(t *testing.T) {
	tests := []struct {
		tag    string
		goos   string
		goarch string
		want   string
	}{
		{tag: "v0.1.2", goos: "windows", goarch: "amd64", want: "uspto_0.1.2_windows_amd64.zip"},
		{tag: "0.1.2", goos: "linux", goarch: "amd64", want: "uspto_0.1.2_linux_amd64.tar.gz"},
		{tag: "v1.0.0", goos: "darwin", goarch: "arm64", want: "uspto_1.0.0_darwin_arm64.tar.gz"},
	}

	for _, tc := range tests {
		got := expectedArchiveName(tc.tag, tc.goos, tc.goarch)
		if got != tc.want {
			t.Fatalf("expectedArchiveName(%q,%q,%q)=%q, want %q", tc.tag, tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestFindReleaseAssetByName(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "checksums.txt", BrowserDownloadURL: "https://example/checksums.txt"},
		{Name: "uspto_0.1.2_windows_amd64.zip", BrowserDownloadURL: "https://example/win.zip"},
	}

	got, ok := findReleaseAssetByName(assets, "checksums.txt")
	if !ok {
		t.Fatal("expected checksums asset to be found")
	}
	if got.BrowserDownloadURL != "https://example/checksums.txt" {
		t.Fatalf("unexpected url: %s", got.BrowserDownloadURL)
	}

	_, ok = findReleaseAssetByName(assets, "missing")
	if ok {
		t.Fatal("did not expect missing asset to be found")
	}
}

func TestFindReleaseAssetByNames(t *testing.T) {
	assets := []githubReleaseAsset{
		{Name: "uspto-cli_0.1.2_windows_amd64.zip", BrowserDownloadURL: "https://example/win.zip"},
	}

	got, name, ok := findReleaseAssetByNames(assets, []string{
		"uspto_0.1.2_windows_amd64.zip",
		"uspto-cli_0.1.2_windows_amd64.zip",
	})
	if !ok {
		t.Fatal("expected fallback legacy asset to be found")
	}
	if name != "uspto-cli_0.1.2_windows_amd64.zip" {
		t.Fatalf("unexpected matched name: %s", name)
	}
	if got.BrowserDownloadURL != "https://example/win.zip" {
		t.Fatalf("unexpected url: %s", got.BrowserDownloadURL)
	}
}

func TestLookupChecksum(t *testing.T) {
	checksums := `
aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  uspto-cli_0.1.2_linux_amd64.tar.gz
bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb *uspto-cli_0.1.2_windows_amd64.zip
`
	got, ok := lookupChecksum(checksums, "uspto-cli_0.1.2_windows_amd64.zip")
	if !ok {
		t.Fatal("expected checksum to be found")
	}
	if got != "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("unexpected checksum: %s", got)
	}

	_, ok = lookupChecksum(checksums, "missing")
	if ok {
		t.Fatal("did not expect missing checksum entry")
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "v0.1.2", want: "0.1.2"},
		{in: "0.1.2", want: "0.1.2"},
		{in: " V1.2.3 ", want: "1.2.3"},
		{in: "dev", want: "dev"},
	}

	for _, tc := range tests {
		got := normalizeVersion(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeVersion(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExpectedArchiveNames(t *testing.T) {
	got := expectedArchiveNames("v1.2.3", "windows", "amd64")
	if len(got) != 2 {
		t.Fatalf("expected 2 archive names, got %d", len(got))
	}
	if got[0] != "uspto_1.2.3_windows_amd64.zip" {
		t.Fatalf("unexpected primary archive name: %s", got[0])
	}
	if got[1] != "uspto-cli_1.2.3_windows_amd64.zip" {
		t.Fatalf("unexpected legacy archive name: %s", got[1])
	}
}

func TestTargetExecutablePath(t *testing.T) {
	dir := t.TempDir()
	legacyName := "uspto-cli"
	newName := "uspto"
	if runtime.GOOS == "windows" {
		legacyName += ".exe"
		newName += ".exe"
	}
	legacyPath := filepath.Join(dir, legacyName)
	newPath := filepath.Join(dir, newName)

	got := targetExecutablePath(legacyPath)
	if got != newPath {
		t.Fatalf("targetExecutablePath(%q) = %q, want %q", legacyPath, got, newPath)
	}

	got = targetExecutablePath(newPath)
	if got != newPath {
		t.Fatalf("targetExecutablePath(%q) = %q, want %q", newPath, got, newPath)
	}
}
