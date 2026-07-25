package gogit

import (
	"context"
	"runtime"
	"sync"
)

// RepoResult holds the outcome of a parallel operation on one repository.
type RepoResult[T any] struct {
	Path  string
	Value T
	Err   error
}

// RunAll executes fn on each repository path in parallel, returning results
// in the same order as paths. Workers controls concurrency; 0 defaults to
// GOMAXPROCS. If ctx is cancelled, in-flight operations finish but queued
// ones are skipped.
func RunAll[T any](ctx context.Context, paths []string, fn func(ctx context.Context, repo *Repo) (T, error), workers int) []RepoResult[T] {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(paths) {
		workers = len(paths)
	}
	if workers < 1 {
		workers = 1
	}

	type workItem struct {
		index int
		path  string
	}

	workCh := make(chan workItem, len(paths))
	results := make([]RepoResult[T], len(paths))

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for work := range workCh {
				results[work.index].Path = work.path

				if err := ctx.Err(); err != nil {
					results[work.index].Err = err
					continue
				}

				repo, err := Open(work.path)
				if err != nil {
					results[work.index].Err = err
					continue
				}

				val, err := fn(ctx, repo)
				results[work.index].Value = val
				results[work.index].Err = err
			}
		})
	}

	for i, p := range paths {
		workCh <- workItem{index: i, path: p}
	}
	close(workCh)

	wg.Wait()
	return results
}

// RunAllPaths executes fn on each path in parallel without requiring a git
// repository. This is the lower-level variant for operations that manage
// their own Repo lifecycle or work on non-repo directories.
func RunAllPaths[T any](ctx context.Context, paths []string, fn func(ctx context.Context, path string) (T, error), workers int) []RepoResult[T] {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(paths) {
		workers = len(paths)
	}
	if workers < 1 {
		workers = 1
	}

	type workItem struct {
		index int
		path  string
	}

	workCh := make(chan workItem, len(paths))
	results := make([]RepoResult[T], len(paths))

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for work := range workCh {
				results[work.index].Path = work.path

				if err := ctx.Err(); err != nil {
					results[work.index].Err = err
					continue
				}

				val, err := fn(ctx, work.path)
				results[work.index].Value = val
				results[work.index].Err = err
			}
		})
	}

	for i, p := range paths {
		workCh <- workItem{index: i, path: p}
	}
	close(workCh)

	wg.Wait()
	return results
}

// ProgressFunc is called during parallel operations with progress updates.
type ProgressFunc func(completed, total int, path string)

// RunAllWithProgress is like RunAll but reports progress via a callback.
func RunAllWithProgress[T any](ctx context.Context, paths []string, fn func(ctx context.Context, repo *Repo) (T, error), workers int, progress ProgressFunc) []RepoResult[T] {
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(paths) {
		workers = len(paths)
	}
	if workers < 1 {
		workers = 1
	}

	type workItem struct {
		index int
		path  string
	}
	type resultItem struct {
		index  int
		result RepoResult[T]
	}

	workCh := make(chan workItem, len(paths))
	resultCh := make(chan resultItem, len(paths))

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for work := range workCh {
				r := RepoResult[T]{Path: work.path}

				if err := ctx.Err(); err != nil {
					r.Err = err
					resultCh <- resultItem{index: work.index, result: r}
					continue
				}

				repo, err := Open(work.path)
				if err != nil {
					r.Err = err
					resultCh <- resultItem{index: work.index, result: r}
					continue
				}

				val, err := fn(ctx, repo)
				r.Value = val
				r.Err = err
				resultCh <- resultItem{index: work.index, result: r}
			}
		})
	}

	go func() {
		for i, p := range paths {
			workCh <- workItem{index: i, path: p}
		}
		close(workCh)
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]RepoResult[T], len(paths))
	completed := 0
	for item := range resultCh {
		results[item.index] = item.result
		completed++
		if progress != nil {
			progress(completed, len(paths), item.result.Path)
		}
	}

	return results
}
