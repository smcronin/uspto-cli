package cmd

import "testing"

func TestExpectedArchiveName(t *testing.T) {
	tests := []struct {
		tag    string
		goos   string
		goarch string
		want   string
	}{
		{tag: "v0.1.2", goos: "windows", goarch: "amd64", want: "uspto-cli_0.1.2_windows_amd64.zip"},
		{tag: "0.1.2", goos: "linux", goarch: "amd64", want: "uspto-cli_0.1.2_linux_amd64.tar.gz"},
		{tag: "v1.0.0", goos: "darwin", goarch: "arm64", want: "uspto-cli_1.0.0_darwin_arm64.tar.gz"},
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
		{Name: "uspto-cli_0.1.2_windows_amd64.zip", BrowserDownloadURL: "https://example/win.zip"},
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
