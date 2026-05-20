package search

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildQueryWithDomainAndSecondPage(t *testing.T) {
	body, err := BuildQuery("golang", "github.com", 2, 10)
	if err != nil {
		t.Fatalf("BuildQuery() error = %v", err)
	}

	got := decodeJSON(t, body)
	want := decodeJSON(t, []byte(`{
		"query": {
			"bool": {
				"must": {
					"multi_match": {
						"query": "golang",
						"fields": ["title^3", "body"],
						"type": "best_fields",
						"fuzziness": "AUTO"
					}
				},
				"filter": [
					{"term": {"domain": "github.com"}}
				]
			}
		},
		"highlight": {
			"fields": {
				"body": {
					"fragment_size": 150,
					"number_of_fragments": 2
				}
			}
		},
		"from": 10,
		"size": 10
	}`))

	assertJSONEqual(t, got, want)
}

func TestBuildQueryDefaultsPageAndSizeWithoutDomain(t *testing.T) {
	body, err := BuildQuery("golang", "", 0, 0)
	if err != nil {
		t.Fatalf("BuildQuery() error = %v", err)
	}

	got := decodeJSON(t, body)
	want := decodeJSON(t, []byte(`{
		"query": {
			"multi_match": {
				"query": "golang",
				"fields": ["title^3", "body"],
				"type": "best_fields",
				"fuzziness": "AUTO"
			}
		},
		"highlight": {
			"fields": {
				"body": {
					"fragment_size": 150,
					"number_of_fragments": 2
				}
			}
		},
		"from": 0,
		"size": 10
	}`))

	assertJSONEqual(t, got, want)
}

func TestBuildQueryRejectsEmptyQuery(t *testing.T) {
	_, err := BuildQuery("   ", "", 1, 10)
	if err == nil {
		t.Fatal("BuildQuery() error = nil, want empty query error")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("BuildQuery() error = %q, want query is required", err)
	}
}

func decodeJSON(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, body)
	}
	return value
}

func assertJSONEqual(t *testing.T, got, want map[string]any) {
	t.Helper()

	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}
