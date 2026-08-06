package tsdr

import (
	"fmt"
	"regexp"
	"strings"
)

// IdentifierType is a TSDR case identifier namespace.
type IdentifierType string

const (
	IdentifierAuto          IdentifierType = "auto"
	IdentifierSerial        IdentifierType = "serial"
	IdentifierRegistration  IdentifierType = "registration"
	IdentifierInternational IdentifierType = "international"
	IdentifierReference     IdentifierType = "reference"
	IdentifierProceeding    IdentifierType = "proceeding"
)

var (
	digitsOnly           = regexp.MustCompile(`^\d+$`)
	referencePattern     = regexp.MustCompile(`^[A-Z]\d{7}$`)
	internationalPattern = regexp.MustCompile(`^(?:\d{6,10}|\d{5,9}[A-Z])$`)
	proceedingPattern    = regexp.MustCompile(`^\d{10}[ER]?$`)
)

// Identifier is a normalized TSDR identifier.
type Identifier struct {
	Type  IdentifierType `json:"type"`
	Value string         `json:"value"`
}

// ParseIdentifier normalizes common human forms such as 78/787,878,
// sn78787878, rn:3500038, and ref:Z1231384. Numeric auto-detection treats
// eight digits as a serial number and other numeric values as registrations;
// international registrations should be made explicit with ir: or --id-type.
func ParseIdentifier(raw, hint string) (Identifier, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Identifier{}, fmt.Errorf("trademark identifier cannot be empty")
	}

	idType, err := normalizeIdentifierType(hint)
	if err != nil {
		return Identifier{}, err
	}

	lower := strings.ToLower(raw)
	prefixes := []struct {
		values []string
		typeID IdentifierType
	}{
		{[]string{"serial:", "serial=", "sn:", "sn="}, IdentifierSerial},
		{[]string{"registration:", "registration=", "reg:", "reg=", "rn:", "rn="}, IdentifierRegistration},
		{[]string{"international:", "international=", "ir:", "ir="}, IdentifierInternational},
		{[]string{"reference:", "reference=", "ref:", "ref="}, IdentifierReference},
		{[]string{"proceeding:", "proceeding=", "petition:", "petition=", "pn:", "pn="}, IdentifierProceeding},
	}
	for _, group := range prefixes {
		for _, prefix := range group.values {
			if strings.HasPrefix(lower, prefix) {
				if idType != IdentifierAuto && idType != group.typeID {
					return Identifier{}, fmt.Errorf("identifier prefix %q conflicts with --id-type %s", prefix, idType)
				}
				idType = group.typeID
				raw = strings.TrimSpace(raw[len(prefix):])
				lower = strings.ToLower(raw)
				break
			}
		}
	}

	// Also accept compact documented path tokens such as sn78787878.
	if idType == IdentifierAuto || idType == IdentifierSerial {
		if strings.HasPrefix(lower, "sn") && digitsOnly.MatchString(cleanNumeric(raw[2:])) {
			idType = IdentifierSerial
			raw = raw[2:]
		}
	}
	if idType == IdentifierAuto || idType == IdentifierRegistration {
		if strings.HasPrefix(lower, "rn") && digitsOnly.MatchString(cleanNumeric(raw[2:])) {
			idType = IdentifierRegistration
			raw = raw[2:]
		}
	}
	if idType == IdentifierAuto || idType == IdentifierInternational {
		if strings.HasPrefix(lower, "ir") && internationalPattern.MatchString(cleanAlphaNumeric(raw[2:])) {
			idType = IdentifierInternational
			raw = raw[2:]
		}
	}
	if idType == IdentifierAuto || idType == IdentifierReference {
		if strings.HasPrefix(lower, "ref") && len(raw) > 3 {
			idType = IdentifierReference
			raw = raw[3:]
		}
	}
	if idType == IdentifierAuto || idType == IdentifierProceeding {
		if strings.HasPrefix(lower, "pn") && len(raw) > 2 {
			idType = IdentifierProceeding
			raw = raw[2:]
		}
	}

	if idType == IdentifierAuto {
		candidate := cleanNumeric(raw)
		if digitsOnly.MatchString(candidate) {
			switch len(candidate) {
			case 5:
				idType = IdentifierRegistration
			case 6, 7:
				return Identifier{}, fmt.Errorf("numeric identifier %q is ambiguous between a US registration and international registration; prefix it with rn: or ir:", raw)
			case 8:
				// Eight digits are overwhelmingly used as a US serial number in
				// agent workflows. Use ir: explicitly for an 8-digit Madrid ID.
				idType = IdentifierSerial
			case 9, 10:
				idType = IdentifierInternational
			default:
				return Identifier{}, fmt.Errorf("cannot infer numeric identifier type from %q; prefix it with sn:, rn:, or ir:", raw)
			}
			raw = candidate
		} else if referencePattern.MatchString(cleanAlphaNumeric(raw)) {
			idType = IdentifierReference
			raw = cleanAlphaNumeric(raw)
		} else if proceedingPattern.MatchString(cleanAlphaNumeric(raw)) {
			idType = IdentifierProceeding
			raw = cleanAlphaNumeric(raw)
		} else {
			return Identifier{}, fmt.Errorf("cannot infer identifier type from %q; prefix it with sn:, rn:, ir:, ref:, or pn:", raw)
		}
	}

	var value string
	switch idType {
	case IdentifierSerial:
		value = cleanNumeric(raw)
		if !digitsOnly.MatchString(value) || len(value) != 8 {
			return Identifier{}, fmt.Errorf("invalid serial number %q: expected 8 digits", raw)
		}
	case IdentifierRegistration:
		value = cleanNumeric(raw)
		if !digitsOnly.MatchString(value) || len(value) < 5 || len(value) > 7 || strings.Trim(value, "0") == "" {
			return Identifier{}, fmt.Errorf("invalid registration number %q: expected 5-7 digits and not all zeros", raw)
		}
	case IdentifierInternational:
		value = cleanAlphaNumeric(raw)
		if !internationalPattern.MatchString(value) {
			return Identifier{}, fmt.Errorf("invalid international registration number %q: expected 6-10 digits or 5-9 digits plus a trailing letter", raw)
		}
		// Match the official TSDR UI's normalization for six-character IRs.
		if len(value) == 6 {
			value = "0" + value
		}
	case IdentifierReference:
		value = cleanAlphaNumeric(raw)
		if !referencePattern.MatchString(value) {
			return Identifier{}, fmt.Errorf("invalid USPTO reference number %q: expected one letter plus 7 digits", raw)
		}
	case IdentifierProceeding:
		value = cleanAlphaNumeric(raw)
		if !proceedingPattern.MatchString(value) {
			return Identifier{}, fmt.Errorf("invalid proceeding number %q: expected 10 digits with optional E or R suffix", raw)
		}
	default:
		return Identifier{}, fmt.Errorf("unsupported identifier type %q", idType)
	}

	return Identifier{Type: idType, Value: value}, nil
}

func normalizeIdentifierType(raw string) (IdentifierType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return IdentifierAuto, nil
	case "serial", "sn":
		return IdentifierSerial, nil
	case "registration", "reg", "rn":
		return IdentifierRegistration, nil
	case "international", "ir":
		return IdentifierInternational, nil
	case "reference", "ref":
		return IdentifierReference, nil
	case "proceeding", "petition", "pn", "expungement", "reexamination":
		return IdentifierProceeding, nil
	default:
		return "", fmt.Errorf("invalid --id-type %q: expected auto, serial, registration, international, reference, or proceeding", raw)
	}
}

func cleanNumeric(raw string) string {
	r := strings.NewReplacer(",", "", "-", "", "/", "", " ", "")
	return r.Replace(strings.TrimSpace(raw))
}

func cleanAlphaNumeric(raw string) string {
	value := strings.ToUpper(strings.TrimSpace(raw))
	r := strings.NewReplacer(",", "", "-", "", "/", "", " ", "")
	return r.Replace(value)
}

// Prefix is the identifier token used by TSDR paths and query parameters.
func (id Identifier) Prefix() string {
	switch id.Type {
	case IdentifierSerial:
		return "sn"
	case IdentifierRegistration:
		return "rn"
	case IdentifierInternational:
		return "ir"
	case IdentifierReference:
		return "ref"
	case IdentifierProceeding:
		return "pn"
	default:
		return ""
	}
}

// PathToken returns a TSDR path identifier such as sn78787878.
func (id Identifier) PathToken() string { return id.Prefix() + id.Value }
