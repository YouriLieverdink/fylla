package commands

import (
	"fmt"
	"io"
	"sync"
)

// BatchResult holds per-key success/failure for batch operations.
type BatchResult struct {
	Key string
	Err error
}

// RunBatch runs fn for each key concurrently, collecting results.
func RunBatch(keys []string, fn func(key string) error) []BatchResult {
	results := make([]BatchResult, len(keys))
	var wg sync.WaitGroup
	for i, key := range keys {
		wg.Add(1)
		go func(idx int, k string) {
			defer wg.Done()
			results[idx] = BatchResult{Key: k, Err: fn(k)}
		}(i, key)
	}
	wg.Wait()
	return results
}

// PrintBatchResults writes per-key outcomes.
func PrintBatchResults(w io.Writer, results []BatchResult, verb string) {
	for _, r := range results {
		if r.Err != nil {
			fmt.Fprintf(w, "Failed to %s %s: %v\n", verb, r.Key, r.Err)
		} else {
			fmt.Fprintf(w, "%s %s\n", capitalize(verb), r.Key)
		}
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	b := []byte(s)
	if b[0] >= 'a' && b[0] <= 'z' {
		b[0] -= 'a' - 'A'
	}
	return string(b)
}
