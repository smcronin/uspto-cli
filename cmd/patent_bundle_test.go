package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
)

// Documents filed in one submission share an official date and document code,
// so their default file names are identical. Each must still land in its own
// file.
func TestDownloadBundlePDFs_SameDateAndCodeDoNotOverwrite(t *testing.T) {
	origClient := api.DefaultClient
	defer func() { api.DefaultClient = origClient }()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The body identifies which document was requested.
		_, _ = w.Write([]byte("pdf for " + r.URL.Path))
	}))
	defer ts.Close()
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	pdf := func(id string) []types.DownloadOption {
		return []types.DownloadOption{{MimeTypeIdentifier: "PDF", DownloadURL: ts.URL + "/download/" + id + ".pdf"}}
	}
	docs := []types.Document{
		{OfficialDate: "2020-03-04T10:15:00.000-0500", DocumentCode: "NPL", DocumentIdentifier: "A", DownloadOptionBag: pdf("A")},
		{OfficialDate: "2020-03-04T10:15:00.000-0500", DocumentCode: "NPL", DocumentIdentifier: "B", DownloadOptionBag: pdf("B")},
		{OfficialDate: "2020-03-04T10:15:00.000-0500", DocumentCode: "NPL", DocumentIdentifier: "C", DownloadOptionBag: pdf("C")},
		{OfficialDate: "2020-05-06T09:00:00.000-0400", DocumentCode: "CTNF", DocumentIdentifier: "D", DownloadOptionBag: pdf("D")},
		{OfficialDate: "2020-05-07T09:00:00.000-0400", DocumentCode: "XML.ONLY", DocumentIdentifier: "E",
			DownloadOptionBag: []types.DownloadOption{{MimeTypeIdentifier: "XML", DownloadURL: ts.URL + "/download/E.tar"}}},
	}

	outDir := filepath.Join(t.TempDir(), "pdf")
	results, downloaded, skipped, failed := downloadBundlePDFs(context.Background(), "16123456", docs, outDir)

	if downloaded != 4 || skipped != 1 || failed != 0 {
		t.Fatalf("downloaded=%d skipped=%d failed=%d, want 4/1/0", downloaded, skipped, failed)
	}
	if len(results) != 4 {
		t.Fatalf("results = %d, want 4", len(results))
	}

	// Every result has its own path, and each file holds its own document.
	seen := map[string]bool{}
	for i, id := range []string{"A", "B", "C", "D"} {
		path := results[i].Path
		if seen[path] {
			t.Errorf("result %d reuses path %s", i+1, path)
		}
		seen[path] = true
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		if want := "pdf for /download/" + id + ".pdf"; string(data) != want {
			t.Errorf("%s holds %q, want %q", filepath.Base(path), data, want)
		}
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Errorf("pdf directory holds %d files, want 4 (one per downloaded document)", len(entries))
	}

	// The first of a colliding group keeps the plain name; later ones record
	// what they were renamed from, as download-all does.
	base := filepath.Join(outDir, "16123456_2020-03-04T10-15-00.000-0500_NPL.pdf")
	if results[0].Path != base || results[0].RenamedFrom != "" {
		t.Errorf("first document: path=%s renamedFrom=%q, want the plain name and no rename", results[0].Path, results[0].RenamedFrom)
	}
	for i := 1; i <= 2; i++ {
		if results[i].RenamedFrom != base {
			t.Errorf("document %d renamedFrom = %q, want %q", i+1, results[i].RenamedFrom, base)
		}
	}
	if results[3].RenamedFrom != "" {
		t.Errorf("non-colliding document should not be renamed, got %q", results[3].RenamedFrom)
	}
}
