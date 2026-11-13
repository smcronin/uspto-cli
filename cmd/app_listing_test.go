package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
)

// wrapperDoc is one document of the fake file wrapper.
type wrapperDoc struct{ id, code, date string }

// fakeWrapper is deliberately not in date order, so --sort changes the
// numbering, and mixes codes, so --codes does too.
var fakeWrapper = []wrapperDoc{
	{"IDSPEC", "SPEC", "2022-01-10"},
	{"IDNOA", "NOA", "2023-09-01"},
	{"IDCTNF1", "CTNF", "2022-08-15"},
	{"IDREM", "REM", "2022-11-20"},
	{"IDCTFR", "CTFR", "2023-03-02"},
	{"IDCTNF2", "CTNF", "2022-05-05"},
}

// wrapperServer serves fakeWrapper and applies documentCodes,
// officialDateFrom and officialDateTo the way the API does. Each document has
// a PDF and an XML archive whose content names the document.
func wrapperServer(t *testing.T) *httptest.Server {
	t.Helper()
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/documents"):
			q := r.URL.Query()
			codes := map[string]bool{}
			for _, c := range strings.Split(q.Get("documentCodes"), ",") {
				if c != "" {
					codes[c] = true
				}
			}
			var docs []string
			for _, d := range fakeWrapper {
				if len(codes) > 0 && !codes[d.code] {
					continue
				}
				if from := q.Get("officialDateFrom"); from != "" && d.date < from {
					continue
				}
				if to := q.Get("officialDateTo"); to != "" && d.date > to {
					continue
				}
				docs = append(docs, fmt.Sprintf(`{"applicationNumberText":"16123456","officialDate":%q,"documentIdentifier":%q,"documentCode":%q,"documentCodeDescriptionText":"doc %s","downloadOptionBag":[{"mimeTypeIdentifier":"PDF","downloadUrl":"%s/files/%s.pdf"},{"mimeTypeIdentifier":"XML","downloadUrl":"%s/files/%s.tar"}]}`,
					d.date, d.id, d.code, d.id, ts.URL, d.id, ts.URL, d.id))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"count":%d,"documentBag":[%s]}`, len(docs), strings.Join(docs, ","))
		case strings.HasSuffix(r.URL.Path, ".tar"):
			id := strings.TrimSuffix(filepath.Base(r.URL.Path), ".tar")
			_, _ = w.Write(xmlArchive(t, "Text of "+id))
		default:
			id := strings.TrimSuffix(filepath.Base(r.URL.Path), ".pdf")
			_, _ = w.Write([]byte("pdf of " + id))
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

// listingFlags is one combination of the flags app docs, app download and
// app text share.
type listingFlags struct{ codes, from, to, sort string }

// listingHarness points the client at ts and resets every flag of the three
// commands, restoring them when the test ends.
func listingHarness(t *testing.T, ts *httptest.Server, format string) {
	t.Helper()
	origClient := api.DefaultClient
	origQuiet, origFormat, origMinify, origDryRun := flagQuiet, flagFormat, flagMinify, flagDryRun
	origDocs := [4]string{appDocsCodesFlag, appDocsFromFlag, appDocsToFlag, appDocsSortFlag}
	origDL := [6]string{appDownloadOutputFlag, appDownloadCodesFlag, appDownloadFromFlag, appDownloadToFlag, appDownloadSortFlag, appDownloadAsFlag}
	origText := [5]string{appTextCodesFlag, appTextFromFlag, appTextToFlag, appTextSortFlag, appTextAsFlag}
	origLatest := appTextLatestFlag
	t.Cleanup(func() {
		api.DefaultClient = origClient
		flagQuiet, flagFormat, flagMinify, flagDryRun = origQuiet, origFormat, origMinify, origDryRun
		appDocsCodesFlag, appDocsFromFlag, appDocsToFlag, appDocsSortFlag = origDocs[0], origDocs[1], origDocs[2], origDocs[3]
		appDownloadOutputFlag, appDownloadCodesFlag, appDownloadFromFlag, appDownloadToFlag, appDownloadSortFlag, appDownloadAsFlag = origDL[0], origDL[1], origDL[2], origDL[3], origDL[4], origDL[5]
		appTextCodesFlag, appTextFromFlag, appTextToFlag, appTextSortFlag, appTextAsFlag = origText[0], origText[1], origText[2], origText[3], origText[4]
		appTextLatestFlag = origLatest
	})
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))
	flagQuiet, flagFormat, flagMinify, flagDryRun = true, format, true, false
	setListingFlags(listingFlags{})
	appDownloadOutputFlag, appDownloadAsFlag = "", "pdf"
	appTextAsFlag, appTextLatestFlag = "auto", false
}

// setListingFlags gives all three commands the same listing flags.
func setListingFlags(f listingFlags) {
	appDocsCodesFlag, appDocsFromFlag, appDocsToFlag, appDocsSortFlag = f.codes, f.from, f.to, f.sort
	appDownloadCodesFlag, appDownloadFromFlag, appDownloadToFlag, appDownloadSortFlag = f.codes, f.from, f.to, f.sort
	appTextCodesFlag, appTextFromFlag, appTextToFlag, appTextSortFlag = f.codes, f.from, f.to, f.sort
}

// docsListing runs `app docs` and returns the documentIdentifiers in the
// order (and so with the numbering) it reports.
func docsListing(t *testing.T) []string {
	t.Helper()
	stdout, _, err := captureTrademarkOutput(t, func() error {
		return appDocsCmd.RunE(appDocsCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("app docs error: %v", err)
	}
	var env struct {
		Results []AppDocumentSummary `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("app docs output: %v\n%s", err, stdout)
	}
	ids := make([]string, len(env.Results))
	for i, d := range env.Results {
		if d.Index != i+1 {
			t.Fatalf("app docs index %d at position %d", d.Index, i+1)
		}
		ids[i] = d.DocumentIdentifier
	}
	return ids
}

// downloadedID runs `app download` with the selector and returns the
// documentIdentifier it reports and the content it saved.
func downloadedID(t *testing.T, selector string) (id, content string) {
	t.Helper()
	appDownloadOutputFlag = filepath.Join(t.TempDir(), "doc.pdf")
	stdout, _, err := captureTrademarkOutput(t, func() error {
		return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456", selector})
	})
	if err != nil {
		t.Fatalf("app download %s error: %v", selector, err)
	}
	var env struct {
		Results struct {
			DocumentIdentifier string `json:"documentIdentifier"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("app download output: %v\n%s", err, stdout)
	}
	data, err := os.ReadFile(appDownloadOutputFlag)
	if err != nil {
		t.Fatal(err)
	}
	return env.Results.DocumentIdentifier, string(data)
}

// textID runs `app text` with the selector and returns the documentIdentifier
// and text it reports.
func textID(t *testing.T, selector string) (id, text string) {
	t.Helper()
	stdout, _, err := captureTrademarkOutput(t, func() error {
		return appTextCmd.RunE(appTextCmd, []string{"16123456", selector})
	})
	if err != nil {
		t.Fatalf("app text %s error: %v", selector, err)
	}
	var env struct {
		Results AppDocumentTextResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("app text output: %v\n%s", err, stdout)
	}
	return env.Results.DocumentIdentifier, env.Results.Text
}

// The same flags give the same numbering in app docs, app download and app
// text, so index N is the same document in all three.
func TestDocumentIndex_SameFlagsSameDocumentInDocsDownloadAndText(t *testing.T) {
	combos := []struct {
		name    string
		flags   listingFlags
		wantIDs string // what app docs must list, in order
	}{
		{"no flags", listingFlags{}, "IDSPEC,IDNOA,IDCTNF1,IDREM,IDCTFR,IDCTNF2"},
		{"codes", listingFlags{codes: "CTNF,CTFR,NOA"}, "IDNOA,IDCTNF1,IDCTFR,IDCTNF2"},
		{"code alias", listingFlags{codes: "rejection"}, "IDCTNF1,IDCTFR,IDCTNF2"},
		{"date range", listingFlags{from: "2022-06-01", to: "2023-03-31"}, "IDCTNF1,IDREM,IDCTFR"},
		{"sort ascending", listingFlags{sort: "date:asc"}, "IDSPEC,IDCTNF2,IDCTNF1,IDREM,IDCTFR,IDNOA"},
		{"sort descending", listingFlags{sort: "date:desc"}, "IDNOA,IDCTFR,IDREM,IDCTNF1,IDCTNF2,IDSPEC"},
		{"codes, range and sort", listingFlags{codes: "CTNF,CTFR,NOA", from: "2022-06-01", sort: "date:desc"}, "IDNOA,IDCTFR,IDCTNF1"},
	}
	for _, combo := range combos {
		t.Run(combo.name, func(t *testing.T) {
			ts := wrapperServer(t)
			listingHarness(t, ts, "json")
			setListingFlags(combo.flags)

			listed := docsListing(t)
			if got := strings.Join(listed, ","); got != combo.wantIDs {
				t.Fatalf("app docs lists %s, want %s", got, combo.wantIDs)
			}
			for i, wantID := range listed {
				selector := fmt.Sprint(i + 1)
				if id, content := downloadedID(t, selector); id != wantID || content != "pdf of "+wantID {
					t.Errorf("app download %s -> %s (%q), want %s", selector, id, content, wantID)
				}
				if id, text := textID(t, selector); id != wantID || !strings.Contains(text, "Text of "+wantID) {
					t.Errorf("app text %s -> %s (%q), want %s", selector, id, text, wantID)
				}
			}
		})
	}
}

// A documentIdentifier taken from a filtered, sorted listing selects that
// document with no flags at all, and also when the caller's filters would
// have excluded it. This is the workflow the skill teaches.
func TestDocumentIdentifier_SelectsTheSameDocumentWhateverTheFlags(t *testing.T) {
	ts := wrapperServer(t)
	listingHarness(t, ts, "json")

	setListingFlags(listingFlags{codes: "CTNF,CTFR,NOA", sort: "date:desc"})
	listed := docsListing(t)
	if len(listed) != 4 {
		t.Fatalf("app docs lists %v, want 4 documents", listed)
	}
	second := listed[1] // "document 2" of the filtered listing

	for _, flags := range []listingFlags{
		{}, // the skill's old example: no flags on the download
		{codes: "SPEC"},
		{from: "2099-01-01"},
		{sort: "date:asc"},
	} {
		setListingFlags(flags)
		if id, content := downloadedID(t, second); id != second || content != "pdf of "+second {
			t.Errorf("flags %+v: app download %s -> %s (%q)", flags, second, id, content)
		}
		if id, _ := textID(t, strings.ToLower(second)); id != second {
			t.Errorf("flags %+v: app text %s -> %s (identifiers match case-insensitively)", flags, second, id)
		}
	}

	// The bare index, by contrast, depends on the flags: without them,
	// "2" is the second document of the whole file wrapper.
	setListingFlags(listingFlags{})
	if id, _ := downloadedID(t, "2"); id == second {
		t.Errorf("index 2 without the listing's flags still resolved to %s; the test wrapper should make the two differ", second)
	}
}

func TestAppDocsTable_ShowsDocumentIdentifierColumn(t *testing.T) {
	ts := wrapperServer(t)
	listingHarness(t, ts, "table")

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return appDocsCmd.RunE(appDocsCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("app docs error: %v", err)
	}
	header := strings.ToUpper(strings.SplitN(stdout, "\n", 3)[1])
	if !strings.Contains(header, "DOCUMENT ID") {
		t.Errorf("table header lacks the Document ID column:\n%s", stdout)
	}
	for _, d := range fakeWrapper {
		if !strings.Contains(stdout, d.id) {
			t.Errorf("table does not show documentIdentifier %s:\n%s", d.id, stdout)
		}
	}
}

// The listing shown when no selector is given is the same table, so it
// offers the identifiers too.
func TestAppDownload_NoSelectorListsIdentifiers(t *testing.T) {
	ts := wrapperServer(t)
	listingHarness(t, ts, "table")
	appDownloadCodesFlag = "NOA"

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("app download error: %v", err)
	}
	if !strings.Contains(stdout, "IDNOA") || strings.Contains(stdout, "IDSPEC") {
		t.Errorf("listing should show only the NOA document with its identifier:\n%s", stdout)
	}
	if !strings.Contains(stderr, "documentIdentifier") {
		t.Errorf("prompt should point at the documentIdentifier:\n%s", stderr)
	}
}

func TestDocumentListing_RejectsBadSortBeforeAnyRequest(t *testing.T) {
	ts := wrapperServer(t)
	listingHarness(t, ts, "json")
	requests := 0
	counter := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	t.Cleanup(counter.Close)
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(counter.URL))

	setListingFlags(listingFlags{sort: "code:asc"})
	for name, run := range map[string]func() error{
		"docs":     func() error { return appDocsCmd.RunE(appDocsCmd, []string{"16123456"}) },
		"download": func() error { return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456", "1"}) },
		"text":     func() error { return appTextCmd.RunE(appTextCmd, []string{"16123456", "1"}) },
	} {
		err := run()
		if err == nil {
			t.Errorf("app %s accepted --sort code:asc", name)
			continue
		}
		if code, _ := usageEnvelope(t, err); code != 2 {
			t.Errorf("app %s: exit %d for a bad --sort, want 2", name, code)
		}
	}
	if requests != 0 {
		t.Errorf("a bad --sort sent %d request(s), want 0", requests)
	}
}
