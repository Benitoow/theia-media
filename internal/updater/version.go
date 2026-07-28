package updater

import (
	"regexp"
	"strconv"
	"strings"
)

// semverPattern matches the versions Theia releases under. The leading "v" is
// optional because git tags carry it and ldflags values often do not.
var semverPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?$`)

type version struct {
	major, minor, patch int

	// prerelease is "beta.1" in 1.0.0-beta.1. An empty value outranks any
	// prerelease, per semver: 1.0.0 is newer than 1.0.0-rc.1.
	prerelease string
}

// parseVersion reads a version string, reporting whether it is one that can be
// compared at all.
//
// Anything unparseable -- "dev", a bare commit hash, an empty string -- returns
// false, and the caller must then refuse to update. A development build that
// cannot say what it is has no business replacing itself with something it
// cannot compare against.
func parseVersion(s string) (version, bool) {
	m := semverPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return version{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return version{major: major, minor: minor, patch: patch, prerelease: m[4]}, true
}

// compare returns -1, 0 or 1 as a sorts before, equal to, or after b.
func compare(a, b version) int {
	switch {
	case a.major != b.major:
		return sign(a.major - b.major)
	case a.minor != b.minor:
		return sign(a.minor - b.minor)
	case a.patch != b.patch:
		return sign(a.patch - b.patch)
	}

	// Equal numbers: a release outranks any prerelease of the same number.
	switch {
	case a.prerelease == "" && b.prerelease == "":
		return 0
	case a.prerelease == "":
		return 1
	case b.prerelease == "":
		return -1
	}
	return comparePrerelease(a.prerelease, b.prerelease)
}

// comparePrerelease orders dot-separated identifiers: numeric ones compare
// numerically, everything else lexically, and numeric ranks below alphabetic.
func comparePrerelease(a, b string) int {
	aParts, bParts := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] == bParts[i] {
			continue
		}
		aNum, aErr := strconv.Atoi(aParts[i])
		bNum, bErr := strconv.Atoi(bParts[i])
		aNumeric, bNumeric := aErr == nil, bErr == nil
		switch {
		case aNumeric && bNumeric:
			return sign(aNum - bNum)
		case aNumeric:
			return -1 // numeric identifiers rank below alphanumeric ones
		case bNumeric:
			return 1
		default:
			return strings.Compare(aParts[i], bParts[i])
		}
	}
	return sign(len(aParts) - len(bParts))
}

// isNewer reports whether latest is a version worth updating to, and whether
// the comparison could be made at all.
func isNewer(latest, current string) (newer, comparable bool) {
	l, ok := parseVersion(latest)
	if !ok {
		return false, false
	}
	c, ok := parseVersion(current)
	if !ok {
		return false, false
	}
	return compare(l, c) > 0, true
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
