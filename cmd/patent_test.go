package cmd

import (
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

func TestNormalizeBundleIDType(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "auto", want: "auto"},
		{in: "APP", want: "app"},
		{in: " publication ", want: "publication"},
		{in: "patent", want: "patent"},
		{in: "foo", wantErr: true},
	}

	for _, tc := range tests {
		got, err := normalizeBundleIDType(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("normalizeBundleIDType(%q): expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("normalizeBundleIDType(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeBundleIDType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizePatentIdentifier(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "US20050021049A1", want: "US20050021049A1"},
		{in: "us-2005-0021049-a1", want: "US20050021049A1"},
		{in: " 10,924,035 ", want: "10924035"},
	}

	for _, tc := range tests {
		got := normalizePatentIdentifier(tc.in)
		if got != tc.want {
			t.Fatalf("normalizePatentIdentifier(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPickMatchingPFW_Publication(t *testing.T) {
	records := []types.PatentFileWrapper{
		{
			ApplicationNumberText: "10924035",
			ApplicationMetaData: types.ApplicationMetaData{
				EarliestPublicationNumber: "US20050021049A1",
				PatentNumber:              "7284931",
			},
		},
		{
			ApplicationNumberText: "12999999",
			ApplicationMetaData: types.ApplicationMetaData{
				EarliestPublicationNumber: "US20100000001A1",
				PatentNumber:              "9000001",
			},
		},
	}

	got, err := pickMatchingPFW(records, "us-2005-0021049-a1", idTypePublication)
	if err != nil {
		t.Fatalf("pickMatchingPFW() error: %v", err)
	}
	if got.ApplicationNumberText != "10924035" {
		t.Fatalf("pickMatchingPFW() app = %s, want 10924035", got.ApplicationNumberText)
	}
}

func TestPickMatchingPFW_Ambiguous(t *testing.T) {
	records := []types.PatentFileWrapper{
		{ApplicationNumberText: "11111111"},
		{ApplicationNumberText: "22222222"},
	}

	_, err := pickMatchingPFW(records, "USNOTREAL", idTypePublication)
	if err == nil {
		t.Fatal("pickMatchingPFW() expected ambiguous error")
	}
}

func TestSanitizePathComponent(t *testing.T) {
	in := `US20050021049A1: bad/name?`
	got := sanitizePathComponent(in)
	want := "US20050021049A1__bad_name"
	if got != want {
		t.Fatalf("sanitizePathComponent(%q) = %q, want %q", in, got, want)
	}
}
