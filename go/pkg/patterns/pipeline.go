package patterns

import (
	"context"
	"net/http"
	"sync"

	"github.com/ratelimiter/go/pkg/realclient"
	"github.com/ratelimiter/go/pkg/targets"
)

// Pipeline implements a multi-stage processing pipeline for HTTP requests.
//
// Architecture:
//
//	Stage 1: Generate → Stage 2: Fetch → Stage 3: Process → Results
//	         (targets)           (HTTP)            (validate)
//
// Each stage:
//   - Receives input from previous stage via channel
//   - Processes the input
//   - Sends output to next stage via channel
//   - Runs with N concurrent workers
//
// This demonstrates:
//   - Channel chaining
//   - Multi-stage concurrency
//   - Context propagation
//   - Graceful shutdown
//
// Example:
//
//	pipeline := patterns.NewPipeline(client, 5)
//	results := pipeline.Run(ctx, allTargets)
//	for result := range results {
//	    log.Printf("Result: %+v", result)
//	}
type Pipeline struct {
	client         *realclient.Client
	workersPerStage int
}

// NewPipeline creates a new pipeline processor.
//
// Parameters:
//   - client: RateLimitedClient for making requests
//   - workersPerStage: Number of concurrent workers per stage
//
// Returns:
//   - *Pipeline: Configured pipeline ready to process
func NewPipeline(client *realclient.Client, workersPerStage int) *Pipeline {
	return &Pipeline{
		client:         client,
		workersPerStage: workersPerStage,
	}
}

// Run executes the pipeline with the given targets.
//
// The pipeline has 3 stages:
//  1. Generate: Emit targets to channel (1 goroutine)
//  2. Fetch: Make rate-limited HTTP requests (N workers)
//  3. Validate: Check response status codes (N workers)
//
// Parameters:
//   - ctx: Context for cancellation
//   - targetsToFetch: Slice of targets to process
//
// Returns:
//   - <-chan *FetchResult: Channel of results (closed when pipeline finishes)
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	pipeline := NewPipeline(client, 5)
//	results := pipeline.Run(ctx, targets.All())
//
//	for result := range results {
//	    if result.Error != nil {
//	        log.Printf("Failed: %v", result.Error)
//	    }
//	}
func (p *Pipeline) Run(ctx context.Context, targetsToFetch []*targets.Target) <-chan *realclient.FetchResult {
	// Stage 1: Generate targets
	targetsCh := p.generateStage(ctx, targetsToFetch)

	// Stage 2: Fetch HTTP requests (with rate limiting)
	fetchCh := p.fetchStage(ctx, targetsCh)

	// Stage 3: Validate responses
	resultsCh := p.validateStage(ctx, fetchCh)

	return resultsCh
}

// generateStage emits targets to a channel.
//
// This is the source stage of the pipeline.
func (p *Pipeline) generateStage(ctx context.Context, targetsToFetch []*targets.Target) <-chan *targets.Target {
	out := make(chan *targets.Target)

	go func() {
		defer close(out)

		for _, target := range targetsToFetch {
			select {
			case out <- target:
				// Target sent successfully
			case <-ctx.Done():
				// Context canceled, stop generating
				return
			}
		}
	}()

	return out
}

// fetchStage makes rate-limited HTTP requests.
//
// This is the core processing stage with N concurrent workers.
type fetchResult struct {
	target   *targets.Target
	response *http.Response
	err      error
}

func (p *Pipeline) fetchStage(ctx context.Context, targets <-chan *targets.Target) <-chan *fetchResult {
	out := make(chan *fetchResult)

	var wg sync.WaitGroup

	// Spawn N workers for this stage
	for i := 0; i < p.workersPerStage; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for target := range targets {
				// Make rate-limited request
				resp, err := p.client.FetchWithContext(ctx, target)

				result := &fetchResult{
					target:   target,
					response: resp,
					err:      err,
				}

				select {
				case out <- result:
					// Result sent
				case <-ctx.Done():
					// Context canceled, cleanup and exit
					if resp != nil {
						resp.Body.Close()
					}
					return
				}
			}
		}()
	}

	// Close output channel when all workers finish
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// validateStage validates HTTP responses.
//
// This is the final processing stage.
func (p *Pipeline) validateStage(ctx context.Context, fetchResults <-chan *fetchResult) <-chan *realclient.FetchResult {
	out := make(chan *realclient.FetchResult)

	var wg sync.WaitGroup

	// Spawn N workers for validation
	for i := 0; i < p.workersPerStage; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for fr := range fetchResults {
				result := &realclient.FetchResult{
					Target:   fr.target,
					Response: fr.response,
					Error:    fr.err,
				}

				// Validate status code if no error
				if fr.err == nil && fr.response != nil {
					if fr.response.StatusCode != fr.target.ExpectedStatus {
						// Close response on validation failure
						fr.response.Body.Close()
						result.Response = nil
						result.Error = fr.err
					}
				}

				select {
				case out <- result:
					// Result sent
				case <-ctx.Done():
					// Context canceled, cleanup and exit
					if fr.response != nil {
						fr.response.Body.Close()
					}
					return
				}
			}
		}()
	}

	// Close output channel when all workers finish
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
