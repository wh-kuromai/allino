package allino

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var tryLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006/01/02 15:04:05",
	"02 Jan 2006 15:04:05 MST",
	time.RFC1123Z,
	time.RFC1123,
	time.RFC850,
	time.ANSIC,
	time.UnixDate,
	time.RubyDate,
}

// 10=sec, 13=ms, 16=µs, 19=ns
func parseEpochDigits(s string) (time.Time, bool) {
	neg := false
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	if len(s) < 10 || len(s) > 19 {
		return time.Time{}, false
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	if neg {
		v = -v
	}

	switch len(s) {
	case 10:
		return time.Unix(v, 0).UTC(), true
	case 13:
		return time.Unix(v/1_000, (v%1_000)*1_000_000).UTC(), true
	case 16:
		return time.Unix(v/1_000_000, (v%1_000_000)*1_000).UTC(), true
	case 19:
		return time.Unix(0, v).UTC(), true
	default:
		return time.Time{}, false
	}
}

func parseTimeSafe(s string, defaultLoc *time.Location) (time.Time, error) {
	in := strings.TrimSpace(s)

	// 1) 純数字なら epoch 判定
	if in != "" && strings.IndexFunc(in, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		if t, ok := parseEpochDigits(in); ok {
			return t, nil
		}
	}

	// 2) time.Parse を順に試す
	for _, layout := range tryLayouts {
		if t, err := time.Parse(layout, in); err == nil {
			return t, nil
		}
		// defaultLoc を使いたいレイアウト用（Zなしをローカルとして解釈）
		if defaultLoc != nil {
			if t, err := time.ParseInLocation(layout, in, defaultLoc); err == nil {
				return t, nil
			}
		}
	}

	return time.Time{}, errors.New("parse failed")
}
