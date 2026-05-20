package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

const (
	PagesIndexName = "pages-v1"
	PagesAliasName = "pages"
)

// NewClient creates an OpenSearch API client for the configured cluster URL.
func NewClient(address string) (*opensearchapi.Client, error) {
	if address == "" {
		return nil, fmt.Errorf("opensearch address is required")
	}

	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{Addresses: []string{address}},
	})
	if err != nil {
		return nil, fmt.Errorf("create opensearch client: %w", err)
	}
	return client, nil
}

func PagesIndexMapping() ([]byte, error) {
	return json.Marshal(map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"url":        map[string]any{"type": "keyword"},
				"title":      map[string]any{"type": "text", "boost": 3},
				"body":       map[string]any{"type": "text"},
				"domain":     map[string]any{"type": "keyword"},
				"crawled_at": map[string]any{"type": "date"},
				"word_count": map[string]any{"type": "integer"},
			},
		},
		"settings": map[string]any{
			"similarity": map[string]any{
				"default": map[string]any{"type": "BM25"},
			},
		},
	})
}

func EnsurePagesIndex(ctx context.Context, client *opensearchapi.Client) error {
	if client == nil {
		return fmt.Errorf("opensearch client is required")
	}

	exists, err := indexExists(ctx, client, PagesIndexName)
	if err != nil {
		return err
	}
	if !exists {
		body, err := PagesIndexMapping()
		if err != nil {
			return fmt.Errorf("build pages index mapping: %w", err)
		}

		resp, err := client.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
			Index: PagesIndexName,
			Body:  bytes.NewReader(body),
		})
		closeResponse(resp.Inspect().Response)
		if err != nil {
			return fmt.Errorf("create %s index: %w", PagesIndexName, err)
		}
	}

	aliasExists, err := indexAliasExists(ctx, client, PagesIndexName, PagesAliasName)
	if err != nil {
		return err
	}
	if aliasExists {
		return nil
	}

	resp, err := client.Indices.Alias.Put(ctx, opensearchapi.AliasPutReq{
		Indices: []string{PagesIndexName},
		Alias:   PagesAliasName,
	})
	closeResponse(resp.Inspect().Response)
	if err != nil {
		return fmt.Errorf("create %s alias for %s: %w", PagesAliasName, PagesIndexName, err)
	}
	return nil
}

func indexExists(ctx context.Context, client *opensearchapi.Client, index string) (bool, error) {
	resp, err := client.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{index},
	})
	defer closeResponse(resp)
	if err == nil {
		return true, nil
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("check %s index: %w", index, err)
}

func indexAliasExists(ctx context.Context, client *opensearchapi.Client, index, alias string) (bool, error) {
	resp, err := client.Indices.Alias.Exists(ctx, opensearchapi.AliasExistsReq{
		Indices: []string{index},
		Alias:   []string{alias},
	})
	defer closeResponse(resp)
	if err == nil {
		return true, nil
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("check %s alias on %s: %w", alias, index, err)
}

func closeResponse(resp *opensearch.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}
