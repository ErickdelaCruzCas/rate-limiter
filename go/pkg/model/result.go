package model

// Decision represents the outcome of a rate limit check.
//
// A rate limiter evaluates each request and makes a binary decision:
// allow the request to proceed, or reject it due to rate limit exceeded.
//
// The Decision type provides a type-safe enumeration for these two outcomes,
// preventing magic boolean values and improving code readability.
type Decision int

const (
	// DecisionReject indicates the request should be rejected.
	//
	// The request has exceeded the rate limit. The client should not
	// process the request and should wait before retrying.
	//
	// When a request is rejected, the accompanying RateLimitResult will
	// contain a non-zero RetryAfterNanos value indicating when to retry.
	DecisionReject Decision = iota

	// DecisionAllow indicates the request should be allowed.
	//
	// The request is within the rate limit. The client can proceed
	// with processing the request.
	//
	// When a request is allowed, the accompanying RateLimitResult will
	// have RetryAfterNanos set to 0.
	DecisionAllow
)

// String returns a human-readable representation of the Decision.
//
// This method enables fmt.Printf("%v", decision) to print "ALLOW" or "REJECT"
// instead of the numeric iota values.
func (d Decision) String() string {
	switch d {
	case DecisionAllow:
		return "ALLOW"
	case DecisionReject:
		return "REJECT"
	default:
		return "UNKNOWN"
	}
}

// RateLimitResult contains the decision and retry timing information.
//
// This type is returned by all RateLimiter.TryAcquire() implementations
// and encapsulates both the allow/reject decision and, in case of rejection,
// guidance on when the client should retry.
//
// Invariant: If Decision is Allow, RetryAfterNanos must be 0.
// Invariant: If Decision is Reject, RetryAfterNanos should be > 0 (but may be 0 in edge cases).
type RateLimitResult struct {
	// Decision indicates whether the request is allowed or rejected.
	Decision Decision

	// RetryAfterNanos specifies how long to wait before retrying, in nanoseconds.
	//
	// - If Decision is Allow: always 0
	// - If Decision is Reject: nanoseconds until the rate limit may allow the request
	//
	// The retry-after value is algorithm-specific:
	// - Token Bucket: time until enough tokens refill
	// - Fixed Window: time until next window starts
	// - Sliding Window Log: time until oldest event expires
	// - Sliding Window Counter: time until next bucket rolls over
	//
	// Clients should wait at least this duration before retrying to avoid
	// repeated rejections. Note that waiting exactly RetryAfterNanos does not
	// guarantee the next request will be allowed (other requests may consume
	// capacity in the meantime).
	RetryAfterNanos int64
}

// Allow creates an ALLOW result with zero retry time.
//
// This factory function ensures the invariant that allowed requests
// always have RetryAfterNanos = 0.
//
// Example usage:
//
//	if bucket.tokens >= permits {
//	    bucket.tokens -= permits
//	    return model.Allow(), nil
//	}
func Allow() RateLimitResult {
	return RateLimitResult{
		Decision:        DecisionAllow,
		RetryAfterNanos: 0,
	}
}

// Reject creates a REJECT result with the specified retry time.
//
// This factory function ensures RetryAfterNanos is never negative,
// clamping negative values to 0. Negative retry times can occur in
// algorithm implementations due to clock precision or edge cases,
// but should never be exposed to callers.
//
// Parameters:
//   - retryAfterNanos: Nanoseconds to wait before retrying (will be clamped to >= 0)
//
// Example usage:
//
//	missing := permits - bucket.tokens
//	retryAfter := int64(math.Ceil(missing / bucket.refillPerNanos))
//	return model.Reject(retryAfter), nil
func Reject(retryAfterNanos int64) RateLimitResult {
	// Clamp to 0 if negative (defensive programming)
	// Negative values should not occur in correct implementations,
	// but this provides a safety net against clock skew or edge cases.
	if retryAfterNanos < 0 {
		retryAfterNanos = 0
	}
	return RateLimitResult{
		Decision:        DecisionReject,
		RetryAfterNanos: retryAfterNanos,
	}
}
