package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// resolveApplicationInput converts an application, publication, or patent
// identifier into the application number required by file-wrapper endpoints.
// Numeric application numbers remain a zero-extra-request fast path.
func resolveApplicationInput(ctx context.Context, input, idType string) (string, error) {
	input = strings.TrimSpace(input)
	idType, err := normalizeBundleIDType(idType)
	if err != nil {
		return "", invalidArgs(err)
	}

	if idType == idTypeAuto && appNumberRegex.MatchString(input) {
		return input, nil
	}
	if idType == idTypeAuto && !looksLikeExternalPatentIdentifier(input) {
		return "", invalidArgsf("invalid identifier %q: use a 6-12 digit application number, a US publication number, or --id-type patent for a patent number", input)
	}
	if idType == idTypeApp {
		if err := validateAppNumber(input); err != nil {
			return "", err
		}
		return input, nil
	}

	var resolution *patentBundleResolution
	if idType == idTypeAuto {
		resolution, _, err = resolvePatentBundleAuto(ctx, input)
	} else {
		resolution, _, err = resolvePatentBundle(ctx, input, idType)
	}
	if err != nil {
		return "", err
	}
	return resolution.ApplicationNumber, nil
}

func looksLikeExternalPatentIdentifier(input string) bool {
	normalized := normalizePatentIdentifier(input)
	if strings.HasPrefix(normalized, "US") {
		return true
	}
	if len(normalized) < 6 || len(normalized) > 12 {
		return false
	}
	for _, r := range normalized {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// planApplicationInputDryRun prints the search requests needed to resolve an
// external identifier, without performing them. The placeholder lets callers
// show their follow-up file-wrapper requests without inventing an app number.
func planApplicationInputDryRun(input, idType string) (string, error) {
	input = strings.TrimSpace(input)
	idType, err := normalizeBundleIDType(idType)
	if err != nil {
		return "", invalidArgs(err)
	}
	if idType == idTypeAuto && appNumberRegex.MatchString(input) {
		return input, nil
	}
	if idType == idTypeAuto && !looksLikeExternalPatentIdentifier(input) {
		return "", invalidArgsf("invalid identifier %q: use a 6-12 digit application number, a US publication number, or --id-type patent for a patent number", input)
	}
	if idType == idTypeApp {
		if err := validateAppNumber(input); err != nil {
			return "", err
		}
		return input, nil
	}

	printResolutionSearch := func(resolvedAs, field string) {
		query := fmt.Sprintf(`%s:"%s"`, field, escapeQueryValue(input))
		printDryRunGET("/api/v1/patent/applications/search", map[string]string{
			"q":     query,
			"limit": "5",
		})
		fmt.Fprintf(os.Stderr, "  Resolve exact %s match to an application number.\n", resolvedAs)
	}

	switch idType {
	case idTypePublication:
		printResolutionSearch("publication", "applicationMetaData.earliestPublicationNumber")
	case idTypePatent:
		printResolutionSearch("patent", "applicationMetaData.patentNumber")
	case idTypeAuto:
		printResolutionSearch("publication", "applicationMetaData.earliestPublicationNumber")
		fmt.Fprintln(os.Stderr, "  If no exact publication match:")
		printResolutionSearch("patent", "applicationMetaData.patentNumber")
	}
	return dryRunResolvedApplicationPlaceholder, nil
}
