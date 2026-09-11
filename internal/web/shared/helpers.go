package shared

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func PausalalLimitForYear(_ int) int64 {
	return 6_000_000
}

// VATLimitForDate returns the VAT registration threshold for the given date.
// Measured over a rolling 365/366-day window.
func VATLimitForDate(_ time.Time) int64 {
	return 8_000_000
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

// Slug converts user-visible text into a portable lowercase filename component.
func Slug(s string) string {
	s = strings.NewReplacer("ђ", "dj", "Ђ", "Dj", "đ", "dj", "Đ", "Dj", "џ", "dz", "Џ", "Dz").Replace(s)
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(s) {
		r = transliterate(r)
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			separator = false
		} else if unicode.IsLetter(r) || unicode.IsNumber(r) {
			// Non-Latin letters are not portable filename components.
			separator = true
		} else if b.Len() > 0 {
			separator = true
		}
		if separator && b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func transliterate(r rune) rune {
	for _, pair := range []struct{ from, to rune }{{'š', 's'}, {'č', 'c'}, {'ć', 'c'}, {'ž', 'z'}, {'љ', 'l'}, {'њ', 'n'}, {'џ', 'd'}, {'ђ', 'd'}, {'ј', 'j'}, {'ћ', 'c'}, {'ч', 'c'}, {'ш', 's'}, {'ж', 'z'}, {'а', 'a'}, {'б', 'b'}, {'в', 'v'}, {'г', 'g'}, {'д', 'd'}, {'е', 'e'}, {'з', 'z'}, {'и', 'i'}, {'к', 'k'}, {'л', 'l'}, {'м', 'm'}, {'н', 'n'}, {'о', 'o'}, {'п', 'p'}, {'р', 'r'}, {'с', 's'}, {'т', 't'}, {'у', 'u'}, {'ф', 'f'}, {'х', 'h'}, {'ц', 'c'}} {
		if r == pair.from {
			return pair.to
		}
	}
	return r
}

func SanitizeFilename(s string) string {
	if slug := Slug(s); slug != "" {
		return slug
	}
	return "slip"
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
