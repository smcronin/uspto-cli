package integration

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Trademark Decisions and Proceedings (ODP release 5.1). These commands use
// the ODP key, so they run wherever the patent tests run.

func TestT016a_TrademarkDecisionsHelp(t *testing.T) {
	r := runCLI("trademark", "decisions", "--help")
	assertExitCode(t, r, 0)
	for _, sub := range []string{"search", "get", "download"} {
		assertContains(t, r.Stdout, sub)
	}
	r = runCLI("trademark", "--help")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, "decisions")
}

func TestT016b_TrademarkDecisionsSearchByCategory(t *testing.T) {
	requireAPIKey(t)
	for flag, value := range map[string]string{
		"expungement-reexam": "ExpungReexam",
		"petitions":          "PetitionsToDirector",
		"administrative":     "Administrative",
	} {
		r := runCLI("trademark", "decisions", "search", "--category", flag, "--limit", "2", "-f", "json", "-q")
		assertExitCode(t, r, 0)
		env := parseEnvelope(t, r.Stdout)
		rows := parseResultsArray(t, env.Results)
		if !env.OK || len(rows) != 2 {
			t.Fatalf("--category %s: ok=%v rows=%d", flag, env.OK, len(rows))
		}
		for _, row := range rows {
			if row["decisionProceedingCategory"] != value {
				t.Errorf("--category %s returned category %v", flag, row["decisionProceedingCategory"])
			}
			if id, _ := row["documentIdentifier"].(string); id == "" {
				t.Errorf("--category %s: record without documentIdentifier", flag)
			}
		}
	}
}

func TestT016c_TrademarkDecisionsSerialSortAndStringIDs(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "search", "--serial", "sn:87712766", "--sort", "decisionActionDate:asc", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	rows := parseResultsArray(t, parseEnvelope(t, r.Stdout).Results)
	if len(rows) < 2 {
		t.Fatalf("expected several records for serial 87712766, got %d", len(rows))
	}
	last := ""
	for _, row := range rows {
		if row["serialNumber"] != "87712766" {
			t.Fatalf("serialNumber = %#v, want the string 87712766", row["serialNumber"])
		}
		date, _ := row["decisionActionDate"].(string)
		if date < last {
			t.Fatalf("not sorted ascending: %s after %s", date, last)
		}
		last = date
	}
}

func TestT016d_TrademarkDecisionsFacetsAndDateRange(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "search", "--from", "2025-01-01", "--to", "2025-12-31",
		"--facets", "decisionProceedingCategory", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	assertContains(t, r.Stdout, `"facets"`)
	assertContains(t, r.Stdout, "ExpungReexam")
	rows := parseResultsArray(t, parseEnvelope(t, r.Stdout).Results)
	if date, _ := rows[0]["decisionActionDate"].(string); !strings.HasPrefix(date, "2025-") {
		t.Fatalf("decisionActionDate %q outside 2025", date)
	}
}

func TestT016e_TrademarkDecisionsNoMatchIsEmpty(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "search", "--mark", "zzzzqqqxxnomatch", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	if rows := parseResultsArray(t, parseEnvelope(t, r.Stdout).Results); len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
}

func TestT016f_TrademarkDecisionsPagesPast100(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "search", "--category", "administrative", "--limit", "150", "--no-text", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	rows := parseResultsArray(t, parseEnvelope(t, r.Stdout).Results)
	seen := map[string]bool{}
	for _, row := range rows {
		id, _ := row["documentIdentifier"].(string)
		if seen[id] {
			t.Fatalf("record %s returned twice", id)
		}
		seen[id] = true
		if _, ok := row["ocrSearchText"]; ok {
			t.Fatal("--no-text left ocrSearchText in a record")
		}
	}
	if len(rows) != 150 {
		t.Fatalf("expected 150 rows, got %d", len(rows))
	}
}

func TestT016g_TrademarkDecisionsGetAndDownload(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "search", "--category", "administrative", "--sort", "decisionActionDate:asc", "--limit", "1", "-f", "json", "-q")
	assertExitCode(t, r, 0)
	rows := parseResultsArray(t, parseEnvelope(t, r.Stdout).Results)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	id, _ := rows[0]["documentIdentifier"].(string)

	r = runCLI("trademark", "decisions", "get", id, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	record := parseResultsObject(t, parseEnvelope(t, r.Stdout).Results)
	if record["documentIdentifier"] != id {
		t.Fatalf("get returned %v, want %s", record["documentIdentifier"], id)
	}

	out := filepath.Join(t.TempDir(), "decision.pdf")
	r = runCLI("trademark", "decisions", "download", id, "-o", out, "-f", "json", "-q")
	assertExitCode(t, r, 0)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Fatalf("download is not a PDF: starts with %q", truncate(string(data), 40))
	}
}

func TestT016h_TrademarkDecisionsUnknownIDIsNotFound(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "get", "deadbeef", "-f", "json", "-q")
	assertExitCode(t, r, 4)
}

func TestT016i_TrademarkDecisionsExportCSV(t *testing.T) {
	requireAPIKey(t)
	r := runCLI("trademark", "decisions", "search", "--category", "administrative", "--download", "csv", "--limit", "5", "-q")
	assertExitCode(t, r, 0)
	records, err := csv.NewReader(strings.NewReader(r.Stdout)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 6 {
		t.Fatalf("expected a header and 5 rows, got %d records", len(records))
	}
	if header := strings.Join(records[0], ","); !strings.Contains(header, "documentIdentifier") {
		t.Fatalf("CSV header lacks documentIdentifier: %s", header)
	}
}

func TestT016j_TrademarkDecisionsUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"trademark", "decisions", "search", "--category", "bogus"},
		{"trademark", "decisions", "search", "--serial", "12x"},
		{"trademark", "decisions", "search", "--from", "11/01/2026"},
		{"trademark", "decisions", "search", "--limit", "0"},
		{"trademark", "decisions", "nosuchcommand"},
	} {
		r := runCLI(args...)
		if r.ExitCode != 2 {
			t.Errorf("%v: exit %d, want 2", args, r.ExitCode)
		}
	}
}
