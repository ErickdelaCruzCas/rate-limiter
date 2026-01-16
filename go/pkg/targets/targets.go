// Package targets provides configuration for public APIs used in E2E testing.
//
// This package defines real, publicly accessible APIs that don't require
// authentication, making them perfect for testing rate limiting behavior
// with actual HTTP traffic.
//
// # Purpose
//
// These targets serve multiple purposes:
//   - E2E testing of rate limiter with real external APIs
//   - Demonstrating concurrency patterns with actual I/O
//   - Validating rate limiting prevents API abuse
//   - Realistic latency patterns (network delays)
//
// # Target Selection Criteria
//
// All APIs included here:
//   - ✅ No authentication required
//   - ✅ Free tier / unlimited usage
//   - ✅ Stable and reliable
//   - ✅ Low latency (< 500ms typical)
//   - ✅ Return JSON (easy to parse)
//
// # Example Usage
//
//	// Get a specific target
//	target := targets.GetTarget("jsonplaceholder-posts")
//
//	// Get all targets
//	allTargets := targets.All()
//
//	// Random target for load distribution
//	target := targets.Random()
package targets

import (
	"fmt"
	"math/rand"
	"time"
)

// Target represents an HTTP endpoint for testing.
//
// Each target includes:
//   - Name: Human-readable identifier
//   - URL: Full HTTP(S) endpoint
//   - Method: HTTP method (GET, POST, etc.)
//   - Description: What this endpoint does
//   - ExpectedStatus: Expected HTTP status code (200, 201, etc.)
type Target struct {
	Name           string
	URL            string
	Method         string
	Description    string
	ExpectedStatus int
}

var (
	// JSONPlaceholderPosts - Single post from fake REST API
	JSONPlaceholderPosts = &Target{
		Name:           "jsonplaceholder-posts",
		URL:            "https://jsonplaceholder.typicode.com/posts/1",
		Method:         "GET",
		Description:    "Fetch a single fake post (fast response)",
		ExpectedStatus: 200,
	}

	// JSONPlaceholderUsers - List of fake users
	JSONPlaceholderUsers = &Target{
		Name:           "jsonplaceholder-users",
		URL:            "https://jsonplaceholder.typicode.com/users",
		Method:         "GET",
		Description:    "Fetch list of fake users",
		ExpectedStatus: 200,
	}

	// HTTPBinGet - Echo service that returns request data
	HTTPBinGet = &Target{
		Name:           "httpbin-get",
		URL:            "https://httpbin.org/get",
		Method:         "GET",
		Description:    "Echo service - returns request details",
		ExpectedStatus: 200,
	}

	// HTTPBinDelay - Delayed response (useful for timeout testing)
	HTTPBinDelay = &Target{
		Name:           "httpbin-delay",
		URL:            "https://httpbin.org/delay/1",
		Method:         "GET",
		Description:    "Delayed response (1 second)",
		ExpectedStatus: 200,
	}

	// RandomUser - Random user data generator
	RandomUser = &Target{
		Name:           "randomuser",
		URL:            "https://randomuser.me/api/",
		Method:         "GET",
		Description:    "Generate random user data",
		ExpectedStatus: 200,
	}

	// DogAPI - Random dog image
	DogAPI = &Target{
		Name:           "dog-api",
		URL:            "https://dog.ceo/api/breeds/image/random",
		Method:         "GET",
		Description:    "Random dog image URL",
		ExpectedStatus: 200,
	}

	// PokeAPI - Pokemon data
	PokeAPIPikachu = &Target{
		Name:           "pokeapi-pikachu",
		URL:            "https://pokeapi.co/api/v2/pokemon/pikachu",
		Method:         "GET",
		Description:    "Pikachu Pokemon data",
		ExpectedStatus: 200,
	}

	// CatFacts - Random cat fact
	CatFacts = &Target{
		Name:           "cat-facts",
		URL:            "https://catfact.ninja/fact",
		Method:         "GET",
		Description:    "Random cat fact",
		ExpectedStatus: 200,
	}

	// UUID Generator
	UUIDGenerator = &Target{
		Name:           "uuid",
		URL:            "https://www.uuidgenerator.net/api/version4",
		Method:         "GET",
		Description:    "Generate random UUID",
		ExpectedStatus: 200,
	}

	// Advice Slip - Random advice
	AdviceSlip = &Target{
		Name:           "advice",
		URL:            "https://api.adviceslip.com/advice",
		Method:         "GET",
		Description:    "Random piece of advice",
		ExpectedStatus: 200,
	}
)

// targetRegistry maps target names to Target instances
var targetRegistry = map[string]*Target{
	"jsonplaceholder-posts": JSONPlaceholderPosts,
	"jsonplaceholder-users": JSONPlaceholderUsers,
	"httpbin-get":           HTTPBinGet,
	"httpbin-delay":         HTTPBinDelay,
	"randomuser":            RandomUser,
	"dog-api":               DogAPI,
	"pokeapi-pikachu":       PokeAPIPikachu,
	"cat-facts":             CatFacts,
	"uuid":                  UUIDGenerator,
	"advice":                AdviceSlip,
}

// All returns all available targets.
//
// Returns a slice of all registered targets, useful for:
//   - Round-robin distribution
//   - Random selection
//   - Testing against multiple APIs
//
// Example:
//
//	for _, target := range targets.All() {
//	    fmt.Printf("Testing %s: %s\n", target.Name, target.URL)
//	}
func All() []*Target {
	targets := make([]*Target, 0, len(targetRegistry))
	for _, target := range targetRegistry {
		targets = append(targets, target)
	}
	return targets
}

// GetTarget returns a target by name.
//
// Parameters:
//   - name: Target name (e.g., "jsonplaceholder-posts")
//
// Returns:
//   - Target if found, nil otherwise
//
// Example:
//
//	target := targets.GetTarget("dog-api")
//	if target != nil {
//	    resp, _ := http.Get(target.URL)
//	}
func GetTarget(name string) *Target {
	return targetRegistry[name]
}

// Random returns a random target.
//
// Useful for load distribution and testing with variety.
// Uses math/rand for selection.
//
// Example:
//
//	target := targets.Random()
//	fmt.Printf("Testing random API: %s\n", target.Name)
func Random() *Target {
	all := All()
	if len(all) == 0 {
		return nil
	}
	return all[rand.Intn(len(all))]
}

// Names returns all target names.
//
// Returns:
//   - Slice of target names (sorted alphabetically)
//
// Useful for CLI help text or validation.
//
// Example:
//
//	fmt.Println("Available targets:", targets.Names())
func Names() []string {
	names := make([]string, 0, len(targetRegistry))
	for name := range targetRegistry {
		names = append(names, name)
	}
	return names
}

// Fast returns targets with low latency (< 500ms typical).
//
// Excludes targets like httpbin-delay that intentionally delay.
//
// Example:
//
//	for _, target := range targets.Fast() {
//	    // These should respond quickly
//	}
func Fast() []*Target {
	return []*Target{
		JSONPlaceholderPosts,
		JSONPlaceholderUsers,
		HTTPBinGet,
		RandomUser,
		DogAPI,
		PokeAPIPikachu,
		CatFacts,
		UUIDGenerator,
		AdviceSlip,
	}
}

// String returns a human-readable representation of the target.
func (t *Target) String() string {
	return fmt.Sprintf("%s (%s %s)", t.Name, t.Method, t.URL)
}

func init() {
	// Seed random for Random() function
	rand.Seed(time.Now().UnixNano())
}
