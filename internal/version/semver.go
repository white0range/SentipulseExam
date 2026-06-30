package version

import (
	"fmt"
	"strconv"
	"strings"
)

type SemanticVersion struct {
	Major int
	Minor int
	Patch int
}

func Parse(raw string) (SemanticVersion, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	mainPart := strings.SplitN(raw, "-", 2)[0]
	mainPart = strings.SplitN(mainPart, "+", 2)[0]

	parts := strings.Split(mainPart, ".")
	if len(parts) != 3 {
		return SemanticVersion{}, fmt.Errorf("version %q must be in major.minor.patch format", raw)
	}

	values := [3]int{}
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return SemanticVersion{}, fmt.Errorf("version %q has invalid numeric part %q", raw, part)
		}
		values[index] = number
	}

	return SemanticVersion{
		Major: values[0],
		Minor: values[1],
		Patch: values[2],
	}, nil
}

func (v SemanticVersion) Compare(other SemanticVersion) int {
	switch {
	case v.Major != other.Major:
		return compareInt(v.Major, other.Major)
	case v.Minor != other.Minor:
		return compareInt(v.Minor, other.Minor)
	default:
		return compareInt(v.Patch, other.Patch)
	}
}

func Satisfies(version, constraints string) (bool, error) {
	parsedVersion, err := Parse(version)
	if err != nil {
		return false, err
	}

	for _, rawConstraint := range strings.Split(constraints, ",") {
		constraint := strings.TrimSpace(rawConstraint)
		if constraint == "" {
			continue
		}

		operator, targetRaw, err := splitConstraint(constraint)
		if err != nil {
			return false, err
		}

		target, err := Parse(targetRaw)
		if err != nil {
			return false, err
		}

		comparison := parsedVersion.Compare(target)
		if !matchesConstraint(comparison, operator) {
			return false, nil
		}
	}

	return true, nil
}

func splitConstraint(constraint string) (string, string, error) {
	operators := []string{">=", "<=", ">", "<", "="}
	for _, operator := range operators {
		if strings.HasPrefix(constraint, operator) {
			return operator, strings.TrimSpace(strings.TrimPrefix(constraint, operator)), nil
		}
	}
	return "", "", fmt.Errorf("unsupported constraint %q", constraint)
}

func matchesConstraint(comparison int, operator string) bool {
	switch operator {
	case ">":
		return comparison > 0
	case ">=":
		return comparison >= 0
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case "=":
		return comparison == 0
	default:
		return false
	}
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
