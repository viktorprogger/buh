package shared

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func PausalalLimitForYear(year int) int64 {
	if year >= 2027 {
		return 8_000_000
	}
	return 6_000_000
}

func FormatIntWithSpaces(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	offset := len(s) % 3
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (i-offset)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, s[i])
	}
	return string(out)
}

func ParseAmount(s string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	return v
}

func Round2(v float64) float64 { return math.Round(v*100) / 100 }

func FormatAmount(s string) string {
	return fmt.Sprintf("%.2f", Round2(ParseAmount(s)))
}

func SanitizeFilename(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	s = r.Replace(s)
	if len(s) > 40 {
		s = s[:40]
	}
	if s == "" {
		s = "slip"
	}
	return s
}

func ParseDayMonth(s string, year int) (time.Time, error) {
	return time.Parse("02.01.2006", strings.TrimSpace(s)+"."+strconv.Itoa(year))
}

func ParseNullDate(s string) sql.NullTime {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullTime{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func SafeIndex(ss []string, i int) string {
	if i < len(ss) {
		return ss[i]
	}
	return ""
}
