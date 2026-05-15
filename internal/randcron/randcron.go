package randcron

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

// Expand converts a cron expression containing `?`
// into a standard cron expression.
//
// Supported:
//
//	?         -> random value in full field range
//	10-20?    -> random value within range
//
// Examples:
//
//	"? * * * *"
//	  -> "17 * * * *"
//
//	"10-20? * * * *"
//	  -> "14 * * * *"
//
// The generated value is deterministic for the same seed.
func Expand(expr string, seed string) (string, error) {
	fields := strings.Fields(expr)

	if len(fields) != 5 {
		return "", fmt.Errorf("cron must have 5 fields")
	}

	ranges := [5][2]int{
		{0, 59}, // minute
		{0, 23}, // hour
		{1, 31}, // day of month
		{1, 12}, // month
		{0, 6},  // day of week
	}

	for i := range fields {
		v, err := expandField(fields[i], ranges[i][0], ranges[i][1], seed, i)
		if err != nil {
			return "", err
		}
		fields[i] = v
	}

	return strings.Join(fields, " "), nil
}

func expandField(field string, min int, max int, seed string, pos int) (string, error) {
	if !strings.Contains(field, "?") {
		return field, nil
	}

	// Full random:
	// ?
	if field == "?" {
		return strconv.Itoa(randomInRange(seed, pos, min, max)), nil
	}

	// Range random:
	// 10-20?
	if strings.HasSuffix(field, "?") {
		base := strings.TrimSuffix(field, "?")

		parts := strings.Split(base, "-")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid random range: %s", field)
		}

		start, err := strconv.Atoi(parts[0])
		if err != nil {
			return "", err
		}

		end, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", err
		}

		if start > end {
			return "", fmt.Errorf("invalid range: %s", field)
		}

		return strconv.Itoa(randomInRange(seed, pos, start, end)), nil
	}

	return "", fmt.Errorf("unsupported random syntax: %s", field)
}

func randomInRange(seed string, pos int, min int, max int) int {
	h := fnv.New32a()

	_, _ = h.Write([]byte(fmt.Sprintf("%s:%d", seed, pos)))

	n := int(h.Sum32())

	return min + (n % (max - min + 1))
}
