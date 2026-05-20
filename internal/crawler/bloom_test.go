package crawler

import (
	"fmt"
	"testing"
)

func TestSeenSetHasNoFalseNegatives(t *testing.T) {
	seen := NewSeenSet(1000, 0.0001)

	for i := 0; i < 1000; i++ {
		url := fmt.Sprintf("https://example.com/page/%d", i)
		if seen.Seen(url) {
			t.Fatalf("url %q was reported seen before insertion", url)
		}
	}

	for i := 0; i < 1000; i++ {
		url := fmt.Sprintf("https://example.com/page/%d", i)
		if !seen.Seen(url) {
			t.Fatalf("url %q was not reported seen after insertion", url)
		}
	}
}
