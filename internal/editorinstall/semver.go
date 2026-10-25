package editorinstall

import (
	"fmt"
	"strconv"
	"strings"
)

type semver [3]int

func parseSemver(text string) (semver, error) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(text), "v"), ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("invalid version %q", text)
	}
	var version semver
	for position, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 {
			return semver{}, fmt.Errorf("invalid version %q", text)
		}
		version[position] = number
	}
	return version, nil
}

func (version semver) compare(other semver) int {
	for position := range version {
		if version[position] != other[position] {
			if version[position] < other[position] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// satisfies reports whether cliVersion meets every comparator of a space-separated range such as ">=0.5.0 <1.0.0".
// An empty range matches everything; a "dev" CLI build matches every range.
func satisfies(cliVersion, requirement string) (bool, error) {
	requirement = strings.TrimSpace(requirement)
	if requirement == "" || cliVersion == "dev" {
		return true, nil
	}
	current, err := parseSemver(cliVersion)
	if err != nil {
		return false, err
	}
	for _, comparator := range strings.Fields(requirement) {
		operator := strings.TrimRight(comparator, "0123456789.v")
		bound, err := parseSemver(strings.TrimPrefix(comparator, operator))
		if err != nil {
			return false, err
		}
		order := current.compare(bound)
		var met bool
		switch operator {
		case ">=":
			met = order >= 0
		case ">":
			met = order > 0
		case "<=":
			met = order <= 0
		case "<":
			met = order < 0
		case "=", "":
			met = order == 0
		default:
			return false, fmt.Errorf("unsupported comparator %q", comparator)
		}
		if !met {
			return false, nil
		}
	}
	return true, nil
}
