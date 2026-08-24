package curate

import (
	"context"
	"sync"
)

// chunkItems splits items into consecutive slices of at most size. A non-
// positive size yields a single chunk (the whole slice). nil/empty stays nil.
func chunkItems[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = len(items)
	}
	if len(items) == 0 || size == 0 {
		return nil
	}
	out := make([][]T, 0, (len(items)+size-1)/size)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}

	return out
}

// runBounded runs fn(i) for every i in [0,n) with at most `limit` goroutines in
// flight at any moment (rate guard so we never burst the model endpoint). It
// blocks until all jobs settle and returns the first non-nil error, if any.
// fn must be concurrency-safe; per-index outputs should be collected into an
// index-keyed slice (goroutine i owns slot i) to avoid map races.
func runBounded(ctx context.Context, n, limit int, fn func(context.Context, int) error) error {
	if n <= 0 {
		return nil
	}
	if limit <= 0 || limit > n {
		limit = n
	}
	sem := make(chan struct{}, limit)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			errs[i] = fn(ctx, i)
		}(i)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}

	return nil
}
