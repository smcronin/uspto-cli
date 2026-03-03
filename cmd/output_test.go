package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/smcronin/uspto-cli/internal/types"
)

// ---------------------------------------------------------------------------
// toSlice
// ---------------------------------------------------------------------------

func TestToSlice(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  []interface{}
	}{
		{
			name:  "nil returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "single string wraps in slice",
			input: "hello",
			want:  []interface{}{"hello"},
		},
		{
			name:  "single int wraps in slice",
			input: 42,
			want:  []interface{}{42},
		},
		{
			name:  "string slice converts",
			input: []string{"a", "b", "c"},
			want:  []interface{}{"a", "b", "c"},
		},
		{
			name:  "int slice converts",
			input: []int{1, 2, 3},
			want:  []interface{}{1, 2, 3},
		},
		{
			name:  "empty slice returns empty",
			input: []string{},
			want:  []interface{}{},
		},
		{
			name:  "single struct wraps in slice",
			input: struct{ Name string }{Name: "test"},
			want:  []interface{}{struct{ Name string }{Name: "test"}},
		},
		{
			name:  "slice of maps converts",
			input: []map[string]string{{"k": "v"}},
			want:  []interface{}{map[string]string{"k": "v"}},
		},
		{
			name:  "single map wraps in slice",
			input: map[string]int{"a": 1},
			want:  []interface{}{map[string]int{"a": 1}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := toSlice(tc.input)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(got), len(tc.want))
			}
			for i := range tc.want {
				if !reflect.DeepEqual(got[i], tc.want[i]) {
					t.Errorf("index %d: got %v (%T), want %v (%T)", i, got[i], got[i], tc.want[i], tc.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// flattenMap
// ---------------------------------------------------------------------------

func TestFlattenMap(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		input  map[string]interface{}
		want   map[string]string
	}{
		{
			name:   "flat map no prefix",
			prefix: "",
			input:  map[string]interface{}{"a": "hello", "b": float64(42)},
			want:   map[string]string{"a": "hello", "b": "42"},
		},
		{
			name:   "flat map with prefix",
			prefix: "root",
			input:  map[string]interface{}{"x": "val"},
			want:   map[string]string{"root.x": "val"},
		},
		{
			name:   "nested map uses dot notation",
			prefix: "",
			input: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "deep",
				},
			},
			want: map[string]string{"outer.inner": "deep"},
		},
		{
			name:   "deeply nested map",
			prefix: "",
			input: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "leaf",
					},
				},
			},
			want: map[string]string{"a.b.c": "leaf"},
		},
		{
			name:   "nil value becomes empty string",
			prefix: "",
			input:  map[string]interface{}{"key": nil},
			want:   map[string]string{"key": ""},
		},
		{
			name:   "array value becomes JSON",
			prefix: "",
			input:  map[string]interface{}{"tags": []interface{}{"a", "b"}},
			want:   map[string]string{"tags": `["a","b"]`},
		},
		{
			name:   "bool value formats as string",
			prefix: "",
			input:  map[string]interface{}{"active": true},
			want:   map[string]string{"active": "true"},
		},
		{
			name:   "empty map produces no keys",
			prefix: "",
			input:  map[string]interface{}{},
			want:   map[string]string{},
		},
		{
			name:   "mixed types at same level",
			prefix: "",
			input: map[string]interface{}{
				"name":   "test",
				"count":  float64(3),
				"nested": map[string]interface{}{"k": "v"},
				"list":   []interface{}{float64(1), float64(2)},
				"empty":  nil,
			},
			want: map[string]string{
				"name":     "test",
				"count":    "3",
				"nested.k": "v",
				"list":     "[1,2]",
				"empty":    "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := make(map[string]string)
			flattenMap(tc.prefix, tc.input, got)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// flattenToMap
// ---------------------------------------------------------------------------

func TestFlattenToMap(t *testing.T) {
	t.Run("struct flattens to map", func(t *testing.T) {
		type sample struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		got := flattenToMap(sample{Name: "test", Count: 5})
		if got["name"] != "test" {
			t.Errorf("name: got %q, want %q", got["name"], "test")
		}
		if got["count"] != "5" {
			t.Errorf("count: got %q, want %q", got["count"], "5")
		}
	})

	t.Run("map flattens directly", func(t *testing.T) {
		input := map[string]interface{}{"key": "value"}
		got := flattenToMap(input)
		if got["key"] != "value" {
			t.Errorf("key: got %q, want %q", got["key"], "value")
		}
	})

	t.Run("nested struct flattens with dots", func(t *testing.T) {
		type inner struct {
			Val string `json:"val"`
		}
		type outer struct {
			Inner inner `json:"inner"`
		}
		got := flattenToMap(outer{Inner: inner{Val: "deep"}})
		if got["inner.val"] != "deep" {
			t.Errorf("inner.val: got %q, want %q", got["inner.val"], "deep")
		}
	})

	t.Run("scalar becomes value key", func(t *testing.T) {
		got := flattenToMap("just a string")
		if _, ok := got["value"]; !ok {
			t.Fatal("expected 'value' key for scalar input")
		}
	})

	t.Run("integer scalar becomes value key", func(t *testing.T) {
		got := flattenToMap(42)
		if _, ok := got["value"]; !ok {
			t.Fatal("expected 'value' key for integer input")
		}
	})

	t.Run("struct with nil pointer", func(t *testing.T) {
		type s struct {
			Name *string `json:"name"`
		}
		got := flattenToMap(s{Name: nil})
		if got["name"] != "" {
			t.Errorf("name: got %q, want empty string for nil pointer", got["name"])
		}
	})

	t.Run("struct with slice field", func(t *testing.T) {
		type s struct {
			Tags []string `json:"tags"`
		}
		got := flattenToMap(s{Tags: []string{"a", "b"}})
		if got["tags"] != `["a","b"]` {
			t.Errorf("tags: got %q, want %q", got["tags"], `["a","b"]`)
		}
	})

	t.Run("empty struct", func(t *testing.T) {
		type empty struct{}
		got := flattenToMap(empty{})
		if len(got) != 0 {
			t.Errorf("expected empty map for empty struct, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// sortStrings
// ---------------------------------------------------------------------------

func TestSortStrings(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "nil slice no panic",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty slice",
			input: []string{},
			want:  []string{},
		},
		{
			name:  "single element",
			input: []string{"a"},
			want:  []string{"a"},
		},
		{
			name:  "already sorted",
			input: []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "reverse order",
			input: []string{"c", "b", "a"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "mixed order",
			input: []string{"banana", "apple", "cherry"},
			want:  []string{"apple", "banana", "cherry"},
		},
		{
			name:  "duplicates",
			input: []string{"b", "a", "b", "a"},
			want:  []string{"a", "a", "b", "b"},
		},
		{
			name:  "case sensitive sorting",
			input: []string{"B", "a", "A", "b"},
			want:  []string{"A", "B", "a", "b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Copy to avoid mutating test data across runs if needed.
			var s []string
			if tc.input != nil {
				s = make([]string, len(tc.input))
				copy(s, tc.input)
			}
			sortStrings(s)
			if !reflect.DeepEqual(s, tc.want) {
				t.Errorf("got %v, want %v", s, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// writeFacetsTable — verify it does not panic on nil or empty facets
// ---------------------------------------------------------------------------

func TestWriteFacetsTable_NilFacets(t *testing.T) {
	// writeFacetsTable writes to os.Stdout; redirect to discard output.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	defer func() {
		w.Close()
		r.Close()
		os.Stdout = oldStdout
	}()

	// Should not panic.
	writeFacetsTable(nil)
}

func TestWriteFacetsTable_EmptyMap(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	defer func() {
		w.Close()
		r.Close()
		os.Stdout = oldStdout
	}()

	writeFacetsTable(map[string][]types.FacetValue{})
}

func TestWriteFacetsTable_EmptyValues(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	defer func() {
		w.Close()
		r.Close()
		os.Stdout = oldStdout
	}()

	writeFacetsTable(map[string][]types.FacetValue{
		"status": {},
	})
}

func TestWriteFacetsTable_WithData(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	facets := map[string][]types.FacetValue{
		"status": {
			{Value: "Active", Count: 10},
			{Value: "Expired", Count: 5},
		},
	}

	writeFacetsTable(facets)

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()
	if len(output) == 0 {
		t.Error("expected non-empty output for facets with data")
	}
}

// ---------------------------------------------------------------------------
// CLIResponse JSON envelope structure
// ---------------------------------------------------------------------------

func TestCLIResponse_MarshalJSON_Basic(t *testing.T) {
	resp := types.CLIResponse{
		OK:      true,
		Command: "search",
		Results: []string{"result1", "result2"},
		Version: "0.2.1",
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if raw["ok"] != true {
		t.Errorf("ok: got %v, want true", raw["ok"])
	}
	if raw["command"] != "search" {
		t.Errorf("command: got %v, want %q", raw["command"], "search")
	}
	if raw["version"] != "0.2.1" {
		t.Errorf("version: got %v, want %q", raw["version"], "0.2.1")
	}
	results, ok := raw["results"].([]interface{})
	if !ok {
		t.Fatalf("results: expected array, got %T", raw["results"])
	}
	if len(results) != 2 {
		t.Errorf("results length: got %d, want 2", len(results))
	}
}

func TestCLIResponse_MarshalJSON_WithPagination(t *testing.T) {
	resp := types.CLIResponse{
		OK:      true,
		Command: "list",
		Pagination: &types.PaginationMeta{
			Offset:  0,
			Limit:   25,
			Total:   100,
			HasMore: true,
		},
		Results: []string{},
		Version: "0.2.1",
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	pag, ok := raw["pagination"].(map[string]interface{})
	if !ok {
		t.Fatalf("pagination: expected object, got %T", raw["pagination"])
	}
	if pag["offset"] != float64(0) {
		t.Errorf("pagination.offset: got %v, want 0", pag["offset"])
	}
	if pag["limit"] != float64(25) {
		t.Errorf("pagination.limit: got %v, want 25", pag["limit"])
	}
	if pag["total"] != float64(100) {
		t.Errorf("pagination.total: got %v, want 100", pag["total"])
	}
	if pag["hasMore"] != true {
		t.Errorf("pagination.hasMore: got %v, want true", pag["hasMore"])
	}
}

func TestCLIResponse_MarshalJSON_NilPaginationOmitted(t *testing.T) {
	resp := types.CLIResponse{
		OK:      true,
		Command: "get",
		Results: "single",
		Version: "0.2.1",
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, exists := raw["pagination"]; exists {
		t.Error("pagination should be omitted when nil")
	}
}

func TestCLIResponse_MarshalJSON_WithFacets(t *testing.T) {
	resp := types.CLIResponse{
		OK:      true,
		Command: "search",
		Results: []string{},
		Facets: map[string][]types.FacetValue{
			"type": {
				{Value: "utility", Count: 50},
				{Value: "design", Count: 20},
			},
		},
		Version: "0.2.1",
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	facets, ok := raw["facets"].(map[string]interface{})
	if !ok {
		t.Fatalf("facets: expected object, got %T", raw["facets"])
	}

	typeFacet, ok := facets["type"].([]interface{})
	if !ok {
		t.Fatalf("facets.type: expected array, got %T", facets["type"])
	}
	if len(typeFacet) != 2 {
		t.Errorf("facets.type length: got %d, want 2", len(typeFacet))
	}

	first, ok := typeFacet[0].(map[string]interface{})
	if !ok {
		t.Fatalf("facets.type[0]: expected object, got %T", typeFacet[0])
	}
	if first["value"] != "utility" {
		t.Errorf("facets.type[0].value: got %v, want %q", first["value"], "utility")
	}
	if first["count"] != float64(50) {
		t.Errorf("facets.type[0].count: got %v, want 50", first["count"])
	}
}

func TestCLIResponse_MarshalJSON_NilFacetsOmitted(t *testing.T) {
	resp := types.CLIResponse{
		OK:      true,
		Command: "get",
		Results: "data",
		Version: "0.2.1",
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, exists := raw["facets"]; exists {
		t.Error("facets should be omitted when nil")
	}
}

func TestCLIResponse_MarshalJSON_ErrorEnvelope(t *testing.T) {
	resp := types.CLIResponse{
		OK:      false,
		Version: "0.2.1",
		Error: &types.CLIError{
			Code:    404,
			Type:    "not_found",
			Message: "Patent not found",
			Hint:    "Check the patent number format",
		},
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if raw["ok"] != false {
		t.Errorf("ok: got %v, want false", raw["ok"])
	}

	errObj, ok := raw["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error: expected object, got %T", raw["error"])
	}
	if errObj["code"] != float64(404) {
		t.Errorf("error.code: got %v, want 404", errObj["code"])
	}
	if errObj["type"] != "not_found" {
		t.Errorf("error.type: got %v, want %q", errObj["type"], "not_found")
	}
	if errObj["message"] != "Patent not found" {
		t.Errorf("error.message: got %v, want %q", errObj["message"], "Patent not found")
	}
	if errObj["hint"] != "Check the patent number format" {
		t.Errorf("error.hint: got %v, want %q", errObj["hint"], "Check the patent number format")
	}
}

func TestCLIResponse_MarshalJSON_NilErrorOmitted(t *testing.T) {
	resp := types.CLIResponse{
		OK:      true,
		Command: "search",
		Results: "ok",
		Version: "0.2.1",
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if _, exists := raw["error"]; exists {
		t.Error("error should be omitted when nil")
	}
}
