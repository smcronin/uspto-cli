package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/smcronin/uspto-cli/internal/api"
)

// tmDecisionTestEnv points the ODP client at a test server, resets the
// search flags and restores everything afterwards.
func tmDecisionTestEnv(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	origFlags := tmDecisionSearchFlags
	origClient := api.DefaultClient
	origQuiet, origFormat, origMinify, origDryRun := flagQuiet, flagFormat, flagMinify, flagDryRun
	origOutput := tmDecisionDownloadOutput
	searchFlags := trademarkDecisionsSearchCmd.Flags()
	t.Cleanup(func() {
		tmDecisionSearchFlags = origFlags
		api.DefaultClient = origClient
		flagQuiet, flagFormat, flagMinify, flagDryRun = origQuiet, origFormat, origMinify, origDryRun
		tmDecisionDownloadOutput = origOutput
		for _, name := range []string{"limit", "download"} {
			flag := searchFlags.Lookup(name)
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		}
	})
	tmDecisionSearchFlags = struct {
		category       string
		proceedingType string
		serial         string
		registration   string
		mark           string
		party          string
		petitioner     string
		registrant     string
		country        string
		docType        string
		petitionNumber string
		basis          string
		text           string
		precedential   bool
		directorInit   bool
		from           string
		to             string
		sort           string
		fields         string
		facets         string
		limit          int
		offset         int
		all            bool
		noText         bool
	}{limit: 25}
	flagQuiet, flagFormat, flagMinify, flagDryRun = true, "json", true, false
	if handler != nil {
		ts := httptest.NewServer(handler)
		t.Cleanup(ts.Close)
		api.DefaultClient = api.NewClient("test-key", api.WithBaseURL(ts.URL))
	}
}

func captureStdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	runErr := run()
	_ = w.Close()
	os.Stdout = old
	out := <-done
	_ = r.Close()
	return out, runErr
}

func TestTrademarkDecisionsProviderIsODP(t *testing.T) {
	for _, path := range [][]string{
		{"trademark", "decisions", "search"},
		{"tm", "proceedings", "get"},
		{"trademark", "decision", "download"},
	} {
		command, remaining, err := rootCmd.Find(path)
		if err != nil || len(remaining) != 0 {
			t.Fatalf("Find(%v): %v, remaining %v", path, err, remaining)
		}
		if got := providerForCommand(command); got != providerODP {
			t.Errorf("providerForCommand(%q) = %q, want %q", command.CommandPath(), got, providerODP)
		}
	}
	group, _, err := rootCmd.Find([]string{"trademark", "decisions"})
	if err != nil {
		t.Fatal(err)
	}
	if got := providerForCommand(group); got != providerNone {
		t.Errorf("group provider = %q, want %q", got, providerNone)
	}
}

func TestBuildTrademarkDecisionQuery(t *testing.T) {
	tmDecisionTestEnv(t, nil)
	fl := &tmDecisionSearchFlags

	if q, err := buildTrademarkDecisionQuery(nil); err != nil || q != "" {
		t.Fatalf("empty query = %q, %v", q, err)
	}

	fl.category = "expungement-reexam"
	fl.proceedingType = "Reexam"
	fl.serial = "sn:87712766"
	fl.registration = "RN5513424"
	fl.mark = "CRUSADE AGAINST ANGER"
	fl.party = `Acme "Best" Co`
	fl.text = "specimen farm"
	fl.precedential = true
	fl.directorInit = true
	fl.from = "2026-01-01"
	q, err := buildTrademarkDecisionQuery([]string{"fraud OR sanction"})
	if err != nil {
		t.Fatal(err)
	}
	want := `(fraud OR sanction) AND decisionProceedingCategory:ExpungReexam AND proceedingTypeCategory:REEXAMINATION` +
		` AND serialNumber:87712766 AND registrationNumber:5513424 AND markLiteralText:"CRUSADE AGAINST ANGER"` +
		` AND partyName:"Acme \"Best\" Co" AND ocrSearchText:"specimen farm" AND precedentialIndicator:true` +
		` AND initiatedByDirectorIndicator:true AND decisionActionDate:[2026-01-01 TO 2100-12-31]`
	if q != want {
		t.Fatalf("query =\n%s\nwant\n%s", q, want)
	}

	// A lone positional query is passed through untouched; a wildcard value
	// is not quoted.
	tmDecisionTestEnv(t, nil)
	if q, _ := buildTrademarkDecisionQuery([]string{"markLiteralText:GALA*"}); q != "markLiteralText:GALA*" {
		t.Fatalf("lone query = %q", q)
	}
	fl.mark = "GALA*"
	fl.to = "2023-01-01"
	if q, _ := buildTrademarkDecisionQuery(nil); q != "markLiteralText:GALA* AND decisionActionDate:[1900-01-01 TO 2023-01-01]" {
		t.Fatalf("wildcard query = %q", q)
	}
}

func TestBuildTrademarkDecisionQueryRejectsBadInput(t *testing.T) {
	for name, set := range map[string]func(){
		"category":        func() { tmDecisionSearchFlags.category = "bogus" },
		"proceeding type": func() { tmDecisionSearchFlags.proceedingType = "appeal" },
		"serial":          func() { tmDecisionSearchFlags.serial = "8771x766" },
		"registration":    func() { tmDecisionSearchFlags.registration = "123456789" },
		"date":            func() { tmDecisionSearchFlags.from = "11/01/2026" },
		"date order": func() {
			tmDecisionSearchFlags.from = "2026-12-01"
			tmDecisionSearchFlags.to = "2026-01-01"
		},
	} {
		t.Run(name, func(t *testing.T) {
			tmDecisionTestEnv(t, nil)
			set()
			_, err := buildTrademarkDecisionQuery(nil)
			if _, ok := err.(*invalidArgsError); !ok {
				t.Fatalf("error = %v (%T), want a usage error", err, err)
			}
		})
	}
}

func TestTrademarkDecisionCategoryAliases(t *testing.T) {
	for raw, want := range map[string]string{
		"expungement-reexam":  "ExpungReexam",
		"ExpungReexam":        "ExpungReexam",
		"reexam":              "ExpungReexam",
		"petitions":           "PetitionsToDirector",
		"PetitionsToDirector": "PetitionsToDirector",
		"admin":               "Administrative",
		"Administrative":      "Administrative",
	} {
		if got, err := tmDecisionCategory(raw); err != nil || got != want {
			t.Errorf("tmDecisionCategory(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}

func TestTrademarkDecisionSort(t *testing.T) {
	for expr, want := range map[string]string{
		"":                       "",
		"decisionActionDate":     "decisionActionDate desc",
		"decisionActionDate:ASC": "decisionActionDate asc",
		"decisionActionDate asc": "decisionActionDate asc",
		" serialNumber : desc ":  "serialNumber desc",
	} {
		if got := tmDecisionSortParam(expr); got != want {
			t.Errorf("tmDecisionSortParam(%q) = %q, want %q", expr, got, want)
		}
	}
	if got, want := tmDecisionStableSort(""), "decisionActionDate desc,serialNumber asc,petitionNumber asc,documentTypeDescriptionText asc,fileSize asc,partyName asc"; got != want {
		t.Errorf("default stable sort = %q, want %q", got, want)
	}
	if got, want := tmDecisionStableSort("serialNumber desc"), "serialNumber desc,petitionNumber asc,documentTypeDescriptionText asc,fileSize asc,partyName asc"; got != want {
		t.Errorf("stable sort = %q, want %q", got, want)
	}
}

func TestNormalizeTrademarkDecision(t *testing.T) {
	record := map[string]interface{}{
		"serialNumber":                float64(87712766),
		"registrationNumber":          float64(5513424),
		"investigationNumber":         float64(0),
		"fileSize":                    float64(1097),
		"proceedingNumber":            "",
		"documentTypeDescriptionText": "PETITION FOR EXPUNGEMENT RECEIVED ",
		"partyName":                   "In re Stelcore, et al.\t",
		"precedentialIndicator":       false,
		"ocrSearchText":               "PTO-2312\nApproved...",
		"petitionerName":              nil,
	}
	got := normalizeTrademarkDecision(record, true)
	want := map[string]interface{}{
		"serialNumber":                "87712766",
		"registrationNumber":          "5513424",
		"fileSize":                    float64(1097),
		"documentTypeDescriptionText": "PETITION FOR EXPUNGEMENT RECEIVED",
		"partyName":                   "In re Stelcore, et al.",
		"precedentialIndicator":       false,
		"ocrSearchText":               "PTO-2312\nApproved...",
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalized =\n%v\nwant\n%v", got, want)
	}
	if _, ok := normalizeTrademarkDecision(record, false)["ocrSearchText"]; ok {
		t.Fatal("ocrSearchText kept although includeText is false")
	}
}

func TestTrademarkDecisionFileTarget(t *testing.T) {
	tests := []struct {
		name     string
		record   map[string]interface{}
		wantPath string
		wantFile string
	}{
		{"file name", map[string]interface{}{"fileName": "TRR2021.pdf"}, "abc.pdf", "TRR2021.pdf"},
		{"no file name", map[string]interface{}{}, "abc.pdf", "abc.pdf"},
		{"uri extension wins", map[string]interface{}{"fileName": "order", "fileDownloadURI": "https://api.uspto.gov/x/abc.PDF"}, "abc.pdf", "order.pdf"},
		{"unsafe name", map[string]interface{}{"fileName": `..\dir/Show: "Cause"?.pdf`}, "abc.pdf", "Show_ _Cause__.pdf"},
		{"other extension", map[string]interface{}{"fileName": "notice.docx"}, "abc.docx", "notice.docx"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			apiPath, fileName := tmDecisionFileTarget("abc", test.record)
			if want := api.TrademarkDecisionsPath + "/file/proceedings-decisions/" + test.wantPath; apiPath != want {
				t.Errorf("path = %q, want %q", apiPath, want)
			}
			if fileName != test.wantFile {
				t.Errorf("file name = %q, want %q", fileName, test.wantFile)
			}
		})
	}
}

func TestIsMissingTrademarkDecisionFile(t *testing.T) {
	tests := []struct {
		head    string
		size    int64
		missing bool
	}{
		{"%PDF-1.7 ...", 1490379, false},
		{`"Requested File \"[PDR2026.pdf]\" Not Found"`, 60, true},
		{"<!doctype html>\n<html lang=\"en\">", 12258, true},
		{"  <HTML><head>", 300, true},
		{"%PDF-1.4 requested file not found", 40, true},
		{"%PDF-1.4 requested file not found", 4096, false},
	}
	for _, test := range tests {
		if got := isMissingTrademarkDecisionFile([]byte(test.head), test.size); got != test.missing {
			t.Errorf("isMissing(%q, %d) = %v, want %v", test.head, test.size, got, test.missing)
		}
	}
}

func TestTrademarkDecisionsSearchPagesStablyAndDropsRepeats(t *testing.T) {
	var requests []string
	tmDecisionTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		requests = append(requests, q.Get("offset")+"/"+q.Get("limit")+"/"+q.Get("sort"))
		if q.Get("q") != "decisionProceedingCategory:Administrative" {
			t.Errorf("q = %q", q.Get("q"))
		}
		offset, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		var rows []string
		for i := offset; i < offset+limit && i < 150; i++ {
			id := i
			if i == 100 {
				id = 99 // the service repeats a row across the page boundary
			}
			rows = append(rows, fmt.Sprintf(`{"documentIdentifier":"doc%03d","serialNumber":%d,"ocrSearchText":"text"}`, id, 90000000+id))
		}
		fmt.Fprintf(w, `{"count":150,"trademarkDecisionsDataBag":[%s],"facets":{"decisionProceedingCategory":[{"value":"Administrative","count":150}]}}`, strings.Join(rows, ","))
	})
	tmDecisionSearchFlags.category = "administrative"
	tmDecisionSearchFlags.limit = 120
	tmDecisionSearchFlags.noText = true

	out, err := captureStdout(t, func() error { return runTrademarkDecisionsSearch(trademarkDecisionsSearchCmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	stable := tmDecisionStableSort("")
	if want := []string{"/100/" + stable, "100/20/" + stable}; fmt.Sprint(requests) != fmt.Sprint(want) {
		t.Fatalf("requests = %v, want %v", requests, want)
	}
	var env struct {
		OK         bool `json:"ok"`
		Pagination struct {
			Total   int  `json:"total"`
			HasMore bool `json:"hasMore"`
		} `json:"pagination"`
		Results  []map[string]interface{} `json:"results"`
		Warnings []string                 `json:"warnings"`
		Facets   map[string]interface{}   `json:"facets"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decoding %q: %v", out, err)
	}
	if !env.OK || len(env.Results) != 119 || env.Pagination.Total != 150 || !env.Pagination.HasMore {
		t.Fatalf("ok=%v results=%d pagination=%+v", env.OK, len(env.Results), env.Pagination)
	}
	if len(env.Warnings) != 1 || !strings.Contains(env.Warnings[0], "1 record(s) on more than one page") {
		t.Fatalf("warnings = %v", env.Warnings)
	}
	if env.Results[0]["serialNumber"] != "90000000" {
		t.Fatalf("serialNumber = %#v, want a string", env.Results[0]["serialNumber"])
	}
	if _, ok := env.Results[0]["ocrSearchText"]; ok {
		t.Fatal("--no-text left ocrSearchText in the results")
	}
	if env.Facets["decisionProceedingCategory"] == nil {
		t.Fatal("facets missing")
	}
}

func TestTrademarkDecisionsSearchSinglePageKeepsCallerSort(t *testing.T) {
	tmDecisionTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("sort"); got != "decisionActionDate asc" {
			t.Errorf("sort = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Errorf("limit = %q", got)
		}
		fmt.Fprint(w, `{"count":1,"trademarkDecisionsDataBag":[{"documentIdentifier":"a"}]}`)
	})
	tmDecisionSearchFlags.sort = "decisionActionDate:asc"
	if _, err := captureStdout(t, func() error { return runTrademarkDecisionsSearch(trademarkDecisionsSearchCmd, nil) }); err != nil {
		t.Fatal(err)
	}
}

func TestTrademarkDecisionsSearchNoMatchIsEmptyResult(t *testing.T) {
	tmDecisionTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"code":404,"error":"Not Found","errorDetails":"No records found matching your search criteria"}`)
	})
	tmDecisionSearchFlags.mark = "zzzznomatch"
	out, err := captureStdout(t, func() error { return runTrademarkDecisionsSearch(trademarkDecisionsSearchCmd, nil) })
	if err != nil {
		t.Fatalf("no match returned an error: %v", err)
	}
	if !strings.Contains(out, `"results":[]`) || !strings.Contains(out, `"total":0`) {
		t.Fatalf("output = %s", out)
	}
}

func TestTrademarkDecisionsExportStitchesPages(t *testing.T) {
	const total = 6500
	row := func(i int) int {
		if i == 3000 {
			return 2999 // repeated across the page boundary
		}
		return i
	}
	for _, format := range []string{"csv", "json"} {
		t.Run(format, func(t *testing.T) {
			var downloads []string
			tmDecisionTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
				q := r.URL.Query()
				if !strings.HasSuffix(r.URL.Path, "/search/download") {
					if q.Get("limit") != "1" {
						t.Errorf("count request limit = %q", q.Get("limit"))
					}
					fmt.Fprintf(w, `{"count":%d,"trademarkDecisionsDataBag":[{"documentIdentifier":"x"}]}`, total)
					return
				}
				downloads = append(downloads, q.Get("offset")+"/"+q.Get("limit"))
				if q.Get("format") != format || q.Get("sort") != tmDecisionStableSort("") || q.Get("facets") != "" {
					t.Errorf("download params = %v", q)
				}
				offset, _ := strconv.Atoi(q.Get("offset"))
				limit, _ := strconv.Atoi(q.Get("limit"))
				if format == "csv" {
					cw := csv.NewWriter(w)
					_ = cw.Write([]string{"partyName", "documentIdentifier"})
					for i := offset; i < offset+limit && i < total; i++ {
						_ = cw.Write([]string{"Party, \"Quoted\"\nLine", fmt.Sprintf("doc%05d", row(i))})
					}
					cw.Flush()
					return
				}
				var rows []string
				for i := offset; i < offset+limit && i < total; i++ {
					rows = append(rows, fmt.Sprintf(`{"documentIdentifier":"doc%05d"}`, row(i)))
				}
				fmt.Fprintf(w, `{"count":%d,"trademarkDecisionsDataBag":[%s]}`, len(rows), strings.Join(rows, ","))
			})
			tmDecisionSearchFlags.facets = "partyName"
			if err := trademarkDecisionsSearchCmd.Flags().Set("download", format); err != nil {
				t.Fatal(err)
			}
			out, err := captureStdout(t, func() error { return runTrademarkDecisionsSearch(trademarkDecisionsSearchCmd, nil) })
			if err != nil {
				t.Fatal(err)
			}
			if want := []string{"/3000", "3000/3000", "6000/500"}; fmt.Sprint(downloads) != fmt.Sprint(want) {
				t.Fatalf("downloads = %v, want %v", downloads, want)
			}
			rows := 0
			if format == "csv" {
				records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
				if err != nil {
					t.Fatal(err)
				}
				rows = len(records) - 1
				if records[0][1] != "documentIdentifier" || records[1][0] != "Party, \"Quoted\"\nLine" {
					t.Fatalf("header %v, first row %v", records[0], records[1])
				}
			} else {
				var parsed struct {
					Count   int               `json:"count"`
					Records []json.RawMessage `json:"trademarkDecisionsDataBag"`
				}
				if err := json.Unmarshal([]byte(out), &parsed); err != nil {
					t.Fatal(err)
				}
				rows = len(parsed.Records)
				if parsed.Count != rows {
					t.Fatalf("count %d != %d rows", parsed.Count, rows)
				}
			}
			if rows != total-1 {
				t.Fatalf("exported %d rows, want %d (one repeat dropped)", rows, total-1)
			}
		})
	}
}

func TestTrademarkDecisionsDownload(t *testing.T) {
	serve := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/search/abc"):
				fmt.Fprint(w, `{"count":1,"trademarkDecisionsDataBag":{"documentIdentifier":"abc","fileName":"order.pdf","serialNumber":86906117,"decisionProceedingCategory":"PetitionsToDirector"}}`)
			case strings.HasSuffix(r.URL.Path, "/file/proceedings-decisions/abc.pdf"):
				fmt.Fprint(w, body)
			default:
				t.Errorf("unexpected request %s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}

	t.Run("saves the document under its file name", func(t *testing.T) {
		tmDecisionTestEnv(t, serve("%PDF-1.7 body"))
		dir := t.TempDir()
		tmDecisionDownloadOutput = dir
		out, err := captureStdout(t, func() error { return runTrademarkDecisionsDownload(trademarkDecisionsDownloadCmd, []string{"abc"}) })
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "order.pdf"))
		if err != nil || string(data) != "%PDF-1.7 body" {
			t.Fatalf("saved file = %q, %v", data, err)
		}
		if !strings.Contains(out, `"bytes":13`) || !strings.Contains(out, `"serialNumber":"86906117"`) {
			t.Fatalf("output = %s", out)
		}
	})

	t.Run("a not-found stub is an error and leaves no file", func(t *testing.T) {
		tmDecisionTestEnv(t, serve(`"Requested File \"[order.pdf]\" Not Found"`))
		dir := t.TempDir()
		tmDecisionDownloadOutput = filepath.Join(dir, "x.pdf")
		_, err := captureStdout(t, func() error { return runTrademarkDecisionsDownload(trademarkDecisionsDownloadCmd, []string{"abc"}) })
		apiErr, ok := err.(*api.UsptoAPIError)
		if !ok || apiErr.StatusCode != 404 || !strings.Contains(apiErr.Message, "uspto trademark docs list sn:86906117") {
			t.Fatalf("error = %v", err)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Fatalf("stub left behind: %v", entries)
		}
	})
}
