package scanner

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var durationUnitRE = regexp.MustCompile(`^(\d+)([dwm])$`)

// ParseDuration parses duration strings like "7d", "2w", "1m", "24h" for use
// with RepoResult.ModifiedSince. Supported custom units: d (days), w
// (weeks), m (months, 30 days). Bare N-unit values are matched against the
// custom format first, since Go's stdlib time.ParseDuration also accepts
// "m" as minutes and would otherwise shadow the "months" unit — e.g. "1m"
// is ambiguous between 1 minute and 1 month, and callers of this format
// mean months.
//
// Combo forms the custom format doesn't match (e.g. "1h30m", "90s") fall
// back to time.ParseDuration.
func ParseDuration(s string) (time.Duration, error) {
	if matches := durationUnitRE.FindStringSubmatch(s); matches != nil {
		value, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, fmt.Errorf("scanner: invalid duration value %q: %w", matches[1], err)
		}
		switch matches[2] {
		case "d":
			return time.Duration(value) * 24 * time.Hour, nil
		case "w":
			return time.Duration(value) * 7 * 24 * time.Hour, nil
		case "m":
			return time.Duration(value) * 30 * 24 * time.Hour, nil
		}
	}

	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	return 0, fmt.Errorf("scanner: invalid duration format %q", s)
}
