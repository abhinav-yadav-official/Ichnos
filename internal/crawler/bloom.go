package crawler

import (
	"sync"

	"github.com/bits-and-blooms/bloom/v3"
)

type SeenSet struct {
	mu     sync.Mutex
	filter *bloom.BloomFilter
}

func NewSeenSet(capacity uint, falsePositiveRate float64) *SeenSet {
	return &SeenSet{
		filter: bloom.NewWithEstimates(capacity, falsePositiveRate),
	}
}

func (s *SeenSet) Seen(url string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := s.filter.TestString(url)
	if !seen {
		s.filter.AddString(url)
	}
	return seen
}
