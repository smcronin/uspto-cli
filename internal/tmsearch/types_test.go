package tmsearch

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSearchRequest(t *testing.T) {
	track := false
	request, err := BuildSearchRequest(`CM:"United States Patent and Trademark Office" AND IC:042`, SearchOptions{
		Source:         []string{" id ", "wordmark", "id", "", "internationalClass"},
		Limit:          25,
		Offset:         50,
		TrackTotalHits: &track,
		Sort:           StableSort(SortAscending),
		Facets: []TermFacet{
			{
				Name:  "classes",
				Field: "internationalClassExact",
				Size:  20,
				Options: map[string]any{
					"min_doc_count": 2,
					"order":         map[string]string{"_count": "desc"},
				},
			},
			{
				Name:    "status",
				Field:   "alive",
				Options: map[string]any{"missing": false},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := request.Query.QueryString.Query; got != `CM:"United States Patent and Trademark Office" AND IC:042` {
		t.Fatalf("query was rewritten: %q", got)
	}
	if request.Size != 25 || request.From != 50 || request.TrackTotalHits {
		t.Fatalf("unexpected paging/tracking: %#v", request)
	}
	if want := []string{"id", "wordmark", "internationalClass"}; !reflect.DeepEqual(request.Source, want) {
		t.Fatalf("source = %#v, want %#v", request.Source, want)
	}
	classes := request.Aggregations["classes"].Terms
	if classes["field"] != "internationalClassExact" || classes["size"] != 20 || classes["min_doc_count"] != 2 {
		t.Fatalf("unexpected classes facet: %#v", classes)
	}
	if request.Aggregations["status"].Terms["missing"] != false {
		t.Fatalf("unexpected status facet: %#v", request.Aggregations["status"])
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["aggs"]; !ok {
		t.Fatalf("request does not use Elasticsearch aggs key: %s", data)
	}
	if _, ok := body["_source"]; !ok {
		t.Fatalf("request does not use Elasticsearch _source key: %s", data)
	}
	sorts, ok := body["sort"].([]any)
	if !ok || len(sorts) != 3 {
		t.Fatalf("unexpected sort JSON: %s", data)
	}
	for i, field := range []string{"wordmarkExact", "internationalClassExact", "id"} {
		entry := sorts[i].(map[string]any)
		if entry[field] != "asc" {
			t.Fatalf("sort[%d] = %#v", i, entry)
		}
	}
}

func TestBuildSearchRequestDefaults(t *testing.T) {
	request, err := BuildSearchRequest("WM:APPLE", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Size != DefaultSearchLimit {
		t.Fatalf("size = %d, want %d", request.Size, DefaultSearchLimit)
	}
	if !request.TrackTotalHits {
		t.Fatal("track_total_hits should default to true")
	}
	if request.From != 0 || request.Source != nil || request.Aggregations != nil {
		t.Fatalf("unexpected defaults: %#v", request)
	}
}

func TestBuildSearchRequestCountOnlyOmitsHitsAndShaping(t *testing.T) {
	request, err := BuildSearchRequest("CM:NIKE", SearchOptions{
		CountOnly: true,
		Limit:     100,
		Offset:    50,
		Source:    []string{"id"},
		Sort:      []SortSpec{{Field: SortID, Direction: SortAscending}},
		Facets:    []TermFacet{{Name: "status", Field: "alive", Size: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Size != 0 || request.From != 0 || len(request.Source) != 0 || len(request.Sort) != 0 || len(request.Aggregations) != 0 || !request.TrackTotalHits {
		t.Fatalf("count-only request = %#v", request)
	}
}

func TestBuildSearchRequestValidation(t *testing.T) {
	tests := []struct {
		name  string
		query string
		opts  SearchOptions
		want  string
	}{
		{name: "empty query", query: " \t", want: "query cannot be empty"},
		{name: "negative limit", query: "WM:A", opts: SearchOptions{Limit: -1}, want: "limit cannot be negative"},
		{name: "negative offset", query: "WM:A", opts: SearchOptions{Offset: -1}, want: "offset cannot be negative"},
		{name: "empty sort field", query: "WM:A", opts: SearchOptions{Sort: []SortSpec{{Direction: SortAscending}}}, want: "sort field cannot be empty"},
		{name: "bad direction", query: "WM:A", opts: SearchOptions{Sort: []SortSpec{{Field: SortID, Direction: "sideways"}}}, want: "expected asc or desc"},
		{name: "empty facet name", query: "WM:A", opts: SearchOptions{Facets: []TermFacet{{Field: "alive"}}}, want: "facet name cannot be empty"},
		{name: "empty facet field", query: "WM:A", opts: SearchOptions{Facets: []TermFacet{{Name: "status"}}}, want: `facet "status" field cannot be empty`},
		{name: "negative facet size", query: "WM:A", opts: SearchOptions{Facets: []TermFacet{{Name: "status", Field: "alive", Size: -1}}}, want: `facet "status" size cannot be negative`},
		{name: "duplicate facet", query: "WM:A", opts: SearchOptions{Facets: []TermFacet{{Name: "status", Field: "alive"}, {Name: " status ", Field: "alive"}}}, want: `duplicate Trademark Search facet name "status"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildSearchRequest(test.query, test.opts)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestSortSpecRoundTrip(t *testing.T) {
	want := SortSpec{Field: SortWordmarkExact, Direction: SortDescending}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"wordmarkExact":"desc"}` {
		t.Fatalf("sort JSON = %s", data)
	}
	var got SortSpec
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round-trip = %#v, want %#v", got, want)
	}
}

func TestHitsAcceptNativeElasticsearchTotal(t *testing.T) {
	var hits Hits
	if err := json.Unmarshal([]byte(`{"total":{"value":123,"relation":"gte"},"hits":[]}`), &hits); err != nil {
		t.Fatal(err)
	}
	if hits.TotalValue != 123 || hits.TotalRelation != "gte" {
		t.Fatalf("unexpected total: %#v", hits)
	}
}

func TestSearchResponseTermsAndDocumentDecode(t *testing.T) {
	data := []byte(`{
		"took":4,
		"timedOut":false,
		"hits":{"totalValue":1,"totalRelation":"eq","maxScore":null,"hits":[
			{"index":"tmsearch-index","type":"_doc","id":"12345678","score":null,"source":{"id":"12345678","wordmark":"EXAMPLE","alive":true}}
		]},
		"aggregations":{"status":{"doc_count_error_upper_bound":0,"sum_other_doc_count":0,"buckets":[{"key":1,"key_as_string":"true","doc_count":1,"future":{"value":7}}]}}
	}`)
	var response SearchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Hits.Hits) != 1 {
		t.Fatalf("hits = %#v", response.Hits)
	}
	hit := response.Hits.Hits[0]
	if hit.Source["wordmark"] != "EXAMPLE" || !json.Valid(hit.RawSource) {
		t.Fatalf("source = %#v, raw = %s", hit.Source, hit.RawSource)
	}
	var decoded struct {
		ID    string `json:"id"`
		Alive bool   `json:"alive"`
	}
	if err := hit.Source.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "12345678" || !decoded.Alive {
		t.Fatalf("decoded source = %#v", decoded)
	}

	terms, err := response.Terms("status")
	if err != nil {
		t.Fatal(err)
	}
	if len(terms.Buckets) != 1 || terms.Buckets[0].KeyAsString != "true" || terms.Buckets[0].DocCount != 1 {
		t.Fatalf("terms = %#v", terms)
	}
	if !json.Valid(terms.Raw) || !json.Valid(terms.Buckets[0].Raw) {
		t.Fatalf("raw facet JSON not retained: %s / %s", terms.Raw, terms.Buckets[0].Raw)
	}
	if _, err := response.Terms("missing"); err == nil {
		t.Fatal("expected missing aggregation error")
	}
}
