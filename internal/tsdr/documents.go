package tsdr

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ListDocuments returns quick normalized document metadata for a query. TSDR's
// bundle endpoint is substantially slower than the per-case metadata endpoint,
// so normalized listings use the latter and apply the documented filters and
// ordering locally. The fast endpoint omits document IDs and page URLs; use
// BundleDocuments when locators are required.
func (c *Client) ListDocuments(ctx context.Context, query DocumentQuery) (*DocumentList, error) {
	if _, err := query.Values(); err != nil {
		return nil, err
	}

	combined := &DocumentList{}
	seenIdentifiers := make(map[string]struct{}, len(query.Identifiers))
	for _, identifier := range query.Identifiers {
		if _, exists := seenIdentifiers[identifier.PathToken()]; exists {
			continue
		}
		seenIdentifiers[identifier.PathToken()] = struct{}{}
		response, err := c.CaseDocumentsXML(ctx, identifier)
		if err != nil {
			return nil, err
		}
		if response.IsNoContent() {
			continue
		}
		list, err := ParseDocumentList(response.Body)
		if err != nil {
			return nil, err
		}
		combined.Documents = append(combined.Documents, list.Documents...)
	}
	return ApplyDocumentQuery(combined, query)
}

// BundleDocuments returns the richer bundle metadata, including legacy
// document IDs and current public page URLs. The live endpoint is slower but is
// required for document selection and download workflows.
func (c *Client) BundleDocuments(ctx context.Context, query DocumentQuery) (*DocumentList, error) {
	if _, err := query.Values(); err != nil {
		return nil, err
	}
	// Ask TSDR only for the identifier set. The live endpoint has additional
	// order- and combination-sensitive filter bugs; local filtering produces a
	// stable contract while retaining the rich IDs and URL paths.
	response, err := c.DocumentsXML(ctx, DocumentQuery{Identifiers: query.Identifiers})
	if err != nil {
		return nil, err
	}
	if response.IsNoContent() {
		return ApplyDocumentQuery(&DocumentList{}, query)
	}
	list, err := ParseDocumentList(response.Body)
	if err != nil {
		return nil, err
	}
	return ApplyDocumentQuery(list, query)
}

// ApplyDocumentQuery applies TSDR's documented date, type, category, and sort
// controls to an already parsed document list. It returns a new list and does
// not reorder the caller's slice.
func ApplyDocumentQuery(list *DocumentList, query DocumentQuery) (*DocumentList, error) {
	if list == nil {
		return nil, fmt.Errorf("document list is nil")
	}
	if _, err := query.Values(); err != nil {
		return nil, err
	}

	types := make(map[string]struct{}, len(query.Types))
	for _, value := range query.Types {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value != "" {
			types[value] = struct{}{}
		}
	}
	category := strings.TrimSpace(query.Category)
	documents := make([]Document, 0, len(list.Documents))
	for _, document := range list.Documents {
		date := documentDate(document.MailRoomDate)
		if query.Date != "" && date != query.Date {
			continue
		}
		if query.FromDate != "" && (date == "" || date < query.FromDate) {
			continue
		}
		if query.ToDate != "" && (date == "" || date > query.ToDate) {
			continue
		}
		if len(types) > 0 {
			if _, ok := types[strings.ToUpper(strings.TrimSpace(document.DocumentTypeCode))]; !ok {
				continue
			}
		}
		if category != "" && !strings.EqualFold(strings.TrimSpace(document.CategoryTypeCode), category) {
			continue
		}
		documents = append(documents, document)
	}

	if err := sortDocuments(documents, query.Sort); err != nil {
		return nil, err
	}
	reindexDocuments(documents)
	return &DocumentList{Documents: documents}, nil
}

func documentDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}

func sortDocuments(documents []Document, expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil
	}
	parts := strings.Split(expression, ":")
	if len(parts) != 2 {
		return fmt.Errorf("document sort must use field:A or field:D")
	}
	field := strings.ToLower(strings.TrimSpace(parts[0]))
	direction := strings.ToUpper(strings.TrimSpace(parts[1]))
	if direction != "A" && direction != "D" {
		return fmt.Errorf("document sort direction must be A or D")
	}
	validFields := map[string]bool{
		"date": true, "type": true, "category": true, "serial": true,
		"id": true, "document": true, "description": true, "pages": true,
	}
	if !validFields[field] {
		return fmt.Errorf("unsupported document sort field %q", field)
	}

	sort.SliceStable(documents, func(i, j int) bool {
		left, right := documentSortValue(documents[i], field), documentSortValue(documents[j], field)
		less := left < right
		if field == "pages" {
			less = documents[i].TotalPageQuantity < documents[j].TotalPageQuantity
			if documents[i].TotalPageQuantity == documents[j].TotalPageQuantity {
				less = false
			}
		}
		if direction == "D" {
			if field == "pages" {
				return documents[i].TotalPageQuantity > documents[j].TotalPageQuantity
			}
			return left > right
		}
		return less
	})
	return nil
}

func documentSortValue(document Document, field string) string {
	switch field {
	case "date":
		return documentDate(document.MailRoomDate)
	case "type":
		return strings.ToUpper(document.DocumentTypeCode)
	case "category":
		return strings.ToUpper(document.CategoryTypeCode)
	case "serial":
		return document.SerialNumber
	case "id", "document":
		return document.DocumentID
	case "description":
		return strings.ToUpper(document.DocumentTypeDescriptionText)
	default:
		return ""
	}
}

func reindexDocuments(documents []Document) {
	// Follow-up commands accept one case identifier, so indices in a combined
	// list must be local to the document's case. The serial number is always
	// emitted alongside the index, giving agents an unambiguous reusable pair.
	caseIndexes := make(map[string]int)
	for i := range documents {
		caseKey := strings.TrimSpace(documents[i].SerialNumber)
		caseIndexes[caseKey]++
		documents[i].Index = caseIndexes[caseKey]
	}
}
