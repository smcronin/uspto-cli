// Package tmsearch implements the public, keyless backend used by the
// official USPTO Trademark Search web application at tmsearch.uspto.gov.
//
// Trademark Search is a companion to, but is not part of, TSDR. It accepts
// Elasticsearch-style search requests and does not use a TSDR API key.
package tmsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// DefaultConfigurationURL is the runtime configuration document published
	// by the official Trademark Search web application. The search service URL
	// is intentionally discovered from this document instead of being frozen
	// into the client because USPTO versions the service path.
	DefaultConfigurationURL = "https://tmsearch.uspto.gov/configuration.json"

	// DefaultSearchLimit is used by BuildSearchRequest when no positive limit
	// is supplied.
	DefaultSearchLimit = 100
)

// Configuration is the forward-compatible subset of configuration.json used
// by this client.
type Configuration struct {
	ServiceURLSearchElastic string `json:"serviceUrlSearchElastic"`
}

// QueryStringQuery is Elasticsearch's query_string query. Query is passed
// through verbatim so Trademark Search field tags such as CM:, WM:, ON:, and
// IC: retain their official semantics.
type QueryStringQuery struct {
	Query           string   `json:"query"`
	DefaultOperator string   `json:"default_operator,omitempty"`
	Fields          []string `json:"fields,omitempty"`
	AnalyzeWildcard *bool    `json:"analyze_wildcard,omitempty"`
	Lenient         *bool    `json:"lenient,omitempty"`
}

// QueryClause is the typed query shape produced by BuildSearchRequest.
type QueryClause struct {
	QueryString QueryStringQuery `json:"query_string"`
}

// SortField is a Trademark Search exact-value field suitable for stable
// sorting. SortSpec itself accepts other non-empty fields for forward
// compatibility, while these constants cover the fields used by the official
// application.
type SortField string

const (
	SortWordmarkExact           SortField = "wordmarkExact"
	SortInternationalClassExact SortField = "internationalClassExact"
	SortID                      SortField = "id"
)

// SortDirection is an Elasticsearch sort direction.
type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

// SortSpec marshals to the Elasticsearch form {"field":"asc"}.
type SortSpec struct {
	Field     SortField
	Direction SortDirection
}

// MarshalJSON implements json.Marshaler.
func (s SortSpec) MarshalJSON() ([]byte, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]SortDirection{string(s.Field): s.Direction})
}

func (s *SortSpec) UnmarshalJSON(data []byte) error {
	var raw map[string]SortDirection
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decoding Trademark Search sort: %w", err)
	}
	if len(raw) != 1 {
		return fmt.Errorf("Trademark Search sort must contain exactly one field")
	}
	for field, direction := range raw {
		s.Field = SortField(field)
		s.Direction = direction
	}
	return s.validate()
}

func (s SortSpec) validate() error {
	if strings.TrimSpace(string(s.Field)) == "" {
		return fmt.Errorf("Trademark Search sort field cannot be empty")
	}
	if s.Direction != SortAscending && s.Direction != SortDescending {
		return fmt.Errorf("invalid Trademark Search sort direction %q: expected asc or desc", s.Direction)
	}
	return nil
}

// StableSort returns the deterministic three-field ordering used for reliable
// pagination and comparison of Trademark Search results.
func StableSort(direction SortDirection) []SortSpec {
	return []SortSpec{
		{Field: SortWordmarkExact, Direction: direction},
		{Field: SortInternationalClassExact, Direction: direction},
		{Field: SortID, Direction: direction},
	}
}

// TermFacet describes an arbitrary Elasticsearch terms aggregation. Name is
// the response aggregation key, Field is the indexed field, Size controls the
// number of buckets, and Options permits additional terms parameters such as
// min_doc_count, missing, include, exclude, or order.
type TermFacet struct {
	Name    string
	Field   string
	Size    int
	Options map[string]any
}

// AggregationRequest is deliberately flexible so new Elasticsearch terms
// options can be used without a client release.
type AggregationRequest struct {
	Terms map[string]any `json:"terms"`
}

// SearchRequest is the Elasticsearch-style body accepted by the official
// Trademark Search service. BuildSearchRequest is the convenient safe path;
// callers that need an unsupported query shape can use Client.SearchRaw.
type SearchRequest struct {
	Query          QueryClause                   `json:"query"`
	From           int                           `json:"from"`
	Size           int                           `json:"size"`
	Source         []string                      `json:"_source,omitempty"`
	TrackTotalHits bool                          `json:"track_total_hits"`
	Sort           []SortSpec                    `json:"sort,omitempty"`
	Aggregations   map[string]AggregationRequest `json:"aggs,omitempty"`
}

// SearchOptions controls the request produced by BuildSearchRequest. A nil
// TrackTotalHits defaults to true so exact totals are safe for agent-driven
// paging; use Bool(false) to explicitly disable that behavior.
type SearchOptions struct {
	Source         []string
	Limit          int
	Offset         int
	CountOnly      bool
	TrackTotalHits *bool
	Sort           []SortSpec
	Facets         []TermFacet
}

// Bool returns a pointer to v for optional boolean fields.
func Bool(v bool) *bool { return &v }

// BuildSearchRequest constructs and validates the official Trademark Search
// request shape. The query string is not rewritten or escaped.
func BuildSearchRequest(query string, opts SearchOptions) (SearchRequest, error) {
	if strings.TrimSpace(query) == "" {
		return SearchRequest{}, fmt.Errorf("Trademark Search query cannot be empty")
	}
	if opts.Limit < 0 {
		return SearchRequest{}, fmt.Errorf("Trademark Search limit cannot be negative")
	}
	if opts.Offset < 0 {
		return SearchRequest{}, fmt.Errorf("Trademark Search offset cannot be negative")
	}

	limit := opts.Limit
	if opts.CountOnly {
		limit = 0
	} else if limit == 0 {
		limit = DefaultSearchLimit
	}
	trackTotalHits := true
	if opts.TrackTotalHits != nil {
		trackTotalHits = *opts.TrackTotalHits
	}

	req := SearchRequest{
		Query:          QueryClause{QueryString: QueryStringQuery{Query: query}},
		From:           opts.Offset,
		Size:           limit,
		Source:         cleanStrings(opts.Source),
		TrackTotalHits: trackTotalHits,
		Sort:           append([]SortSpec(nil), opts.Sort...),
	}
	if opts.CountOnly {
		req.From = 0
		req.Source = nil
		req.Sort = nil
	}
	for _, sort := range req.Sort {
		if err := sort.validate(); err != nil {
			return SearchRequest{}, err
		}
	}

	if !opts.CountOnly && len(opts.Facets) > 0 {
		req.Aggregations = make(map[string]AggregationRequest, len(opts.Facets))
		for _, facet := range opts.Facets {
			name := strings.TrimSpace(facet.Name)
			field := strings.TrimSpace(facet.Field)
			if name == "" {
				return SearchRequest{}, fmt.Errorf("Trademark Search facet name cannot be empty")
			}
			if field == "" {
				return SearchRequest{}, fmt.Errorf("Trademark Search facet %q field cannot be empty", name)
			}
			if facet.Size < 0 {
				return SearchRequest{}, fmt.Errorf("Trademark Search facet %q size cannot be negative", name)
			}
			if _, duplicate := req.Aggregations[name]; duplicate {
				return SearchRequest{}, fmt.Errorf("duplicate Trademark Search facet name %q", name)
			}

			terms := cloneMap(facet.Options)
			terms["field"] = field
			if facet.Size > 0 {
				terms["size"] = facet.Size
			}
			req.Aggregations[name] = AggregationRequest{Terms: terms}
		}
	}
	return req, nil
}

func cleanStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+2)
	for key, value := range in {
		out[key] = value
	}
	return out
}

// Document is a flexible Trademark Search _source object.
type Document map[string]any

// Decode remarshal-decodes a document into a caller-defined schema.
func (d Document) Decode(dst any) error {
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("encoding Trademark Search document: %w", err)
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return fmt.Errorf("decoding Trademark Search document: %w", err)
	}
	return nil
}

// SearchResponse types the stable fields returned by the official service and
// retains the complete response in Raw for fields added by USPTO later.
type SearchResponse struct {
	Took             int64                      `json:"took"`
	TimedOut         bool                       `json:"timedOut"`
	ShardsTotal      int64                      `json:"shardsTotal"`
	ShardsSuccessful int64                      `json:"shardsSuccessful"`
	ShardsSkipped    int64                      `json:"shardsSkipped"`
	ShardsFailed     int64                      `json:"shardsFailed"`
	Hits             Hits                       `json:"hits"`
	Aggregations     map[string]json.RawMessage `json:"aggregations,omitempty"`
	Raw              json.RawMessage            `json:"-"`
}

// Hits contains the total and result documents. The custom unmarshaller also
// accepts Elasticsearch's native total object in case the wrapper evolves.
type Hits struct {
	TotalValue    int64    `json:"totalValue"`
	TotalRelation string   `json:"totalRelation"`
	MaxScore      *float64 `json:"maxScore"`
	Hits          []Hit    `json:"hits"`
}

func (h *Hits) UnmarshalJSON(data []byte) error {
	type alias Hits
	var envelope struct {
		alias
		Total json.RawMessage `json:"total"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	*h = Hits(envelope.alias)
	if h.TotalValue != 0 || len(envelope.Total) == 0 || bytes.Equal(envelope.Total, []byte("null")) {
		return nil
	}
	var number int64
	if err := json.Unmarshal(envelope.Total, &number); err == nil {
		h.TotalValue = number
		return nil
	}
	var object struct {
		Value    int64  `json:"value"`
		Relation string `json:"relation"`
	}
	if err := json.Unmarshal(envelope.Total, &object); err != nil {
		return fmt.Errorf("decoding Trademark Search hit total: %w", err)
	}
	h.TotalValue = object.Value
	h.TotalRelation = object.Relation
	return nil
}

// Hit is one Trademark Search result. Source is schema-flexible and RawSource
// preserves its original JSON representation.
type Hit struct {
	Index     string          `json:"index"`
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Score     *float64        `json:"score"`
	Source    Document        `json:"source"`
	Sort      []any           `json:"sort,omitempty"`
	RawSource json.RawMessage `json:"-"`
}

func (h *Hit) UnmarshalJSON(data []byte) error {
	type alias Hit
	var envelope struct {
		alias
		Source json.RawMessage `json:"source"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	*h = Hit(envelope.alias)
	h.RawSource = append(h.RawSource[:0], envelope.Source...)
	if len(envelope.Source) > 0 && !bytes.Equal(envelope.Source, []byte("null")) {
		if err := json.Unmarshal(envelope.Source, &h.Source); err != nil {
			return fmt.Errorf("decoding Trademark Search source for hit %q: %w", h.ID, err)
		}
	}
	return nil
}

// TermsAggregation is the common response shape for arbitrary terms facets.
type TermsAggregation struct {
	DocCountErrorUpperBound int64           `json:"doc_count_error_upper_bound"`
	SumOtherDocCount        int64           `json:"sum_other_doc_count"`
	Buckets                 []TermBucket    `json:"buckets"`
	Raw                     json.RawMessage `json:"-"`
}

// TermBucket represents either a string or numeric terms bucket. KeyAsString
// contains the server's display form when one is supplied.
type TermBucket struct {
	Key         any             `json:"key"`
	KeyAsString string          `json:"key_as_string,omitempty"`
	DocCount    int64           `json:"doc_count"`
	Raw         json.RawMessage `json:"-"`
}

// Terms decodes a named terms facet while retaining its raw JSON.
func (r *SearchResponse) Terms(name string) (TermsAggregation, error) {
	if r == nil {
		return TermsAggregation{}, fmt.Errorf("nil Trademark Search response")
	}
	raw, ok := r.Aggregations[name]
	if !ok {
		return TermsAggregation{}, fmt.Errorf("Trademark Search response has no aggregation %q", name)
	}
	var terms TermsAggregation
	if err := json.Unmarshal(raw, &terms); err != nil {
		return TermsAggregation{}, fmt.Errorf("decoding Trademark Search aggregation %q: %w", name, err)
	}
	terms.Raw = append(terms.Raw[:0], raw...)
	for i := range terms.Buckets {
		// Preserve individual bucket JSON without imposing a schema on nested
		// sub-aggregations.
		var envelope struct {
			Buckets []json.RawMessage `json:"buckets"`
		}
		if err := json.Unmarshal(raw, &envelope); err == nil && i < len(envelope.Buckets) {
			terms.Buckets[i].Raw = append(terms.Buckets[i].Raw[:0], envelope.Buckets[i]...)
		}
	}
	return terms, nil
}
