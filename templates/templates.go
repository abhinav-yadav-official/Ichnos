package templates

import (
	"embed"
	"html/template"
	"strings"
)

//go:embed *.html
var files embed.FS

func Parse() (*template.Template, error) {
	return template.New("").
		Funcs(template.FuncMap{
			"markHighlights": markHighlights,
			"seq":            seq,
			"dec":            func(value int) int { return value - 1 },
			"inc":            func(value int) int { return value + 1 },
		}).
		ParseFS(files, "*.html")
}

func markHighlights(snippet string) template.HTML {
	escaped := template.HTMLEscapeString(snippet)
	escaped = strings.ReplaceAll(escaped, "&lt;em&gt;", "<mark>")
	escaped = strings.ReplaceAll(escaped, "&lt;/em&gt;", "</mark>")
	return template.HTML(escaped)
}

func seq(start, end int) []int {
	if end < start {
		return nil
	}
	values := make([]int, 0, end-start+1)
	for value := start; value <= end; value++ {
		values = append(values, value)
	}
	return values
}
