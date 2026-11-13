package cmd

import (
	"strings"
	"time"
)

const isoDateLayout = "2006-01-02"

// validateISODate validates an optional YYYY-MM-DD date string.
func validateISODate(flagName, value string) error {
	if value == "" {
		return nil
	}
	t, err := time.Parse(isoDateLayout, value)
	if err != nil || t.Format(isoDateLayout) != value {
		return usageErrorf("invalid %s %q: expected YYYY-MM-DD", flagName, value)
	}
	return nil
}

// validateDateRange validates optional from/to dates and their order.
func validateDateRange(fromFlag, fromVal, toFlag, toVal string) error {
	if err := validateISODate(fromFlag, fromVal); err != nil {
		return err
	}
	if err := validateISODate(toFlag, toVal); err != nil {
		return err
	}
	if fromVal != "" && toVal != "" && fromVal > toVal {
		return usageErrorf("invalid date range: %s (%s) must be <= %s (%s)", fromFlag, fromVal, toFlag, toVal)
	}
	return nil
}

// validateSortExpr validates sort syntax "field:asc|desc" or "field".
func validateSortExpr(flagName, expr string) error {
	if strings.TrimSpace(expr) == "" {
		return nil
	}
	parts := strings.Split(expr, ":")
	if len(parts) > 2 {
		return usageErrorf("invalid %s %q: expected field[:asc|desc]", flagName, expr)
	}
	field := strings.TrimSpace(parts[0])
	if field == "" {
		return usageErrorf("invalid %s %q: missing field", flagName, expr)
	}
	if len(parts) == 2 {
		order := strings.ToLower(strings.TrimSpace(parts[1]))
		if order != "asc" && order != "desc" {
			return usageErrorf("invalid %s %q: order must be asc or desc", flagName, expr)
		}
	}
	return nil
}
