package cmd

import (
	"context"
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
