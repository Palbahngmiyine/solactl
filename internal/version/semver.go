package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Semver represents a parsed semantic version.
type Semver struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Raw        string
}

// ParseSemver parses a version string like "v1.2.3" or "1.2.3-rc1".
func ParseSemver(s string) (Semver, error) {
	if s == "" {
		return Semver{}, fmt.Errorf("빈 버전 문자열")
	}

	raw := s
	s = strings.TrimPrefix(s, "v")

	// Strip build metadata (e.g., +homebrew, +build.123)
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		s = s[:idx]
	}

	// Separate prerelease
	var prerelease string
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		prerelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Semver{}, fmt.Errorf("유효하지 않은 버전 형식: %s", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Semver{}, fmt.Errorf("major 버전 파싱 실패: %s", raw)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Semver{}, fmt.Errorf("minor 버전 파싱 실패: %s", raw)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Semver{}, fmt.Errorf("patch 버전 파싱 실패: %s", raw)
	}

	return Semver{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Raw:        raw,
	}, nil
}

// CompareSemver compares two Semver values.
// Returns -1 if a < b, 0 if a == b, +1 if a > b.
func CompareSemver(a, b Semver) int {
	if a.Major != b.Major {
		return cmpInt(a.Major, b.Major)
	}
	if a.Minor != b.Minor {
		return cmpInt(a.Minor, b.Minor)
	}
	if a.Patch != b.Patch {
		return cmpInt(a.Patch, b.Patch)
	}

	// Stable (no prerelease) > prerelease
	if a.Prerelease == "" && b.Prerelease == "" {
		return 0
	}
	if a.Prerelease == "" {
		return 1 // a is stable, b has prerelease
	}
	if b.Prerelease == "" {
		return -1 // b is stable, a has prerelease
	}

	// Both have prerelease — compare per SemVer spec (dot-separated identifiers)
	return comparePrereleaseIdentifiers(a.Prerelease, b.Prerelease)
}

func comparePrereleaseIdentifiers(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	limit := min(len(bParts), len(aParts))

	for i := 0; i < limit; i++ {
		aNum, aIsNum := strconv.Atoi(aParts[i])
		bNum, bIsNum := strconv.Atoi(bParts[i])

		switch {
		case aIsNum == nil && bIsNum == nil:
			// Both numeric: compare as integers
			if aNum != bNum {
				return cmpInt(aNum, bNum)
			}
		case aIsNum == nil:
			// Numeric < string
			return -1
		case bIsNum == nil:
			// String > numeric
			return 1
		default:
			// Both strings: compare lexicographically
			if aParts[i] < bParts[i] {
				return -1
			}
			if aParts[i] > bParts[i] {
				return 1
			}
		}
	}

	// Fewer fields = lower precedence
	return cmpInt(len(aParts), len(bParts))
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
