package tsdr

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParseIdentifierNormalizesSupportedForms(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		hint string
		want Identifier
	}{
		{
			name: "formatted serial auto detection",
			raw:  " 78/787,878 ",
			want: Identifier{Type: IdentifierSerial, Value: "78787878"},
		},
		{
			name: "compact serial token",
			raw:  "SN78787878",
			want: Identifier{Type: IdentifierSerial, Value: "78787878"},
		},
		{
			name: "serial prefix with punctuation",
			raw:  "serial: 78-787-878",
			want: Identifier{Type: IdentifierSerial, Value: "78787878"},
		},
		{
			name: "registration prefix",
			raw:  "rn=3,500,038",
			want: Identifier{Type: IdentifierRegistration, Value: "3500038"},
		},
		{
			name: "explicit international number",
			raw:  "IR: 1 234 567",
			want: Identifier{Type: IdentifierInternational, Value: "1234567"},
		},
		{
			name: "compact trailing-letter international token",
			raw:  "ir12345A",
			want: Identifier{Type: IdentifierInternational, Value: "012345A"},
		},
		{
			name: "hinted international number",
			raw:  "1234567",
			hint: "international",
			want: Identifier{Type: IdentifierInternational, Value: "1234567"},
		},
		{
			name: "reference prefix",
			raw:  "ref: z 1231384",
			want: Identifier{Type: IdentifierReference, Value: "Z1231384"},
		},
		{
			name: "nonnumeric auto detection is reference",
			raw:  "  z1231384  ",
			want: Identifier{Type: IdentifierReference, Value: "Z1231384"},
		},
		{
			name: "hint aliases",
			raw:  "3,500,038",
			hint: "reg",
			want: Identifier{Type: IdentifierRegistration, Value: "3500038"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseIdentifier(tc.raw, tc.hint)
			if err != nil {
				t.Fatalf("ParseIdentifier(%q, %q) unexpected error: %v", tc.raw, tc.hint, err)
			}
			if got != tc.want {
				t.Fatalf("ParseIdentifier(%q, %q) = %#v, want %#v", tc.raw, tc.hint, got, tc.want)
			}
			if got.PathToken() != got.Prefix()+got.Value {
				t.Errorf("PathToken() = %q, want prefix plus value", got.PathToken())
			}
		})
	}
}

func TestParseIdentifierRejectsInvalidForms(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		hint    string
		wantErr string
	}{
		{name: "empty", raw: "  ", wantErr: "cannot be empty"},
		{name: "unknown hint", raw: "78787878", hint: "mark", wantErr: "invalid --id-type"},
		{name: "prefix conflicts with hint", raw: "sn:78787878", hint: "registration", wantErr: "conflicts"},
		{name: "serial too short", raw: "7878787", hint: "serial", wantErr: "expected 8 digits"},
		{name: "serial has letters", raw: "AB787878", hint: "serial", wantErr: "expected 8 digits"},
		{name: "registration too short", raw: "1234", hint: "registration", wantErr: "registration"},
		{name: "registration too long", raw: "12345678", hint: "registration", wantErr: "registration"},
		{name: "registration zero", raw: "00000", hint: "registration", wantErr: "registration"},
		{name: "international too short", raw: "12345", hint: "international", wantErr: "invalid international"},
		{name: "international too long", raw: "12345678901", hint: "international", wantErr: "invalid international"},
		{name: "international leading letter", raw: "A123456", hint: "international", wantErr: "invalid international"},
		{name: "international two trailing letters", raw: "123456AB", hint: "international", wantErr: "invalid international"},
		{name: "reference path separator", raw: "ref:foo/bar", wantErr: "invalid USPTO reference"},
		{name: "reference query delimiter", raw: "ref:foo?bar", wantErr: "invalid USPTO reference"},
		{name: "reference requires one leading letter", raw: "ref:AB123456", wantErr: "invalid USPTO reference"},
		{name: "reference requires seven digits", raw: "ref:Z123456", wantErr: "invalid USPTO reference"},
		{name: "empty reference", raw: "ref:", wantErr: "invalid USPTO reference"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseIdentifier(tc.raw, tc.hint)
			if err == nil {
				t.Fatalf("ParseIdentifier(%q, %q) expected an error", tc.raw, tc.hint)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ParseIdentifier(%q, %q) error = %q, want substring %q", tc.raw, tc.hint, err, tc.wantErr)
			}
		})
	}
}

func TestParseIdentifierAuthoritativeInternationalForms(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "ir:123456", want: "0123456"},
		{raw: "ir:1234567890", want: "1234567890"},
		{raw: "ir:12345A", want: "012345A"},
		{raw: "ir:123456789A", want: "123456789A"},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseIdentifier(tc.raw, "")
			if err != nil {
				t.Fatalf("ParseIdentifier(%q) unexpected error: %v", tc.raw, err)
			}
			if got.Type != IdentifierInternational || got.Value != tc.want || got.Prefix() != "ir" {
				t.Fatalf("ParseIdentifier(%q) = %#v (prefix %q), want international %q", tc.raw, got, got.Prefix(), tc.want)
			}
		})
	}
}

func TestParseIdentifierProceedingForms(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "pn:1234567890", want: "1234567890"},
		{raw: "pn:1234567890E", want: "1234567890E"},
		{raw: "pn:1234567890r", want: "1234567890R"},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := ParseIdentifier(tc.raw, "")
			if err != nil {
				t.Fatalf("ParseIdentifier(%q) unexpected error: %v", tc.raw, err)
			}
			if string(got.Type) != "proceeding" || got.Value != tc.want || got.Prefix() != "pn" || got.PathToken() != "pn"+tc.want {
				t.Fatalf("ParseIdentifier(%q) = %#v (prefix %q, token %q), want proceeding %q", tc.raw, got, got.Prefix(), got.PathToken(), tc.want)
			}
		})
	}

	for _, raw := range []string{"pn:123456789", "pn:12345678901", "pn:1234567890X", "pn:1234567890ER"} {
		t.Run("invalid_"+raw, func(t *testing.T) {
			if _, err := ParseIdentifier(raw, ""); err == nil {
				t.Fatalf("ParseIdentifier(%q) expected proceeding validation error", raw)
			}
		})
	}
}

func TestParseIdentifierNumericAutoDetectionAvoidsAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		raw      string
		wantType IdentifierType
	}{
		{raw: "12345", wantType: IdentifierRegistration},
		{raw: "78787878", wantType: IdentifierSerial},
		{raw: "123456789", wantType: IdentifierInternational},
		{raw: "1234567890", wantType: IdentifierInternational},
	} {
		got, err := ParseIdentifier(tc.raw, "")
		if err != nil {
			t.Errorf("ParseIdentifier(%q) unexpected error: %v", tc.raw, err)
			continue
		}
		if got.Type != tc.wantType {
			t.Errorf("ParseIdentifier(%q).Type = %q, want %q", tc.raw, got.Type, tc.wantType)
		}
	}

	for _, raw := range []string{"123456", "1234567"} {
		_, err := ParseIdentifier(raw, "")
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ambiguous") {
			t.Errorf("ParseIdentifier(%q) error = %v, want ambiguity requiring --id-type", raw, err)
		}
	}
}

func TestIdentifierPrefixesAndPathTokens(t *testing.T) {
	tests := []struct {
		id         Identifier
		wantPrefix string
		wantToken  string
	}{
		{Identifier{Type: IdentifierSerial, Value: "78787878"}, "sn", "sn78787878"},
		{Identifier{Type: IdentifierRegistration, Value: "3500038"}, "rn", "rn3500038"},
		{Identifier{Type: IdentifierInternational, Value: "1234567"}, "ir", "ir1234567"},
		{Identifier{Type: IdentifierReference, Value: "Z1231384"}, "ref", "refZ1231384"},
		{Identifier{Type: IdentifierAuto, Value: "78787878"}, "", "78787878"},
	}

	for _, tc := range tests {
		if got := tc.id.Prefix(); got != tc.wantPrefix {
			t.Errorf("%#v.Prefix() = %q, want %q", tc.id, got, tc.wantPrefix)
		}
		if got := tc.id.PathToken(); got != tc.wantToken {
			t.Errorf("%#v.PathToken() = %q, want %q", tc.id, got, tc.wantToken)
		}
	}
}

func TestDocumentQueryValues(t *testing.T) {
	query := DocumentQuery{
		Identifiers: []Identifier{
			{Type: IdentifierSerial, Value: "78787878"},
			{Type: IdentifierRegistration, Value: "3500038"},
			{Type: IdentifierSerial, Value: "75757575"},
			{Type: IdentifierInternational, Value: "1234567"},
			{Type: IdentifierReference, Value: "Z1231384"},
		},
		Date:     "2026-08-06",
		FromDate: "2026-01-01",
		ToDate:   "2026-08-01",
		Types:    []string{"OOA", "NOA"},
		Category: "OUT",
		Sort:     "desc",
	}

	got, err := query.Values()
	if err != nil {
		t.Fatalf("DocumentQuery.Values() unexpected error: %v", err)
	}
	want := url.Values{
		"sn":       {"78787878,75757575"},
		"rn":       {"3500038"},
		"ir":       {"1234567"},
		"ref":      {"Z1231384"},
		"date":     {"2026-08-06"},
		"fromDate": {"2026-01-01"},
		"toDate":   {"2026-08-01"},
		"type":     {"OOA,NOA"},
		"category": {"OUT"},
		"sort":     {"desc"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DocumentQuery.Values() = %#v, want %#v", got, want)
	}
}

func TestDocumentQueryValuesRejectsMissingOrInvalidIdentifier(t *testing.T) {
	if _, err := (DocumentQuery{}).Values(); err == nil || !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("empty DocumentQuery.Values() error = %v, want at-least-one error", err)
	}
	if _, err := (DocumentQuery{Identifiers: []Identifier{{Type: IdentifierAuto, Value: "78787878"}}}).Values(); err == nil || !strings.Contains(err.Error(), "invalid empty") {
		t.Fatalf("invalid DocumentQuery.Values() error = %v, want invalid-empty error", err)
	}
}
