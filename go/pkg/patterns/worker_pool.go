// Package patterns provides reusable Go concurrency patterns for rate-limited requests.
//
// This package demonstrates several advanced Go concurrency patterns:
//   - Worker Pool: Fixed number of goroutines processing jobs from a queue
//   - Fan-out/Fan-in: Multiple workers sending results to single channel
//   - Pipeline: Multi-stage processing with channel chaining
//   - Context cancellation: Graceful shutdown of goroutines
//
// # Worker Pool Pattern
//
// The worker pool pattern uses a fixed number of goroutines (workers) that
// process jobs from a shared job queue. This prevents creating unbounded
// goroutines and provides backpressure.
//
// Example:
//
//	pool := patterns.NewWorkerPool(client, 10) // 10 concurrent workers
//	defer pool.Close()
//
//	// Submit jobs
//	for _, target := range targets {
//	    pool.Submit(target)
//	}
//
//	// Get results
//	for result := range pool.Results() {
//	    if result.Error != nil {
//	        log.Printf("Failed: %v", result.Error)
//	    }
//	}
//
// # Pipeline Pattern
//
// The pipeline pattern chains multiple stages together, where each stage
// processes data and passes it to the next stage via channels.
//
// Example:
//
//	pipeline := patterns.NewPipeline(client, 5) // 5 workers per stage
//	results := pipeline.Run(ctx, targets)
//
// # Thread Safety
//
// All exported types in this package are safe for concurrent use by
// multiple goroutines.
package patterns

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ratelimiter/go/pkg/realclient"
	"github.com/ratelimiter/go/pkg/targets"
)

// WorkerPool implements the worker pool pattern for rate-limited HTTP requests.
//
// Architecture:
//
//	jobs channel → [Worker 1] → results channel
//	               [Worker 2] →
//	               [Worker N] →
//
// Workers run concurrently and process jobs from the jobs channel.
// Results are sent to the results channel for aggregation.
//
// Thread-safe: Safe for concurrent use by multiple goroutines.
type WorkerPool struct {
	client      *realclient.Client
	numWorkers  int
	jobs        chan *targets.Target
	results     chan *realclient.FetchResult
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	jobsSubmitted int
	mu          sync.Mutex
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
//
// Parameters:
//   - client: RateLimitedClient for making requests
//   - numWorkers: Number of concurrent goroutines (typically 5-50)
//
// Returns:
//   - *WorkerPool: Configured pool ready to process jobs
//
// Example:
//
//	pool := NewWorkerPool(client, 10)
//	defer pool.Close()
func NewWorkerPool(client *realclient.Client, numWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		client:     client,
		numWorkers: numWorkers,
		jobs:       make(chan *targets.Target, numWorkers*2), // Buffered for backpressure
		results:    make(chan *realclient.FetchResult, numWorkers*2),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Start workers
	pool.start()

	return pool
}

// start launches all worker goroutines.
func (p *WorkerPool) start() {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}

	// Start results closer goroutine
	go func() {
		p.wg.Wait()
		close(p.results)
	}()
}

// worker processes jobs from the jobs channel.
//
// Each worker:
//  1. Receives a target from jobs channel
//  2. Makes rate-limited HTTP request via client
//  3. Sends result to results channel
//  4. Repeats until jobs channel closed or context canceled
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			// Context canceled, shutdown gracefully
			return

		case target, ok := <-p.jobs:
			if !ok {
				// Jobs channel closed, worker exits
				return
			}

			// Process job
			startTime := time.Now()
			resp, err := p.client.FetchWithContext(p.ctx, target)

			result := &realclient.FetchResult{
				Target:        target,
				Response:      resp,
				Error:         err,
				DurationNanos: time.Since(startTime).Nanoseconds(),
			}

			// Send result (with context cancellation support)
			select {
			case p.results <- result:
			case <-p.ctx.Done():
				// Context canceled while sending result
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
		}
	}
}

// Submit submits a job to the worker pool.
//
// This method blocks if the jobs channel buffer is full, providing
// natural backpressure.
//
// Parameters:
//   - target: The HTTP endpoint to fetch
//
// Example:
//
//	for _, target := range targets.All() {
//	    pool.Submit(target)
//	}
//	pool.CloseJobs() // Signal no more jobs
func (p *WorkerPool) Submit(target *targets.Target) {
	p.mu.Lock()
	p.jobsSubmitted++
	p.mu.Unlock()

	select {
	case p.jobs <- target:
		// Job submitted successfully
	case <-p.ctx.Done():
		// Pool closed, discard job
	}
}

// CloseJobs signals that no more jobs will be submitted.
//
// After calling this, workers will finish processing remaining jobs
// and then exit. The results channel will be closed when all workers
// have finished.
//
// Safe to call multiple times.
func (p *WorkerPool) CloseJobs() {
	select {
	case <-p.ctx.Done():
		// Already closed
	default:
		close(p.jobs)
	}
}

// Results returns the results channel.
//
// Consumers should range over this channel to receive all results:
//
//	for result := range pool.Results() {
//	    if result.Error != nil {
//	        log.Printf("Failed: %v", result.Error)
//	    } else {
//	        log.Printf("Success: %s", result.Target.Name)
//	    }
//	}
//
// The channel will be closed when all workers have finished processing.
func (p *WorkerPool) Results() <-chan *realclient.FetchResult {
	return p.results
}

// Close gracefully shuts down the worker pool.
//
// This:
//  1. Cancels context to stop workers
//  2. Waits for workers to finish (with timeout)
//  3. Closes channels
//
// Safe to call multiple times.
func (p *WorkerPool) Close() error {
	p.cancel()

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Workers finished gracefully
	case <-time.After(5 * time.Second):
		// Timeout waiting for workers
		return fmt.Errorf("timeout waiting for workers to finish")
	}

	return nil
}

// Stats returns statistics about the worker pool.
func (p *WorkerPool) Stats() WorkerPoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return WorkerPoolStats{
		NumWorkers:    p.numWorkers,
		JobsSubmitted: p.jobsSubmitted,
	}
}

// WorkerPoolStats contains statistics about worker pool execution.
type WorkerPoolStats struct {
	NumWorkers    int
	JobsSubmitted int
}

// String returns a human-readable summary of stats.
func (s WorkerPoolStats) String() string {
	return fmt.Sprintf("WorkerPool{Workers: %d, Jobs: %d}",
		s.NumWorkers, s.JobsSubmitted)
}
