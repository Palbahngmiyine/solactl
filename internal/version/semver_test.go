package version

import "testing"

func TestParseSemver_Valid(t *testing.T) {
	tests := []struct {
		input      string
		major      int
		minor      int
		patch      int
		prerelease string
	}{
		{"v1.2.3", 1, 2, 3, ""},
		{"1.0.0", 1, 0, 0, ""},
		{"v0.0.1", 0, 0, 1, ""},
		{"v2.1.0-rc1", 2, 1, 0, "rc1"},
		{"v10.20.30-beta.2", 10, 20, 30, "beta.2"},
		{"v1.2.3+homebrew", 1, 2, 3, ""},
		{"v1.0.0-rc1+build.123", 1, 0, 0, "rc1"},
		{"0.0.0", 0, 0, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			sv, err := ParseSemver(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sv.Major != tt.major {
				t.Errorf("major: got %d, want %d", sv.Major, tt.major)
			}
			if sv.Minor != tt.minor {
				t.Errorf("minor: got %d, want %d", sv.Minor, tt.minor)
			}
			if sv.Patch != tt.patch {
				t.Errorf("patch: got %d, want %d", sv.Patch, tt.patch)
			}
			if sv.Prerelease != tt.prerelease {
				t.Errorf("prerelease: got %q, want %q", sv.Prerelease, tt.prerelease)
			}
			if sv.Raw != tt.input {
				t.Errorf("raw: got %q, want %q", sv.Raw, tt.input)
			}
		})
	}
}

func TestParseSemver_Invalid(t *testing.T) {
	tests := []string{
		"",
		"vx.y.z",
		"v1.2",
		"v1.2.3.4",
		"abc",
		"v1.2.x",
		"...",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := ParseSemver(input)
			if err == nil {
				t.Fatalf("expected error for %q", input)
			}
		})
	}
}

func TestCompareSemver_MajorDifference(t *testing.T) {
	a := Semver{Major: 2, Minor: 0, Patch: 0}
	b := Semver{Major: 1, Minor: 9, Patch: 9}
	if got := CompareSemver(a, b); got != 1 {
		t.Errorf("2.0.0 vs 1.9.9: got %d, want 1", got)
	}
	if got := CompareSemver(b, a); got != -1 {
		t.Errorf("1.9.9 vs 2.0.0: got %d, want -1", got)
	}
}

func TestCompareSemver_MinorDifference(t *testing.T) {
	a := Semver{Major: 1, Minor: 3, Patch: 0}
	b := Semver{Major: 1, Minor: 2, Patch: 9}
	if got := CompareSemver(a, b); got != 1 {
		t.Errorf("1.3.0 vs 1.2.9: got %d, want 1", got)
	}
}

func TestCompareSemver_PatchDifference(t *testing.T) {
	a := Semver{Major: 1, Minor: 2, Patch: 4}
	b := Semver{Major: 1, Minor: 2, Patch: 3}
	if got := CompareSemver(a, b); got != 1 {
		t.Errorf("1.2.4 vs 1.2.3: got %d, want 1", got)
	}
}

func TestCompareSemver_Equal(t *testing.T) {
	a := Semver{Major: 1, Minor: 2, Patch: 3}
	b := Semver{Major: 1, Minor: 2, Patch: 3}
	if got := CompareSemver(a, b); got != 0 {
		t.Errorf("1.2.3 vs 1.2.3: got %d, want 0", got)
	}
}

func TestCompareSemver_PrereleaseVsStable(t *testing.T) {
	stable := Semver{Major: 1, Minor: 2, Patch: 3}
	rc := Semver{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc1"}

	if got := CompareSemver(stable, rc); got != 1 {
		t.Errorf("stable vs rc: got %d, want 1", got)
	}
	if got := CompareSemver(rc, stable); got != -1 {
		t.Errorf("rc vs stable: got %d, want -1", got)
	}
}

func TestCompareSemver_PrereleaseOrder(t *testing.T) {
	tests := []struct {
		name string
		a, b Semver
		want int
	}{
		{
			name: "rc2 > rc1 (string comparison)",
			a:    Semver{Major: 1, Prerelease: "rc2"},
			b:    Semver{Major: 1, Prerelease: "rc1"},
			want: 1,
		},
		{
			name: "rc.10 > rc.2 (numeric dot-separated)",
			a:    Semver{Major: 1, Prerelease: "rc.10"},
			b:    Semver{Major: 1, Prerelease: "rc.2"},
			want: 1,
		},
		{
			name: "beta.11 > beta.2 (numeric dot-separated)",
			a:    Semver{Major: 1, Prerelease: "beta.11"},
			b:    Semver{Major: 1, Prerelease: "beta.2"},
			want: 1,
		},
		{
			name: "alpha < beta (string comparison)",
			a:    Semver{Major: 1, Prerelease: "alpha"},
			b:    Semver{Major: 1, Prerelease: "beta"},
			want: -1,
		},
		{
			name: "1.0.0-alpha < 1.0.0-alpha.1 (fewer fields)",
			a:    Semver{Major: 1, Prerelease: "alpha"},
			b:    Semver{Major: 1, Prerelease: "alpha.1"},
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareSemver(tt.a, tt.b); got != tt.want {
				t.Errorf("%s: got %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestCompareSemver_PrereleaseEqual(t *testing.T) {
	a := Semver{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc1"}
	b := Semver{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc1"}
	if got := CompareSemver(a, b); got != 0 {
		t.Errorf("identical prerelease: got %d, want 0", got)
	}
}

func TestCompareSemver_CurrentGreaterThanLatest(t *testing.T) {
	current := Semver{Major: 2, Minor: 0, Patch: 0}
	latest := Semver{Major: 1, Minor: 5, Patch: 0}

	if got := CompareSemver(current, latest); got != 1 {
		t.Errorf("current > latest: got %d, want 1", got)
	}
}

func FuzzParseSemver(f *testing.F) {
	f.Add("v1.2.3")
	f.Add("1.0.0-rc1")
	f.Add("")
	f.Add("abc")
	f.Add("v0.0.0")
	f.Add("v999.999.999-alpha.123")

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic
		_, _ = ParseSemver(input)
	})
}
