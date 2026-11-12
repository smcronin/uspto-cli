package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
)

// dryRunHarness points the API client at a server that counts requests, and
// restores every global the dry-run tests touch.
func dryRunHarness(t *testing.T, format string) *int32 {
	t.Helper()
	origClient := api.DefaultClient
	origDryRun, origFormat, origQuiet, origMinify := flagDryRun, flagFormat, flagQuiet, flagMinify
	origSearch := searchFlags
	origDLOut, origDLCodes, origDLAs := appDownloadOutputFlag, appDownloadCodesFlag, appDownloadAsFlag
	origAllOut, origAllCodes, origAllFrom, origAllTo, origAllAs := appDownloadAllOutputFlag, appDownloadAllCodesFlag, appDownloadAllFromFlag, appDownloadAllToFlag, appDownloadAllAsFlag
	origTextCodes, origTextFrom, origTextTo, origTextAs, origTextLatest := appTextCodesFlag, appTextFromFlag, appTextToFlag, appTextAsFlag, appTextLatestFlag
	origTextAllCodes, origTextAllFrom, origTextAllTo, origTextAllAs := appTextAllCodesFlag, appTextAllFromFlag, appTextAllToFlag, appTextAllAsFlag
	origBundleOut, origBundleType := patentBundleOutputDir, patentBundleIDType

	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"patentFileWrapperDataBag":[],"documentBag":[]}`))
	}))
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

	t.Cleanup(func() {
		ts.Close()
		api.DefaultClient = origClient
		flagDryRun, flagFormat, flagQuiet, flagMinify = origDryRun, origFormat, origQuiet, origMinify
		searchFlags = origSearch
		appDownloadOutputFlag, appDownloadCodesFlag, appDownloadAsFlag = origDLOut, origDLCodes, origDLAs
		appDownloadAllOutputFlag, appDownloadAllCodesFlag, appDownloadAllFromFlag, appDownloadAllToFlag, appDownloadAllAsFlag = origAllOut, origAllCodes, origAllFrom, origAllTo, origAllAs
		appTextCodesFlag, appTextFromFlag, appTextToFlag, appTextAsFlag, appTextLatestFlag = origTextCodes, origTextFrom, origTextTo, origTextAs, origTextLatest
		appTextAllCodesFlag, appTextAllFromFlag, appTextAllToFlag, appTextAllAsFlag = origTextAllCodes, origTextAllFrom, origTextAllTo, origTextAllAs
		patentBundleOutputDir, patentBundleIDType = origBundleOut, origBundleType
	})

	flagDryRun, flagFormat, flagQuiet, flagMinify = true, format, false, false
	appDownloadOutputFlag, appDownloadCodesFlag, appDownloadAsFlag = "", "", "pdf"
	appDownloadAllOutputFlag, appDownloadAllCodesFlag, appDownloadAllFromFlag, appDownloadAllToFlag, appDownloadAllAsFlag = "", "", "", "", "pdf"
	appTextCodesFlag, appTextFromFlag, appTextToFlag, appTextAsFlag, appTextLatestFlag = "", "", "", "auto", false
	appTextAllCodesFlag, appTextAllFromFlag, appTextAllToFlag, appTextAllAsFlag = "", "", "", "auto"
	patentBundleOutputDir, patentBundleIDType = "", idTypeAuto
	resetSearchFlagsForTest()
	return &requests
}

// A dry run prints the request it would make on stderr, sends nothing, and
// leaves stdout empty in every output format.
func TestDryRun_NoRequestsAndNoStdout(t *testing.T) {
	const documents = "GET /api/v1/patent/applications/16123456/documents"
	tests := []struct {
		name       string
		setup      func()
		run        func() error
		wantStderr []string
	}{
		{
			name:       "app download with index",
			setup:      func() { appDownloadCodesFlag = "rejection" },
			run:        func() error { return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456", "3"}) },
			wantStderr: []string{documents, "documentCodes=CTNF,CTFR", "Then: GET <pdf downloadUrl of document 3> -> 16123456_<officialDate>_<documentCode>.pdf"},
		},
		{
			name:       "app download with output path",
			setup:      func() { appDownloadOutputFlag = "oa.docx"; appDownloadAsFlag = "docx" },
			run:        func() error { return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456", "LMGEU99F"}) },
			wantStderr: []string{documents, "Then: GET <docx downloadUrl of document LMGEU99F> -> oa.docx"},
		},
		{
			name:       "app download without index",
			run:        func() error { return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456"}) },
			wantStderr: []string{documents, "nothing is downloaded"},
		},
		{
			name:       "app download-all",
			setup:      func() { appDownloadAllFromFlag = "2023-01-01" },
			run:        func() error { return appDownloadAllCmd.RunE(appDownloadAllCmd, []string{"16123456"}) },
			wantStderr: []string{documents, "officialDateFrom=2023-01-01", "for each listed document"},
		},
		{
			name:       "app text latest",
			setup:      func() { appTextLatestFlag = true; appTextCodesFlag = "NOA" },
			run:        func() error { return appTextCmd.RunE(appTextCmd, []string{"16123456"}) },
			wantStderr: []string{documents, "documentCodes=NOA", "latest listed document"},
		},
		{
			name:       "app text with selector",
			run:        func() error { return appTextCmd.RunE(appTextCmd, []string{"16123456", "2"}) },
			wantStderr: []string{documents, "downloadUrl of document 2"},
		},
		{
			name:       "app text-all",
			run:        func() error { return appTextAllCmd.RunE(appTextAllCmd, []string{"16123456"}) },
			wantStderr: []string{documents, "for each listed document"},
		},
		{
			name:       "patent bundle",
			run:        func() error { return runPatentBundle(patentBundleCmd, []string{"US20050021049A1"}) },
			wantStderr: []string{"Patent bundle dry-run", "GET /api/v1/patent/applications/<appNumber>/documents"},
		},
		{
			name:       "search single page",
			setup:      func() { searchFlags.title = "battery" },
			run:        func() error { return runSearch(searchCmd, nil) },
			wantStderr: []string{"GET /api/v1/patent/applications/search", "limit=25", "q=applicationMetaData.inventionTitle:battery"},
		},
		{
			name:       "search POST",
			setup:      func() { searchFlags.title = "battery"; searchFlags.granted = true },
			run:        func() error { return runSearch(searchCmd, nil) },
			wantStderr: []string{"POST /api/v1/patent/applications/search", `"limit": 25`},
		},
		{
			name:       "search all pages",
			setup:      func() { searchFlags.title = "battery"; searchFlags.all = true },
			run:        func() error { return runSearch(searchCmd, nil) },
			wantStderr: []string{"GET /api/v1/patent/applications/search", "limit=100", "Then: repeat with offset increased by 100"},
		},
		{
			name:       "search count only",
			setup:      func() { searchFlags.title = "battery"; searchFlags.countOnly = true },
			run:        func() error { return runSearch(searchCmd, nil) },
			wantStderr: []string{"GET /api/v1/patent/applications/search", "limit=1"},
		},
	}

	for _, format := range []string{"json", "ndjson", "csv", "table"} {
		for _, tt := range tests {
			t.Run(format+"/"+tt.name, func(t *testing.T) {
				requests := dryRunHarness(t, format)
				if tt.setup != nil {
					tt.setup()
				}

				stdout, stderr, err := captureTrademarkOutput(t, tt.run)
				if err != nil {
					t.Fatalf("dry run returned an error: %v", err)
				}
				if got := atomic.LoadInt32(requests); got != 0 {
					t.Errorf("dry run sent %d API request(s), want 0", got)
				}
				if stdout != "" {
					t.Errorf("dry run wrote to stdout, want nothing:\n%s", stdout)
				}
				for _, want := range tt.wantStderr {
					if !strings.Contains(stderr, want) {
						t.Errorf("stderr lacks %q:\n%s", want, stderr)
					}
				}
			})
		}
	}
}

func TestDryRun_SearchAllCSVPrintsOneRequest(t *testing.T) {
	requests := dryRunHarness(t, "csv")
	searchFlags.title = "battery"
	searchFlags.all = true

	stdout, stderr, err := captureTrademarkOutput(t, func() error { return runSearch(searchCmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(requests) != 0 || stdout != "" {
		t.Errorf("requests=%d stdout=%q, want no requests and empty stdout", atomic.LoadInt32(requests), stdout)
	}
	if got := strings.Count(stderr, "GET /api/v1/patent/applications/search"); got != 1 {
		t.Errorf("stderr shows %d search requests, want 1:\n%s", got, stderr)
	}
}

func TestDryRun_DownloadAllDoesNotCreateOutputDirectory(t *testing.T) {
	requests := dryRunHarness(t, "table")
	outDir := filepath.Join(t.TempDir(), "not", "created")
	appDownloadAllOutputFlag = outDir

	_, stderr, err := captureTrademarkOutput(t, func() error {
		return appDownloadAllCmd.RunE(appDownloadAllCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(outDir); !os.IsNotExist(statErr) {
		t.Errorf("dry run created the output directory (stat err = %v)", statErr)
	}
	if atomic.LoadInt32(requests) != 0 {
		t.Errorf("dry run sent %d API request(s), want 0", atomic.LoadInt32(requests))
	}
	if !strings.Contains(stderr, filepath.Join(outDir, "16123456_<officialDate>_<documentCode>.pdf")) {
		t.Errorf("stderr should name the output location:\n%s", stderr)
	}
}

// When a command needs the caller to pick a document it shows the list. That
// prompt must not land on stdout when the caller asked for machine-readable
// output.
func TestDocumentChoices_GoToStderrInMachineFormats(t *testing.T) {
	const listing = `{"documentBag":[
		{"applicationNumberText":"16123456","officialDate":"2020-01-02","documentIdentifier":"DOC1","documentCode":"CTNF","documentCodeDescriptionText":"Non-Final Rejection","downloadOptionBag":[{"mimeTypeIdentifier":"PDF","downloadUrl":"/d/1.pdf"}]},
		{"applicationNumberText":"16123456","officialDate":"2020-03-04","documentIdentifier":"DOC2","documentCode":"NOA","documentCodeDescriptionText":"Notice of Allowance","downloadOptionBag":[{"mimeTypeIdentifier":"PDF","downloadUrl":"/d/2.pdf"}]}
	]}`
	commands := []struct {
		name string
		run  func() error
	}{
		{name: "app download", run: func() error { return appDownloadCmd.RunE(appDownloadCmd, []string{"16123456"}) }},
		{name: "app text", run: func() error { return appTextCmd.RunE(appTextCmd, []string{"16123456"}) }},
	}

	for _, format := range []string{"json", "ndjson", "csv", "table"} {
		for _, command := range commands {
			t.Run(format+"/"+command.name, func(t *testing.T) {
				dryRunHarness(t, format)
				flagDryRun = false
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(listing))
				}))
				defer ts.Close()
				api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))

				stdout, stderr, err := captureTrademarkOutput(t, command.run)
				if err != nil {
					t.Fatalf("command error: %v", err)
				}
				if !strings.Contains(stderr, "Found 2 documents") {
					t.Errorf("stderr lacks the selection prompt:\n%s", stderr)
				}

				if format == "table" {
					if !strings.Contains(stdout, "Non-Final Rejection") {
						t.Errorf("table format should keep the document list on stdout:\n%s", stdout)
					}
					return
				}
				if stdout != "" {
					t.Errorf("-f %s wrote non-data output to stdout:\n%s", format, stdout)
				}
				if !strings.Contains(stderr, "Non-Final Rejection") {
					t.Errorf("document list should be on stderr:\n%s", stderr)
				}
			})
		}
	}
}

// A real (non-dry-run) search still produces its envelope; the dry-run early
// return must not swallow normal output.
func TestRunSearch_NonDryRunStillEmitsEnvelope(t *testing.T) {
	requests := dryRunHarness(t, "json")
	flagDryRun = false
	flagQuiet = true
	searchFlags.title = "battery"

	stdout, _, err := captureTrademarkOutput(t, func() error { return runSearch(searchCmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		OK         bool `json:"ok"`
		Pagination struct {
			Limit int `json:"limit"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
	}
	if !envelope.OK || envelope.Pagination.Limit != 25 || atomic.LoadInt32(requests) != 1 {
		t.Errorf("envelope=%+v requests=%d, want ok with limit 25 after one request", envelope, atomic.LoadInt32(requests))
	}
}
