package indexer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

type RawPage struct {
	URL        string
	StatusCode int
	Title      string
	Body       string
	CrawledAt  time.Time
	WordCount  int
}

type PageDocument struct {
	ID        string `json:"-"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Domain    string `json:"domain"`
	CrawledAt string `json:"crawled_at"`
	WordCount int    `json:"word_count"`
}

type BulkResult struct {
	Indexed  int
	Failed   int
	Failures []BulkFailure
}

type BulkFailure struct {
	ID     string
	Status int
	Type   string
	Reason string
}

type BulkError struct {
	Result BulkResult
}

func (e BulkError) Error() string {
	return fmt.Sprintf("bulk index failed for %d documents", e.Result.Failed)
}

type BulkIndexer struct {
	client *opensearchapi.Client
}

func NewBulkIndexer(client *opensearchapi.Client) *BulkIndexer {
	return &BulkIndexer{client: client}
}

func (i *BulkIndexer) BulkIndex(ctx context.Context, docs []PageDocument) (BulkResult, error) {
	if i == nil || i.client == nil {
		return BulkResult{}, fmt.Errorf("opensearch client is required")
	}
	if len(docs) == 0 {
		return BulkResult{}, nil
	}

	body, err := BuildBulkBody(docs)
	if err != nil {
		return BulkResult{}, err
	}

	resp, err := i.client.Bulk(ctx, opensearchapi.BulkReq{
		Body: bytes.NewReader(body),
	})
	if resp != nil {
		closeResponse(resp.Inspect().Response)
	}
	if err != nil {
		return BulkResult{}, fmt.Errorf("post bulk request: %w", err)
	}

	result := parseBulkResult(resp)
	if result.Failed > 0 {
		return result, BulkError{Result: result}
	}
	return result, nil
}

func BuildBulkBody(docs []PageDocument) ([]byte, error) {
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)

	for _, doc := range docs {
		if doc.ID == "" {
			return nil, fmt.Errorf("document ID is required for %q", doc.URL)
		}
		if err := encoder.Encode(map[string]any{
			"index": map[string]any{
				"_index": PagesAliasName,
				"_id":    doc.ID,
			},
		}); err != nil {
			return nil, err
		}
		if err := encoder.Encode(doc); err != nil {
			return nil, err
		}
	}

	return body.Bytes(), nil
}

func RawPageFromStream(values map[string]any) (RawPage, error) {
	crawledAt, err := time.Parse(time.RFC3339Nano, stringValue(values, "crawled_at"))
	if err != nil {
		return RawPage{}, fmt.Errorf("parse crawled_at: %w", err)
	}

	statusCode, err := intValue(values, "status_code")
	if err != nil {
		return RawPage{}, err
	}
	wordCount, err := intValue(values, "word_count")
	if err != nil {
		return RawPage{}, err
	}

	page := RawPage{
		URL:        stringValue(values, "url"),
		StatusCode: statusCode,
		Title:      stringValue(values, "title"),
		Body:       stringValue(values, "body"),
		CrawledAt:  crawledAt,
		WordCount:  wordCount,
	}
	if page.URL == "" {
		return RawPage{}, errors.New("url is required")
	}
	return page, nil
}

func (p RawPage) Document() (PageDocument, error) {
	parsed, err := url.Parse(p.URL)
	if err != nil {
		return PageDocument{}, err
	}
	domain := strings.ToLower(parsed.Hostname())
	if domain == "" {
		return PageDocument{}, fmt.Errorf("url host is required: %s", p.URL)
	}

	return PageDocument{
		ID:        DocumentID(p.URL),
		URL:       p.URL,
		Title:     p.Title,
		Body:      p.Body,
		Domain:    domain,
		CrawledAt: p.CrawledAt.UTC().Format(time.RFC3339Nano),
		WordCount: p.WordCount,
	}, nil
}

func DocumentID(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

func parseBulkResult(resp *opensearchapi.BulkResp) BulkResult {
	var result BulkResult
	if resp == nil {
		return result
	}

	for _, item := range resp.Items {
		for _, operation := range item {
			if operation.Status >= 200 && operation.Status < 300 {
				result.Indexed++
				continue
			}

			result.Failed++
			failure := BulkFailure{
				ID:     operation.ID,
				Status: operation.Status,
			}
			if operation.Error != nil {
				failure.Type = operation.Error.Type
				failure.Reason = operation.Error.Reason
			}
			result.Failures = append(result.Failures, failure)
		}
	}
	return result
}

func stringValue(values map[string]any, key string) string {
	if value, ok := values[key]; ok {
		return fmt.Sprint(value)
	}
	return ""
}

func intValue(values map[string]any, key string) (int, error) {
	value, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}

	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return parsed, nil
	default:
		parsed, err := strconv.Atoi(fmt.Sprint(typed))
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return parsed, nil
	}
}
