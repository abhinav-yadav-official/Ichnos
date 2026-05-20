package search

import (
	"encoding/json"
	"fmt"
	"strings"
)

const defaultPageSize = 10

func BuildQuery(rawQuery, domain string, page, size int) ([]byte, error) {
	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = defaultPageSize
	}

	multiMatch := map[string]any{
		"multi_match": map[string]any{
			"query":     query,
			"fields":    []string{"title^3", "body"},
			"type":      "best_fields",
			"fuzziness": "AUTO",
		},
	}

	searchQuery := any(multiMatch)
	if normalizedDomain := strings.TrimSpace(domain); normalizedDomain != "" {
		searchQuery = map[string]any{
			"bool": map[string]any{
				"must": multiMatch,
				"filter": []any{
					map[string]any{
						"term": map[string]any{"domain": normalizedDomain},
					},
				},
			},
		}
	}

	return json.Marshal(map[string]any{
		"query": searchQuery,
		"highlight": map[string]any{
			"fields": map[string]any{
				"body": map[string]any{
					"fragment_size":       150,
					"number_of_fragments": 2,
				},
			},
		},
		"from": (page - 1) * size,
		"size": size,
	})
}
