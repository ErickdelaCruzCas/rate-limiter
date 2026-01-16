package targets

import "testing"

// TestAll verifies All() returns all registered targets.
func TestAll(t *testing.T) {
	all := All()
	if len(all) != len(targetRegistry) {
		t.Errorf("All() returned %d targets, expected %d", len(all), len(targetRegistry))
	}
}

// TestGetTarget verifies GetTarget() retrieves correct targets.
func TestGetTarget(t *testing.T) {
	tests := []struct {
		name     string
		expected *Target
	}{
		{"jsonplaceholder-posts", JSONPlaceholderPosts},
		{"httpbin-get", HTTPBinGet},
		{"dog-api", DogAPI},
		{"uuid", UUIDGenerator},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := GetTarget(tt.name)
			if target != tt.expected {
				t.Errorf("GetTarget(%s) returned %v, expected %v", tt.name, target, tt.expected)
			}
		})
	}
}

// TestGetTarget_NotFound verifies GetTarget returns nil for unknown names.
func TestGetTarget_NotFound(t *testing.T) {
	target := GetTarget("nonexistent")
	if target != nil {
		t.Errorf("GetTarget(nonexistent) should return nil, got %v", target)
	}
}

// TestRandom verifies Random() returns a valid target.
func TestRandom(t *testing.T) {
	target := Random()
	if target == nil {
		t.Error("Random() returned nil")
	}

	// Verify it's a registered target
	found := false
	for _, registered := range targetRegistry {
		if target == registered {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Random() returned unregistered target: %v", target)
	}
}

// TestNames verifies Names() returns all target names.
func TestNames(t *testing.T) {
	names := Names()
	if len(names) != len(targetRegistry) {
		t.Errorf("Names() returned %d names, expected %d", len(names), len(targetRegistry))
	}
}

// TestFast verifies Fast() excludes delayed targets.
func TestFast(t *testing.T) {
	fast := Fast()

	// Verify HTTPBinDelay is NOT included
	for _, target := range fast {
		if target == HTTPBinDelay {
			t.Error("Fast() should not include HTTPBinDelay")
		}
	}

	// Verify at least some targets are included
	if len(fast) == 0 {
		t.Error("Fast() returned empty slice")
	}
}

// TestTarget_String verifies String() method.
func TestTarget_String(t *testing.T) {
	target := JSONPlaceholderPosts
	str := target.String()

	// Should contain name, method, and URL
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Basic validation (contains key components)
	expected := "jsonplaceholder-posts (GET https://jsonplaceholder.typicode.com/posts/1)"
	if str != expected {
		t.Errorf("String() = %q, expected %q", str, expected)
	}
}

// TestTargetRegistry_Complete verifies all exported targets are registered.
func TestTargetRegistry_Complete(t *testing.T) {
	expectedTargets := []*Target{
		JSONPlaceholderPosts,
		JSONPlaceholderUsers,
		HTTPBinGet,
		HTTPBinDelay,
		RandomUser,
		DogAPI,
		PokeAPIPikachu,
		CatFacts,
		UUIDGenerator,
		AdviceSlip,
	}

	for _, expected := range expectedTargets {
		registered := GetTarget(expected.Name)
		if registered != expected {
			t.Errorf("target %s not properly registered", expected.Name)
		}
	}
}
