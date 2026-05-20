package crawler

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	readability "github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
)

type Parser struct {
	pageURL *url.URL
}

type ParsedPage struct {
	Title string
	Body  string
	Links []string
}

func NewParser(pageURL string) *Parser {
	parsed, _ := url.Parse(pageURL)
	return &Parser{pageURL: parsed}
}

func (p *Parser) Parse(input io.Reader) (ParsedPage, error) {
	body, err := io.ReadAll(input)
	if err != nil {
		return ParsedPage{}, err
	}

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ParsedPage{}, err
	}
	links := extractLinks(doc, p.pageURL)

	article, err := readability.FromReader(bytes.NewReader(body), p.pageURL)
	if err != nil {
		return ParsedPage{}, err
	}
	return ParsedPage{
		Title: strings.TrimSpace(article.Title),
		Body:  strings.TrimSpace(article.TextContent),
		Links: links,
	}, nil
}

func extractLinks(doc *html.Node, base *url.URL) []string {
	var links []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" {
					if link, ok := normalizeLink(attr.Val, base); ok {
						links = append(links, link)
					}
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return links
}

func normalizeLink(raw string, base *url.URL) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if base != nil {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	normalized, err := NormalizeURL(parsed.String())
	if err != nil {
		return "", false
	}
	return normalized, true
}
