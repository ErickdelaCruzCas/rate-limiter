// Package realclient provides a rate-limited HTTP client that demonstrates
// Go concurrency patterns in a real-world E2E scenario.
//
// This package combines:
//   - Rate limiting (gRPC calls to rate limiter service)
//   - Real HTTP requests (to external public APIs)
//   - Go concurrency patterns (channels, goroutines, select)
//   - Error handling and retry logic
//   - Metrics collection
//
// # Architecture
//
// The RateLimitedClient follows this flow:
//
//	1. Client.Fetch(target) → Check rate limiter via gRPC
//	2. If ALLOW → Make real HTTP request to target.URL
//	3. If REJECT → Wait retry-after duration, then retry
//	4. Track metrics (allowed, rejected, errors)
//
// # Usage Example
//
//	// Create client
//	client, err := realclient.NewClient("localhost:50051", "user:123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Fetch from a target
//	target := targets.JSONPlaceholderPosts
//	resp, err := client.Fetch(target)
//	if err != nil {
//	    log.Printf("Request failed: %v", err)
//	}
//	defer resp.Body.Close()
//
//	// Print metrics
//	metrics := client.Metrics()
//	fmt.Printf("Allowed: %d, Rejected: %d, Failed: %d\n",
//	    metrics.Allowed, metrics.Rejected, metrics.Failed)
//
// # Concurrency Patterns
//
// This package demonstrates several Go concurrency patterns:
//
//   - Worker Pool: FetchConcurrent() uses N goroutines processing jobs
//   - Fan-out/Fan-in: Multiple workers, single results channel
//   - Select with timeout: Bounded waiting for rate limit checks
//   - Context cancellation: Graceful shutdown of goroutines
//   - Buffered channels: Job queues and result aggregation
//
// # Thread Safety
//
// All exported methods are safe for concurrent use by multiple goroutines.
// Metrics are protected by sync.Mutex.
package realclient

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	pb "github.com/ratelimiter/go/api/ratelimit/v1"
	"github.com/ratelimiter/go/pkg/targets"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is a rate-limited HTTP client that checks a gRPC rate limiter
// before making real HTTP requests.
//
// Thread-safe: Safe for concurrent use by multiple goroutines.
type Client struct {
	// gRPC connection to rate limiter service
	conn          *grpc.ClientConn
	rateLimiter   pb.RateLimitServiceClient
	httpClient    *http.Client
	rateLimitKey  string
	defaultPermits int32

	// Metrics (protected by mu)
	mu      sync.Mutex
	metrics Metrics
}

// Metrics tracks statistics for rate-limited requests.
type Metrics struct {
	// Allowed: Number of requests allowed by rate limiter
	Allowed int64

	// Rejected: Number of requests rejected by rate limiter (with retry)
	Rejected int64

	// Failed: Number of requests that failed (HTTP errors, timeouts, etc.)
	Failed int64

	// TotalHTTPRequests: Total number of successful HTTP requests made
	TotalHTTPRequests int64

	// TotalRetries: Total number of retries after rate limit rejection
	TotalRetries int64
}

// FetchResult contains the outcome of a rate-limited fetch operation.
type FetchResult struct {
	Target       *targets.Target
	Response     *http.Response
	Error        error
	WasRejected  bool
	RetryCount   int
	DurationNanos int64
}

// NewClient creates a new RateLimitedClient.
//
// Parameters:
//   - serverAddr: Address of gRPC rate limiter service (e.g., "localhost:50051")
//   - rateLimitKey: Key to use for rate limiting (e.g., "user:123", "service:api")
//
// Returns:
//   - *Client: Configured client ready to make rate-limited requests
//   - error: Connection error if gRPC dial fails
//
// Example:
//
//	client, err := NewClient("localhost:50051", "user:123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
func NewClient(serverAddr, rateLimitKey string) (*Client, error) {
	// Connect to gRPC rate limiter
	conn, err := grpc.Dial(
		serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rate limiter at %s: %w", serverAddr, err)
	}

	return &Client{
		conn:           conn,
		rateLimiter:    pb.NewRateLimitServiceClient(conn),
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		rateLimitKey:   rateLimitKey,
		defaultPermits: 1,
		metrics:        Metrics{},
	}, nil
}

// Close closes the gRPC connection to the rate limiter service.
//
// Should be called when the client is no longer needed.
// Safe to call multiple times.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Fetch makes a rate-limited HTTP request to the specified target.
//
// Flow:
//  1. Check rate limiter (gRPC call)
//  2. If ALLOW → make HTTP request immediately
//  3. If REJECT → wait retry-after duration, then retry (max 3 attempts)
//  4. Update metrics
//
// Parameters:
//   - target: The HTTP endpoint to fetch (see pkg/targets)
//
// Returns:
//   - *http.Response: HTTP response if successful (caller must close response.Body)
//   - error: Rate limit exhausted, HTTP error, or network error
//
// Example:
//
//	target := targets.JSONPlaceholderPosts
//	resp, err := client.Fetch(target)
//	if err != nil {
//	    log.Printf("Fetch failed: %v", err)
//	    return
//	}
//	defer resp.Body.Close()
//	body, _ := io.ReadAll(resp.Body)
//	fmt.Printf("Response: %s\n", body)
func (c *Client) Fetch(target *targets.Target) (*http.Response, error) {
	return c.FetchWithContext(context.Background(), target)
}

// FetchWithContext makes a rate-limited HTTP request with context support.
//
// Supports:
//   - Context cancellation (cancel request mid-flight)
//   - Context timeout (bounded waiting)
//   - Graceful shutdown (stop retries on context cancel)
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - target: The HTTP endpoint to fetch
//
// Returns:
//   - *http.Response: HTTP response if successful
//   - error: Rate limit exhausted, context canceled, or HTTP error
//
// Example with timeout:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	resp, err := client.FetchWithContext(ctx, target)
func (c *Client) FetchWithContext(ctx context.Context, target *targets.Target) (*http.Response, error) {
	const maxRetries = 3
	retryCount := 0

	for retryCount < maxRetries {
		// Check context before attempting
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 1. Check rate limiter
		allowed, retryAfterNanos, err := c.checkRateLimit(ctx)
		if err != nil {
			c.recordFailed()
			return nil, fmt.Errorf("rate limit check failed: %w", err)
		}

		// 2. If allowed, make HTTP request
		if allowed {
			c.recordAllowed()
			resp, err := c.makeHTTPRequest(ctx, target)
			if err != nil {
				c.recordFailed()
				return nil, fmt.Errorf("HTTP request failed: %w", err)
			}
			c.recordHTTPRequest()
			return resp, nil
		}

		// 3. If rejected, wait and retry
		c.recordRejected()
		retryCount++

		if retryCount >= maxRetries {
			return nil, fmt.Errorf("rate limit exceeded after %d retries", maxRetries)
		}

		c.recordRetry()
		waitDuration := time.Duration(retryAfterNanos) * time.Nanosecond

		// Wait with context cancellation support
		select {
		case <-time.After(waitDuration):
			// Continue to retry
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("unexpected: exceeded max retries")
}

// checkRateLimit calls the gRPC rate limiter service.
//
// Returns:
//   - allowed: true if request should proceed
//   - retryAfterNanos: how long to wait if rejected (nanoseconds)
//   - error: gRPC communication error
func (c *Client) checkRateLimit(ctx context.Context) (allowed bool, retryAfterNanos int64, err error) {
	req := &pb.CheckRateLimitRequest{
		Key:     c.rateLimitKey,
		Permits: c.defaultPermits,
	}

	resp, err := c.rateLimiter.CheckRateLimit(ctx, req)
	if err != nil {
		return false, 0, err
	}

	return resp.Allowed, resp.RetryAfterNanos, nil
}

// makeHTTPRequest performs the actual HTTP request to the target.
func (c *Client) makeHTTPRequest(ctx context.Context, target *targets.Target) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, target.Method, target.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Validate expected status code
	if resp.StatusCode != target.ExpectedStatus {
		// Still return response so caller can inspect it
		return resp, fmt.Errorf("unexpected status code: got %d, expected %d",
			resp.StatusCode, target.ExpectedStatus)
	}

	return resp, nil
}

// Metrics returns a snapshot of current metrics.
//
// Thread-safe: Safe to call concurrently with other operations.
//
// Returns:
//   - Metrics: Copy of current statistics
func (c *Client) Metrics() Metrics {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.metrics
}

// ResetMetrics clears all metrics counters.
//
// Useful for benchmarking or testing.
func (c *Client) ResetMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = Metrics{}
}

// recordAllowed increments the allowed counter.
func (c *Client) recordAllowed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Allowed++
}

// recordRejected increments the rejected counter.
func (c *Client) recordRejected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Rejected++
}

// recordFailed increments the failed counter.
func (c *Client) recordFailed() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.Failed++
}

// recordHTTPRequest increments the total HTTP requests counter.
func (c *Client) recordHTTPRequest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.TotalHTTPRequests++
}

// recordRetry increments the retry counter.
func (c *Client) recordRetry() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.TotalRetries++
}

// String returns a human-readable summary of metrics.
func (m Metrics) String() string {
	return fmt.Sprintf("Metrics{Allowed: %d, Rejected: %d, Failed: %d, HTTP Requests: %d, Retries: %d}",
		m.Allowed, m.Rejected, m.Failed, m.TotalHTTPRequests, m.TotalRetries)
}
