package cmd

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
	"github.com/smcronin/uspto-cli/internal/types"
)

// notedEnvelope is the part of the JSON envelope the compound-command tests
// look at.
type notedEnvelope struct {
	OK       bool            `json:"ok"`
	Partial  bool            `json:"partial"`
	Warnings []string        `json:"warnings"`
	Results  json.RawMessage `json:"results"`
}

// compoundHarness points the ODP client at ts and sets the output flags,
// restoring every global the compound commands read when the test ends.
func compoundHarness(t *testing.T, ts *httptest.Server, format string, quiet bool) {
	t.Helper()
	origClient := api.DefaultClient
	origQuiet, origFormat, origMinify, origDryRun := flagQuiet, flagFormat, flagMinify, flagDryRun
	origDepth, origDates := flagFamilyDepth, flagFamilyWithDates
	origAllOut, origAllCodes, origAllFrom, origAllTo, origAllAs := appDownloadAllOutputFlag, appDownloadAllCodesFlag, appDownloadAllFromFlag, appDownloadAllToFlag, appDownloadAllAsFlag
	origTextAllCodes, origTextAllFrom, origTextAllTo, origTextAllAs := appTextAllCodesFlag, appTextAllFromFlag, appTextAllToFlag, appTextAllAsFlag
	origBundleOut, origBundleType := patentBundleOutputDir, patentBundleIDType
	t.Cleanup(func() {
		api.DefaultClient = origClient
		flagQuiet, flagFormat, flagMinify, flagDryRun = origQuiet, origFormat, origMinify, origDryRun
		flagFamilyDepth, flagFamilyWithDates = origDepth, origDates
		appDownloadAllOutputFlag, appDownloadAllCodesFlag, appDownloadAllFromFlag, appDownloadAllToFlag, appDownloadAllAsFlag = origAllOut, origAllCodes, origAllFrom, origAllTo, origAllAs
		appTextAllCodesFlag, appTextAllFromFlag, appTextAllToFlag, appTextAllAsFlag = origTextAllCodes, origTextAllFrom, origTextAllTo, origTextAllAs
		patentBundleOutputDir, patentBundleIDType = origBundleOut, origBundleType
	})

	api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))
	flagQuiet, flagFormat, flagMinify, flagDryRun = quiet, format, true, false
	flagFamilyDepth, flagFamilyWithDates = 2, false
	appDownloadAllOutputFlag, appDownloadAllCodesFlag, appDownloadAllFromFlag, appDownloadAllToFlag, appDownloadAllAsFlag = "", "", "", "", "pdf"
	appTextAllCodesFlag, appTextAllFromFlag, appTextAllToFlag, appTextAllAsFlag = "", "", "", "auto"
	patentBundleOutputDir, patentBundleIDType = "", idTypeAuto
}

func decodeNotedEnvelope(t *testing.T, stdout string) notedEnvelope {
	t.Helper()
	var env notedEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not a JSON envelope: %v\n%s", err, stdout)
	}
	return env
}

// wantExit asserts that err is a typed API error handleError maps to code.
func wantExit(t *testing.T, err error, code int) {
	t.Helper()
	if err == nil {
		t.Fatalf("command returned nil, want an error with exit code %d", code)
	}
	var apiErr *api.UsptoAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %q does not wrap *api.UsptoAPIError", err)
	}
	got, errType := usageEnvelope(t, err)
	if got != code {
		t.Errorf("exit code = %d (%s), want %d for %q", got, errType, code, err)
	}
}

func wantWarning(t *testing.T, warnings []string, substr string) {
	t.Helper()
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return
		}
	}
	t.Errorf("no warning mentions %q: %q", substr, warnings)
}

// statusServer answers every request with the status chosen by statusFor;
// 200 responses carry body.
func statusServer(statusFor func(path string) int, bodyFor func(path string) string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status := statusFor(r.URL.Path); status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(bodyFor(r.URL.Path)))
	}))
}

const emptyPFW = `{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456"}]}`

// ---------------------------------------------------------------------------
// summary
// ---------------------------------------------------------------------------

func TestRunSummary_TotalFailureReturnsTypedError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   int
	}{
		{"unknown application", http.StatusNotFound, types.ExitNotFound},
		{"bad key", http.StatusForbidden, types.ExitAuthFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := statusServer(func(string) int { return tt.status }, nil)
			defer ts.Close()
			compoundHarness(t, ts, "json", true)

			var runErr error
			stdout, _, _ := captureTrademarkOutput(t, func() error {
				runErr = runSummary(summaryCmd, []string{"16123456"})
				return nil
			})
			if stdout != "" {
				t.Errorf("a failed summary wrote a result to stdout:\n%s", stdout)
			}
			wantExit(t, runErr, tt.code)
		})
	}
}

// Without metadata there is nothing to identify the application by, so a
// failed metadata request fails the summary even when every other endpoint
// would answer. The other endpoints are not asked.
func TestRunSummary_MetadataFailureIsTotalFailure(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		code   int
	}{
		{"metadata refused", http.StatusForbidden, "", types.ExitAuthFailure},
		{"metadata server error", http.StatusBadRequest, "", types.ExitGeneralError},
		{"metadata with no record", http.StatusOK, `{"count":0,"patentFileWrapperDataBag":[]}`, types.ExitNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var others int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/meta-data") {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
					return
				}
				atomic.AddInt32(&others, 1)
				_, _ = w.Write([]byte(standAloneBody(r.URL.Path)))
			}))
			defer ts.Close()
			compoundHarness(t, ts, "json", true)

			var runErr error
			stdout, _, _ := captureTrademarkOutput(t, func() error {
				runErr = runSummary(summaryCmd, []string{"16123456"})
				return nil
			})
			if stdout != "" {
				t.Errorf("a summary without metadata wrote a result to stdout:\n%s", stdout)
			}
			wantExit(t, runErr, tt.code)
			if n := atomic.LoadInt32(&others); n != 0 {
				t.Errorf("summary sent %d further request(s) after metadata failed, want 0", n)
			}
		})
	}
}

func TestRunSummary_PartialFailureIsMarkedEvenWhenQuiet(t *testing.T) {
	failContinuity := func(path string) int {
		if strings.HasSuffix(path, "/continuity") {
			return http.StatusForbidden
		}
		return http.StatusOK
	}
	body := func(path string) string {
		if strings.HasSuffix(path, "/documents") {
			return `{"documentBag":[]}`
		}
		return emptyPFW
	}

	t.Run("json envelope carries warnings and partial", func(t *testing.T) {
		ts := statusServer(failContinuity, body)
		defer ts.Close()
		compoundHarness(t, ts, "json", true)

		stdout, stderr, err := captureTrademarkOutput(t, func() error {
			return runSummary(summaryCmd, []string{"16123456"})
		})
		if err != nil {
			t.Fatalf("runSummary() error: %v", err)
		}
		env := decodeNotedEnvelope(t, stdout)
		if !env.OK || !env.Partial {
			t.Errorf("ok=%v partial=%v, want true/true:\n%s", env.OK, env.Partial, stdout)
		}
		if len(env.Warnings) != 1 {
			t.Errorf("warnings = %q, want exactly the continuity failure", env.Warnings)
		}
		wantWarning(t, env.Warnings, "continuity")
		if stderr != "" {
			t.Errorf("-q -f json should leave stderr empty, got %q", stderr)
		}
	})

	t.Run("table output warns on stderr under quiet", func(t *testing.T) {
		ts := statusServer(failContinuity, body)
		defer ts.Close()
		compoundHarness(t, ts, "table", true)

		_, stderr, err := captureTrademarkOutput(t, func() error {
			return runSummary(summaryCmd, []string{"16123456"})
		})
		if err != nil {
			t.Fatalf("runSummary() error: %v", err)
		}
		if !strings.Contains(stderr, "Warning: failed to fetch continuity") {
			t.Errorf("stderr does not carry the warning under -q:\n%s", stderr)
		}
	})

	t.Run("complete summary has neither field", func(t *testing.T) {
		ts := statusServer(func(string) int { return http.StatusOK }, body)
		defer ts.Close()
		compoundHarness(t, ts, "json", true)

		stdout, _, err := captureTrademarkOutput(t, func() error {
			return runSummary(summaryCmd, []string{"16123456"})
		})
		if err != nil {
			t.Fatalf("runSummary() error: %v", err)
		}
		if strings.Contains(stdout, `"partial"`) || strings.Contains(stdout, `"warnings"`) {
			t.Errorf("a complete summary should omit partial and warnings:\n%s", stdout)
		}
	})
}

// standAloneBody serves a granted application with no continuity, no
// recorded assignment and no foreign priority, the way ODP answers for one:
// checked live on 2026-11-13 against application 29932490, whose
// /continuity, /assignment and /foreign-priority each returned 200 with one
// file-wrapper record and no bag, not 404.
func standAloneBody(path string) string {
	switch {
	case strings.HasSuffix(path, "/meta-data"):
		return `{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","applicationMetaData":{"inventionTitle":"Widget","firstApplicantName":"Solo Inventor"}}]}`
	case strings.HasSuffix(path, "/documents"):
		return `{"documentBag":[]}`
	}
	return emptyPFW
}

// A stand-alone application is a complete summary, not a partial one, and
// with no title conveyance on record the applicant stands in as assignee.
func TestRunSummary_StandAloneApplicationIsNotPartial(t *testing.T) {
	ts := statusServer(func(path string) int {
		// No PTA data: the one optional sub-request that answers 404.
		if strings.HasSuffix(path, "/adjustment") {
			return http.StatusNotFound
		}
		return http.StatusOK
	}, standAloneBody)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runSummary(summaryCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runSummary() error: %v", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || env.Partial || len(env.Warnings) != 0 {
		t.Errorf("ok=%v partial=%v warnings=%q, want ok with neither", env.OK, env.Partial, env.Warnings)
	}
	var summary AppSummary
	if err := json.Unmarshal(env.Results, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.CurrentAssignee != "Solo Inventor" || summary.CurrentAssigneeSource != assigneeFromApplicant {
		t.Errorf("currentAssignee = %q (source %q), want the applicant", summary.CurrentAssignee, summary.CurrentAssigneeSource)
	}
	if len(summary.Parents) != 0 || len(summary.Children) != 0 || summary.ForeignPriorityCount != 0 {
		t.Errorf("parents=%v children=%v foreignPriority=%d, want none", summary.Parents, summary.Children, summary.ForeignPriorityCount)
	}
}

// A 404 from /assignment is not how ODP reports "no assignments" (see
// standAloneBody), so it stays a failure: the summary is partial and does not
// name the applicant as owner on records it could not read.
func TestRunSummary_AssignmentNotFoundIsPartialWithoutApplicantFallback(t *testing.T) {
	ts := statusServer(func(path string) int {
		if strings.HasSuffix(path, "/assignment") {
			return http.StatusNotFound
		}
		return http.StatusOK
	}, standAloneBody)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runSummary(summaryCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runSummary() error: %v", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || !env.Partial {
		t.Errorf("ok=%v partial=%v, want true/true", env.OK, env.Partial)
	}
	wantWarning(t, env.Warnings, "assignment")
	var summary AppSummary
	if err := json.Unmarshal(env.Results, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.CurrentAssignee != "" || summary.CurrentAssigneeSource != "" {
		t.Errorf("currentAssignee = %q (source %q), want none when the records could not be read",
			summary.CurrentAssignee, summary.CurrentAssigneeSource)
	}
}

// ---------------------------------------------------------------------------
// family
// ---------------------------------------------------------------------------

// A family walk from an application without relatives is one complete node.
func TestRunFamily_StandAloneApplicationIsNotPartial(t *testing.T) {
	ts := statusServer(func(string) int { return http.StatusOK }, standAloneBody)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runFamily(familyCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runFamily() error: %v", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || env.Partial || len(env.Warnings) != 0 {
		t.Errorf("ok=%v partial=%v warnings=%q, want ok with neither", env.OK, env.Partial, env.Warnings)
	}
	var result FamilyResult
	if err := json.Unmarshal(env.Results, &result); err != nil {
		t.Fatal(err)
	}
	if result.TotalMembers != 1 || result.Tree.Title != "Widget" {
		t.Errorf("totalMembers=%d title=%q, want the root alone with its metadata", result.TotalMembers, result.Tree.Title)
	}
}

// familyBody serves a root application (16123456) with one child (17000001).
func familyBody(path string) string {
	switch {
	case strings.HasSuffix(path, "/16123456/continuity"):
		return `{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","childContinuityBag":[{"childApplicationNumberText":"17000001","claimParentageTypeCode":"CON"}]}]}`
	case strings.HasSuffix(path, "/continuity"):
		return `{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"17000001"}]}`
	default:
		return `{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"x","applicationMetaData":{"inventionTitle":"Widget"}}]}`
	}
}

func TestRunFamily_RootFailureReturnsTypedError(t *testing.T) {
	ts := statusServer(func(string) int { return http.StatusNotFound }, nil)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	var runErr error
	stdout, _, _ := captureTrademarkOutput(t, func() error {
		runErr = runFamily(familyCmd, []string{"16123456"})
		return nil
	})
	if stdout != "" {
		t.Errorf("a failed family walk wrote a result to stdout:\n%s", stdout)
	}
	wantExit(t, runErr, types.ExitNotFound)
}

func TestRunFamily_MemberFailureIsPartial(t *testing.T) {
	ts := statusServer(func(path string) int {
		if strings.HasSuffix(path, "/17000001/meta-data") {
			return http.StatusForbidden
		}
		return http.StatusOK
	}, familyBody)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return runFamily(familyCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runFamily() error: %v", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || !env.Partial {
		t.Errorf("ok=%v partial=%v, want true/true:\n%s", env.OK, env.Partial, stdout)
	}
	wantWarning(t, env.Warnings, "metadata for 17000001")
	var result FamilyResult
	if err := json.Unmarshal(env.Results, &result); err != nil {
		t.Fatal(err)
	}
	if result.TotalMembers != 2 {
		t.Errorf("totalMembers = %d, want 2 (the member is kept, without metadata)", result.TotalMembers)
	}
	if stderr != "" {
		t.Errorf("-q -f json should leave stderr empty, got %q", stderr)
	}
}

func TestRunFamily_RootContinuityFailureIsPartial(t *testing.T) {
	ts := statusServer(func(path string) int {
		if strings.HasSuffix(path, "/continuity") {
			return http.StatusForbidden
		}
		return http.StatusOK
	}, familyBody)
	defer ts.Close()
	compoundHarness(t, ts, "ndjson", true)

	_, stderr, err := captureTrademarkOutput(t, func() error {
		return runFamily(familyCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("runFamily() error: %v (metadata worked, so the walk is partial, not failed)", err)
	}
	if !strings.Contains(stderr, "Warning: failed to fetch continuity for 16123456") {
		t.Errorf("ndjson output has no envelope, so the warning must reach stderr under -q:\n%s", stderr)
	}
}

// ---------------------------------------------------------------------------
// app download-all
// ---------------------------------------------------------------------------

// documentsServer lists two documents whose PDF and XML renditions live under
// /files/. failing names the file paths that answer 403.
func documentsServer(t *testing.T, failing ...string) *httptest.Server {
	t.Helper()
	fail := map[string]bool{}
	for _, f := range failing {
		fail[f] = true
	}
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/documents"):
			w.Header().Set("Content-Type", "application/json")
			doc := func(id, code, date string) string {
				return fmt.Sprintf(`{"applicationNumberText":"16123456","officialDate":%q,"documentIdentifier":%q,"documentCode":%q,"documentCodeDescriptionText":"doc %s","downloadOptionBag":[{"mimeTypeIdentifier":"PDF","downloadUrl":"%s/files/%s.pdf"},{"mimeTypeIdentifier":"XML","downloadUrl":"%s/files/%s.tar"}]}`,
					date, id, code, id, ts.URL, id, ts.URL, id)
			}
			_, _ = w.Write([]byte(`{"count":2,"documentBag":[` + doc("A", "CTNF", "2023-01-05") + `,` + doc("B", "NOA", "2023-06-07") + `]}`))
		case fail[r.URL.Path]:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		case strings.HasSuffix(r.URL.Path, ".tar"):
			_, _ = w.Write(xmlArchive(t, "Text of "+r.URL.Path))
		default:
			_, _ = w.Write([]byte("pdf for " + r.URL.Path))
		}
	}))
	return ts
}

// xmlArchive builds the tar archive the documents API serves for XML.
func xmlArchive(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	body := `<?xml version="1.0" encoding="UTF-8"?><root><P>` + text + `</P></root>`
	if err := tw.WriteHeader(&tar.Header{Name: "doc.xml", Mode: 0600, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestAppDownloadAll_PartialFailureIsMarked(t *testing.T) {
	for _, format := range []string{"json", "table"} {
		t.Run(format, func(t *testing.T) {
			ts := documentsServer(t, "/files/B.pdf")
			defer ts.Close()
			compoundHarness(t, ts, format, true)
			appDownloadAllOutputFlag = t.TempDir()

			stdout, stderr, err := captureTrademarkOutput(t, func() error {
				return appDownloadAllCmd.RunE(appDownloadAllCmd, []string{"16123456"})
			})
			if err != nil {
				t.Fatalf("download-all error: %v (one of two files arrived, so exit 0)", err)
			}
			entries, _ := os.ReadDir(appDownloadAllOutputFlag)
			if len(entries) != 1 {
				t.Errorf("output directory holds %d files, want 1", len(entries))
			}
			if format == "table" {
				if !strings.Contains(stderr, "Warning: document 2 (NOA, 2023-06-07) was not downloaded") {
					t.Errorf("table output must warn on stderr under -q:\n%s", stderr)
				}
				return
			}
			env := decodeNotedEnvelope(t, stdout)
			if !env.OK || !env.Partial {
				t.Errorf("ok=%v partial=%v, want true/true:\n%s", env.OK, env.Partial, stdout)
			}
			wantWarning(t, env.Warnings, "document 2 (NOA, 2023-06-07)")
			if stderr != "" {
				t.Errorf("-q -f json should leave stderr empty, got %q", stderr)
			}
		})
	}
}

// Without -q each failed document gets one stderr line. It used to get two:
// an "Error:" line during the run and a "Warning:" line at the end.
func TestAppDownloadAll_OneStderrLinePerFailureWithoutQuiet(t *testing.T) {
	for _, format := range []string{"json", "table"} {
		t.Run(format, func(t *testing.T) {
			ts := documentsServer(t, "/files/B.pdf")
			defer ts.Close()
			compoundHarness(t, ts, format, false)
			appDownloadAllOutputFlag = t.TempDir()

			stdout, stderr, err := captureTrademarkOutput(t, func() error {
				return appDownloadAllCmd.RunE(appDownloadAllCmd, []string{"16123456"})
			})
			if err != nil {
				t.Fatalf("download-all error: %v", err)
			}
			if n := strings.Count(stderr, "document 2 (NOA, 2023-06-07)"); n != 1 {
				t.Errorf("the failure is reported %d times on stderr, want once:\n%s", n, stderr)
			}
			if strings.Contains(stderr, "Error:") {
				t.Errorf("stderr still has a separate Error: line:\n%s", stderr)
			}
			if format == "json" {
				wantWarning(t, decodeNotedEnvelope(t, stdout).Warnings, "document 2 (NOA, 2023-06-07)")
			}
		})
	}
}

func TestAppDownloadAll_TotalFailureReturnsTypedError(t *testing.T) {
	ts := documentsServer(t, "/files/A.pdf", "/files/B.pdf")
	defer ts.Close()
	compoundHarness(t, ts, "json", true)
	appDownloadAllOutputFlag = t.TempDir()

	var runErr error
	stdout, _, _ := captureTrademarkOutput(t, func() error {
		runErr = appDownloadAllCmd.RunE(appDownloadAllCmd, []string{"16123456"})
		return nil
	})
	if stdout != "" {
		t.Errorf("a failed download-all wrote a result to stdout:\n%s", stdout)
	}
	wantExit(t, runErr, types.ExitAuthFailure)
}

func TestAppDownloadAll_NothingToDownloadIsNotAFailure(t *testing.T) {
	ts := documentsServer(t)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)
	appDownloadAllOutputFlag = t.TempDir()
	appDownloadAllAsFlag = "docx" // neither document has a DOCX rendition

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return appDownloadAllCmd.RunE(appDownloadAllCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("download-all error: %v (skipped documents are not failures)", err)
	}
	if env := decodeNotedEnvelope(t, stdout); !env.OK || env.Partial || len(env.Warnings) != 0 {
		t.Errorf("envelope = %+v, want ok without partial or warnings", env)
	}
}

// ---------------------------------------------------------------------------
// app text-all
// ---------------------------------------------------------------------------

func TestAppTextAll_PartialFailureIsMarked(t *testing.T) {
	ts := documentsServer(t, "/files/B.tar")
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return appTextAllCmd.RunE(appTextAllCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("text-all error: %v (one of two documents was read, so exit 0)", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || !env.Partial {
		t.Errorf("ok=%v partial=%v, want true/true:\n%s", env.OK, env.Partial, stdout)
	}
	wantWarning(t, env.Warnings, "document 2 (NOA, 2023-06-07) could not be read")
	var result AppDocumentTextBatchResult
	if err := json.Unmarshal(env.Results, &result); err != nil {
		t.Fatal(err)
	}
	if result.ExtractedCount != 1 || result.SkippedCount != 1 {
		t.Errorf("extracted=%d skipped=%d, want 1/1", result.ExtractedCount, result.SkippedCount)
	}
	if stderr != "" {
		t.Errorf("-q -f json should leave stderr empty, got %q", stderr)
	}
}

func TestAppTextAll_TotalFailureReturnsTypedError(t *testing.T) {
	ts := documentsServer(t, "/files/A.tar", "/files/B.tar")
	defer ts.Close()
	compoundHarness(t, ts, "json", true)

	var runErr error
	stdout, _, _ := captureTrademarkOutput(t, func() error {
		runErr = appTextAllCmd.RunE(appTextAllCmd, []string{"16123456"})
		return nil
	})
	if stdout != "" {
		t.Errorf("a failed text-all wrote a result to stdout:\n%s", stdout)
	}
	wantExit(t, runErr, types.ExitAuthFailure)
}

func TestAppTextAll_UnreadableFormatsAreSkippedNotFailed(t *testing.T) {
	ts := documentsServer(t)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)
	appTextAllAsFlag = "docx" // neither document has a DOCX rendition

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return appTextAllCmd.RunE(appTextAllCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("text-all error: %v (documents without the format are skipped, not failed)", err)
	}
	if env := decodeNotedEnvelope(t, stdout); !env.OK || env.Partial || len(env.Warnings) != 0 {
		t.Errorf("envelope = %+v, want ok without partial or warnings", env)
	}
}

// ---------------------------------------------------------------------------
// patent bundle
// ---------------------------------------------------------------------------

// bundleServer serves what `patent bundle --id-type app` reads: metadata,
// associated documents (a grant XML at /files/grant.xml) and the two documents
// of documentsServer. failing names the file paths that answer 403.
func bundleServer(t *testing.T, failing ...string) *httptest.Server {
	t.Helper()
	docs := documentsServer(t, failing...)
	t.Cleanup(docs.Close)
	fail := map[string]bool{}
	for _, f := range failing {
		fail[f] = true
	}
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/meta-data"):
			_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","applicationMetaData":{"inventionTitle":"Widget","patentNumber":"10000001"}}]}`))
		case strings.HasSuffix(r.URL.Path, "/associated-documents"):
			_, _ = w.Write([]byte(`{"count":1,"patentFileWrapperDataBag":[{"applicationNumberText":"16123456","grantDocumentMetaData":{"fileLocationURI":"` + ts.URL + `/files/grant.xml"}}]}`))
		case r.URL.Path == "/files/grant.xml":
			if fail[r.URL.Path] {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`<us-patent-grant><us-bibliographic-data-grant><invention-title>Widget</invention-title></us-bibliographic-data-grant></us-patent-grant>`))
		default:
			// Document listing and PDFs come from the documents server.
			resp, err := http.Get(docs.URL + r.URL.Path)
			if err != nil {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			w.WriteHeader(resp.StatusCode)
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(resp.Body)
			_, _ = w.Write(buf.Bytes())
		}
	}))
	return ts
}

func TestPatentBundle_PartialFailureIsMarked(t *testing.T) {
	ts := bundleServer(t, "/files/grant.xml")
	defer ts.Close()
	compoundHarness(t, ts, "json", true)
	patentBundleOutputDir, patentBundleIDType = t.TempDir(), idTypeApp

	stdout, stderr, err := captureTrademarkOutput(t, func() error {
		return runPatentBundle(patentBundleCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("patent bundle error: %v (the PDFs arrived, so exit 0)", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || !env.Partial {
		t.Errorf("ok=%v partial=%v, want true/true:\n%s", env.OK, env.Partial, stdout)
	}
	wantWarning(t, env.Warnings, "grant XML download failed")
	for _, w := range env.Warnings {
		if strings.Contains(w, "unavailable") {
			t.Errorf("envelope warnings should list failures only, got %q", w)
		}
	}
	var summary patentBundleSummary
	if err := json.Unmarshal(env.Results, &summary); err != nil {
		t.Fatal(err)
	}
	if summary.PDFDownloaded != 2 || summary.PDFFailed != 0 {
		t.Errorf("pdfDownloaded=%d pdfFailed=%d, want 2/0", summary.PDFDownloaded, summary.PDFFailed)
	}
	if stderr != "" {
		t.Errorf("-q -f json should leave stderr empty, got %q", stderr)
	}
}

func TestPatentBundle_TotalDownloadFailureReturnsTypedError(t *testing.T) {
	ts := bundleServer(t, "/files/grant.xml", "/files/A.pdf", "/files/B.pdf")
	defer ts.Close()
	compoundHarness(t, ts, "json", true)
	patentBundleOutputDir, patentBundleIDType = t.TempDir(), idTypeApp

	var runErr error
	stdout, _, _ := captureTrademarkOutput(t, func() error {
		runErr = runPatentBundle(patentBundleCmd, []string{"16123456"})
		return nil
	})
	if stdout != "" {
		t.Errorf("a failed bundle wrote a result to stdout:\n%s", stdout)
	}
	wantExit(t, runErr, types.ExitAuthFailure)

	// The indexes and a README naming the failures are still written.
	readme, err := os.ReadFile(filepath.Join(patentBundleOutputDir, "README.md"))
	if err != nil {
		t.Fatalf("README was not written: %v", err)
	}
	if !strings.Contains(string(readme), "2 of 2 file-wrapper PDF downloads failed") {
		t.Errorf("README does not report the PDF failures:\n%s", readme)
	}
}

func TestPatentBundle_CompleteBundleHasNoPartialMarker(t *testing.T) {
	ts := bundleServer(t)
	defer ts.Close()
	compoundHarness(t, ts, "json", true)
	patentBundleOutputDir, patentBundleIDType = t.TempDir(), idTypeApp

	stdout, _, err := captureTrademarkOutput(t, func() error {
		return runPatentBundle(patentBundleCmd, []string{"16123456"})
	})
	if err != nil {
		t.Fatalf("patent bundle error: %v", err)
	}
	env := decodeNotedEnvelope(t, stdout)
	if !env.OK || env.Partial || len(env.Warnings) != 0 {
		t.Errorf("envelope = ok %v partial %v warnings %q, want ok with neither", env.OK, env.Partial, env.Warnings)
	}
	// "pgpub XML unavailable" is a note, not a failure: it stays in the result.
	var summary patentBundleSummary
	if err := json.Unmarshal(env.Results, &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Warnings) != 1 || !strings.Contains(summary.Warnings[0], "pgpub XML unavailable") {
		t.Errorf("results.warnings = %q, want the pgpub note", summary.Warnings)
	}
}
