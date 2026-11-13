package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
	"github.com/spf13/cobra"
)

// setFlagForTest sets a flag the way the command line would (marking it
// Changed) and restores its value and Changed state when the test ends.
func setFlagForTest(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		t.Fatalf("%s has no --%s flag", cmd.CommandPath(), name)
	}
	origValue, origChanged := flag.Value.String(), flag.Changed
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set --%s=%s: %v", name, value, err)
	}
	t.Cleanup(func() {
		_ = flag.Value.Set(origValue)
		flag.Changed = origChanged
	})
}

// exportRequest is one request the export test server received.
type exportRequest struct {
	Method string
	Path   string
	Limit  string // limit query parameter, or pagination.limit of a POST body
	Offset string
	Format string
}

// exportServer answers search requests with the given count and download
// requests with a two-line CSV, and records every request.
func exportServer(t *testing.T, count int) (*httptest.Server, func() []exportRequest) {
	t.Helper()
	var mu sync.Mutex
	var seen []exportRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := exportRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Limit:  r.URL.Query().Get("limit"),
			Offset: r.URL.Query().Get("offset"),
			Format: r.URL.Query().Get("format"),
		}
		if r.Method == http.MethodPost {
			var body struct {
				Format     string `json:"format"`
				Pagination struct {
					Limit  int `json:"limit"`
					Offset int `json:"offset"`
				} `json:"pagination"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode POST body: %v", err)
			}
			req.Limit, req.Offset, req.Format = fmt.Sprint(body.Pagination.Limit), fmt.Sprint(body.Pagination.Offset), body.Format
		}
		mu.Lock()
		seen = append(seen, req)
		mu.Unlock()

		if strings.HasSuffix(r.URL.Path, "/download") {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("applicationNumberText\n16123456\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"count":%d}`, count)
	}))
	t.Cleanup(ts.Close)
	return ts, func() []exportRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]exportRequest(nil), seen...)
	}
}

// exportHarness points the ODP client at ts and resets the flags the export
// paths read.
func exportHarness(t *testing.T, ts *httptest.Server, quiet bool) {
	t.Helper()
	origClient := api.DefaultClient
	origDryRun, origFormat, origQuiet, origMinify := flagDryRun, flagFormat, flagQuiet, flagMinify
	origSearch := searchFlags
	t.Cleanup(func() {
		api.DefaultClient = origClient
		flagDryRun, flagFormat, flagQuiet, flagMinify = origDryRun, origFormat, origQuiet, origMinify
		searchFlags = origSearch
	})
	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))
	flagDryRun, flagFormat, flagQuiet, flagMinify = false, "table", quiet, false
	resetSearchFlagsForTest()
}

const (
	patentSearchPath   = "/api/v1/patent/applications/search"
	patentDownloadPath = "/api/v1/patent/applications/search/download"
)

func TestExportLimit(t *testing.T) {
	tests := []struct {
		total, offset, want int
	}{
		{137, 0, 137},
		{10000, 0, 10000},
		{25000, 0, 10000},
		{137, 100, 37},
		{25000, 9000, 1000},
		{0, 0, 1},     // nothing to export; never fall back to the server's 25
		{137, 500, 1}, // offset past the end
	}
	for _, tt := range tests {
		if got := exportLimit(tt.total, tt.offset); got != tt.want {
			t.Errorf("exportLimit(%d, %d) = %d, want %d", tt.total, tt.offset, got, tt.want)
		}
	}
}

// Without --limit, --download counts first and asks for every row. It used
// to send the search default, limit=25, and export one page.
func TestRunSearch_DownloadWithoutLimitExportsEveryRow(t *testing.T) {
	tests := []struct {
		name       string
		setup      func()
		wantMethod string
	}{
		{name: "GET", setup: func() {}, wantMethod: http.MethodGet},
		{name: "POST", setup: func() { searchFlags.granted = true }, wantMethod: http.MethodPost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, requests := exportServer(t, 137)
			exportHarness(t, ts, false)
			tt.setup()
			setFlagForTest(t, searchCmd, "download", "csv")

			stdout, stderr, err := captureTrademarkOutput(t, func() error {
				return runSearch(searchCmd, []string{"battery"})
			})
			if err != nil {
				t.Fatalf("runSearch() error: %v", err)
			}

			got := requests()
			if len(got) != 2 {
				t.Fatalf("server saw %d requests, want a count and a download: %+v", len(got), got)
			}
			if got[0].Path != patentSearchPath || got[0].Limit != "1" {
				t.Errorf("first request = %+v, want a limit=1 count on %s", got[0], patentSearchPath)
			}
			if got[1].Path != patentDownloadPath || got[1].Method != tt.wantMethod {
				t.Errorf("second request = %+v, want %s %s", got[1], tt.wantMethod, patentDownloadPath)
			}
			if got[1].Limit != "137" || got[1].Format != "csv" {
				t.Errorf("download sent limit=%s format=%s, want limit=137 format=csv", got[1].Limit, got[1].Format)
			}
			if !strings.HasPrefix(stdout, "applicationNumberText\n16123456") {
				t.Errorf("stdout is not the raw export:\n%s", stdout)
			}
			if !strings.Contains(stderr, "Exporting 137 of 137 results.") {
				t.Errorf("stderr lacks the export size:\n%s", stderr)
			}
		})
	}
}

func TestRunSearch_DownloadRespectsExplicitLimit(t *testing.T) {
	ts, requests := exportServer(t, 137)
	exportHarness(t, ts, true)
	setFlagForTest(t, searchCmd, "download", "json")
	setFlagForTest(t, searchCmd, "limit", "10")

	if _, _, err := captureTrademarkOutput(t, func() error {
		return runSearch(searchCmd, []string{"battery"})
	}); err != nil {
		t.Fatalf("runSearch() error: %v", err)
	}

	got := requests()
	if len(got) != 1 || got[0].Path != patentDownloadPath {
		t.Fatalf("requests = %+v, want only the download (no count)", got)
	}
	if got[0].Limit != "10" || got[0].Format != "json" {
		t.Errorf("download sent limit=%s format=%s, want limit=10 format=json", got[0].Limit, got[0].Format)
	}
}

// --all used to be ignored when --download was present. It now means the
// same full export, and wins over --limit as it does for a paginated search.
func TestRunSearch_DownloadWithAllIsAFullExport(t *testing.T) {
	ts, requests := exportServer(t, 137)
	exportHarness(t, ts, true)
	setFlagForTest(t, searchCmd, "download", "csv")
	setFlagForTest(t, searchCmd, "limit", "10")
	searchFlags.all = true

	if _, _, err := captureTrademarkOutput(t, func() error {
		return runSearch(searchCmd, []string{"battery"})
	}); err != nil {
		t.Fatalf("runSearch() error: %v", err)
	}

	got := requests()
	if len(got) != 2 || got[1].Limit != "137" {
		t.Fatalf("requests = %+v, want a count then a download with limit=137", got)
	}
}

func TestRunSearch_DownloadCapsAtTenThousandAndSaysSoWhenQuiet(t *testing.T) {
	ts, requests := exportServer(t, 25000)
	exportHarness(t, ts, true)
	setFlagForTest(t, searchCmd, "download", "csv")

	_, stderr, err := captureTrademarkOutput(t, func() error {
		return runSearch(searchCmd, []string{"battery"})
	})
	if err != nil {
		t.Fatalf("runSearch() error: %v", err)
	}
	got := requests()
	if len(got) != 2 || got[1].Limit != "10000" {
		t.Fatalf("requests = %+v, want a download with limit=10000", got)
	}
	if !strings.Contains(stderr, "Warning: exporting 10000 of 25000 results") {
		t.Errorf("a truncated export must be reported on stderr even under -q:\n%s", stderr)
	}
}

func TestRunSearch_DownloadFromOffsetExportsTheRest(t *testing.T) {
	ts, requests := exportServer(t, 137)
	exportHarness(t, ts, true)
	setFlagForTest(t, searchCmd, "download", "csv")
	searchFlags.offset = 100

	if _, _, err := captureTrademarkOutput(t, func() error {
		return runSearch(searchCmd, []string{"battery"})
	}); err != nil {
		t.Fatalf("runSearch() error: %v", err)
	}
	got := requests()
	if len(got) != 2 || got[1].Limit != "37" || got[1].Offset != "100" {
		t.Fatalf("requests = %+v, want a download with offset=100 limit=37", got)
	}
}

func TestDownloadFormat_RejectsUnknownValues(t *testing.T) {
	newCmd := func() *cobra.Command {
		cmd := &cobra.Command{Use: "x"}
		addDownloadFlag(cmd)
		return cmd
	}

	t.Run("not set", func(t *testing.T) {
		got, err := downloadFormat(newCmd())
		if got != "" || err != nil {
			t.Errorf("downloadFormat() = %q, %v, want empty and nil", got, err)
		}
	})
	for value, want := range map[string]string{"json": "json", "csv": "csv", "CSV": "csv", " Json ": "json"} {
		t.Run("accepts "+value, func(t *testing.T) {
			cmd := newCmd()
			_ = cmd.Flags().Set("download", value)
			got, err := downloadFormat(cmd)
			if got != want || err != nil {
				t.Errorf("downloadFormat(%q) = %q, %v, want %q", value, got, err, want)
			}
		})
	}
	for _, value := range []string{"xlsx", "xml", "", "table"} {
		t.Run("rejects "+value, func(t *testing.T) {
			cmd := newCmd()
			_ = cmd.Flags().Set("download", value)
			_, err := downloadFormat(cmd)
			if err == nil {
				t.Fatalf("downloadFormat(%q) = nil error, want a usage error", value)
			}
			if code, errType := usageEnvelope(t, err); code != types.ExitInvalidArgs || errType != "INVALID_ARGUMENTS" {
				t.Errorf("exit %d type %q, want exit %d INVALID_ARGUMENTS", code, errType, types.ExitInvalidArgs)
			}
		})
	}
}

// A bad --download value is rejected before any request, on search and on
// every PTAB search command.
func TestDownload_UnknownFormatSendsNothing(t *testing.T) {
	commands := []struct {
		cmd *cobra.Command
		run func() error
	}{
		{searchCmd, func() error { return runSearch(searchCmd, []string{"battery"}) }},
		{ptabSearchCmd, func() error { return runPtabSearch(ptabSearchCmd, nil) }},
		{ptabDecisionsCmd, func() error { return runPtabDecisions(ptabDecisionsCmd, nil) }},
		{ptabDocsCmd, func() error { return runPtabDocs(ptabDocsCmd, nil) }},
		{ptabAppealsCmd, func() error { return runPtabAppeals(ptabAppealsCmd, nil) }},
		{ptabInterferencesCmd, func() error { return runPtabInterferences(ptabInterferencesCmd, nil) }},
	}
	for _, c := range commands {
		t.Run(c.cmd.CommandPath(), func(t *testing.T) {
			ts, requests := exportServer(t, 137)
			exportHarness(t, ts, true)
			setFlagForTest(t, c.cmd, "download", "xlsx")

			err := c.run()
			if err == nil {
				t.Fatal("want a usage error for --download xlsx")
			}
			if code, _ := usageEnvelope(t, err); code != types.ExitInvalidArgs {
				t.Errorf("exit code = %d, want %d", code, types.ExitInvalidArgs)
			}
			if got := requests(); len(got) != 0 {
				t.Errorf("server saw %+v, want no requests", got)
			}
		})
	}
}

func TestPtabDownload_CountsFirstUnlessLimitIsExplicit(t *testing.T) {
	commands := []struct {
		cmd  *cobra.Command
		run  func() error
		path string
	}{
		{ptabSearchCmd, func() error { return runPtabSearch(ptabSearchCmd, nil) }, "/api/v1/patent/trials/proceedings/search"},
		{ptabDecisionsCmd, func() error { return runPtabDecisions(ptabDecisionsCmd, nil) }, "/api/v1/patent/trials/decisions/search"},
		{ptabDocsCmd, func() error { return runPtabDocs(ptabDocsCmd, nil) }, "/api/v1/patent/trials/documents/search"},
		{ptabAppealsCmd, func() error { return runPtabAppeals(ptabAppealsCmd, nil) }, "/api/v1/patent/appeals/decisions/search"},
		{ptabInterferencesCmd, func() error { return runPtabInterferences(ptabInterferencesCmd, nil) }, "/api/v1/patent/interferences/decisions/search"},
	}
	for _, c := range commands {
		t.Run(c.cmd.CommandPath()+" full export", func(t *testing.T) {
			ts, requests := exportServer(t, 312)
			exportHarness(t, ts, true)
			setFlagForTest(t, c.cmd, "download", "csv")

			stdout, _, err := captureTrademarkOutput(t, c.run)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			got := requests()
			if len(got) != 2 {
				t.Fatalf("server saw %d requests, want a count and a download: %+v", len(got), got)
			}
			if got[0].Path != c.path || got[0].Limit != "1" {
				t.Errorf("first request = %+v, want a limit=1 count on %s", got[0], c.path)
			}
			if got[1].Path != c.path+"/download" || got[1].Limit != "312" || got[1].Format != "csv" {
				t.Errorf("second request = %+v, want %s/download with limit=312 format=csv", got[1], c.path)
			}
			if !strings.HasPrefix(stdout, "applicationNumberText") {
				t.Errorf("stdout is not the raw export:\n%s", stdout)
			}
		})
		t.Run(c.cmd.CommandPath()+" explicit limit", func(t *testing.T) {
			ts, requests := exportServer(t, 312)
			exportHarness(t, ts, true)
			setFlagForTest(t, c.cmd, "download", "json")
			setFlagForTest(t, c.cmd, "limit", "40")

			if _, _, err := captureTrademarkOutput(t, c.run); err != nil {
				t.Fatalf("run error: %v", err)
			}
			got := requests()
			if len(got) != 1 || got[0].Path != c.path+"/download" || got[0].Limit != "40" {
				t.Fatalf("requests = %+v, want only %s/download with limit=40", got, c.path)
			}
		})
	}
}

// --dry-run still sends nothing. It prints the count request, then the
// download request with a placeholder for the limit the count would set.
func TestDryRun_DownloadPrintsCountThenExportWithoutSendingEither(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T)
		run        func() error
		wantStderr []string
		notStderr  []string
	}{
		{
			name:  "search GET",
			setup: func(t *testing.T) { setFlagForTest(t, searchCmd, "download", "csv") },
			run:   func() error { return runSearch(searchCmd, []string{"battery"}) },
			wantStderr: []string{
				"GET " + patentSearchPath + "\n", "limit=1&q=battery",
				"Then, with limit set to the count above",
				"GET " + patentDownloadPath, "format=csv&limit=" + exportLimitPlaceholder,
			},
			notStderr: []string{"limit=25"},
		},
		{
			name: "search POST",
			setup: func(t *testing.T) {
				setFlagForTest(t, searchCmd, "download", "json")
				searchFlags.granted = true
			},
			run: func() error { return runSearch(searchCmd, []string{"battery"}) },
			wantStderr: []string{
				"POST " + patentSearchPath + "\n", `"limit": 1`,
				"POST " + patentDownloadPath, `"limit": "` + exportLimitPlaceholder + `"`, `"format": "json"`,
			},
			notStderr: []string{`"limit": 25`},
		},
		{
			name: "search explicit limit",
			setup: func(t *testing.T) {
				setFlagForTest(t, searchCmd, "download", "csv")
				setFlagForTest(t, searchCmd, "limit", "40")
			},
			run:        func() error { return runSearch(searchCmd, []string{"battery"}) },
			wantStderr: []string{"GET " + patentDownloadPath, "format=csv&limit=40&q=battery"},
			notStderr:  []string{"Then, with limit", "GET " + patentSearchPath + "\n"},
		},
		{
			name:  "ptab search",
			setup: func(t *testing.T) { setFlagForTest(t, ptabSearchCmd, "download", "csv") },
			run:   func() error { return runPtabSearch(ptabSearchCmd, []string{"sensor"}) },
			wantStderr: []string{
				"GET /api/v1/patent/trials/proceedings/search\n", "limit=1&q=sensor",
				"GET /api/v1/patent/trials/proceedings/search/download", "format=csv&limit=" + exportLimitPlaceholder,
			},
			notStderr: []string{"limit=25"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := dryRunHarness(t, "json")
			tt.setup(t)

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
			for _, not := range tt.notStderr {
				if strings.Contains(stderr, not) {
					t.Errorf("stderr should not contain %q:\n%s", not, stderr)
				}
			}
		})
	}
}
