package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/abhinav-yadav-official/Ichnos/internal/indexer"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type OpenSearchService struct {
	client *opensearchapi.Client
}

func NewOpenSearchService(client *opensearchapi.Client) *OpenSearchService {
	return &OpenSearchService{client: client}
}

func (s *OpenSearchService) Search(ctx context.Context, request SearchRequest) (SearchResponse, error) {
	if s == nil || s.client == nil {
		return SearchResponse{}, fmt.Errorf("opensearch client is required")
	}

	size := request.Size
	if size <= 0 {
		size = defaultPageSize
	}
	page := request.Page
	if page < 1 {
		page = 1
	}

	body, err := BuildQuery(request.Query, request.Domain, page, size)
	if err != nil {
		return SearchResponse{}, err
	}

	resp, err := s.client.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{indexer.PagesAliasName},
		Body:    bytes.NewReader(body),
	})
	if resp != nil {
		closeOpenSearchResponse(resp.Inspect().Response)
	}
	if err != nil {
		return SearchResponse{}, fmt.Errorf("search pages: %w", err)
	}

	hits := make([]Hit, 0, len(resp.Hits.Hits))
	for _, searchHit := range resp.Hits.Hits {
		var source pageSource
		if err := json.Unmarshal(searchHit.Source, &source); err != nil {
			return SearchResponse{}, fmt.Errorf("parse search hit source: %w", err)
		}
		hits = append(hits, Hit{
			URL:       source.URL,
			Title:     source.Title,
			Snippet:   snippetForHit(searchHit.Highlight, source.Body),
			Domain:    source.Domain,
			CrawledAt: source.CrawledAt,
		})
	}

	total := resp.Hits.Total.Value
	return SearchResponse{
		Hits:   hits,
		Total:  total,
		TookMS: resp.Took,
		Page:   page,
		Pages:  pagesFor(total, size),
	}, nil
}

type pageSource struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Domain    string `json:"domain"`
	CrawledAt string `json:"crawled_at"`
}

func snippetForHit(highlight map[string][]string, fallback string) string {
	if fragments := highlight["body"]; len(fragments) > 0 {
		return fragments[0]
	}
	return strings.TrimSpace(fallback)
}

func pagesFor(total, size int) int {
	if total == 0 {
		return 0
	}
	return (total + size - 1) / size
}

func closeOpenSearchResponse(resp *opensearch.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
