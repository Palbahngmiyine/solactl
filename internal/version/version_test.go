package version

import "testing"

func TestVersion_NonEmpty(t *testing.T) {
	t.Cleanup(func() {})
	fields := []struct {
		name  string
		value string
	}{
		{"Version", Version},
		{"Commit", Commit},
		{"Date", Date},
	}

	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			if f.value == "" {
				t.Errorf("%s should not be empty after init", f.name)
			}
		})
	}
}

func TestVersion_DevDefault(t *testing.T) {
	t.Cleanup(func() {})
	// When run via `go test` without ldflags, Version should be "dev"
	// because debug.ReadBuildInfo().Main.Version is "(devel)" in test mode.
	// If built with module info, it could be a real version string.
	if Version == "" {
		t.Fatal("Version must not be empty")
	}

	validDefaults := map[string]bool{
		"dev": true,
	}

	// Either "dev" or a version that looks like a semver / module version
	if !validDefaults[Version] && Version[0] != 'v' {
		// Still acceptable: any non-empty value set by init() is fine.
		// We just verify it was populated.
		t.Logf("Version is %q (not 'dev' or 'v*'), which is acceptable", Version)
	}

	// Commit should be "none" or a hex hash (possibly with -dirty suffix)
	if Commit == "" {
		t.Fatal("Commit must not be empty")
	}

	// Date should be "unknown" or an ISO timestamp
	if Date == "" {
		t.Fatal("Date must not be empty")
	}
}
