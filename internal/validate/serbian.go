package validate

import "regexp"

var reDigits9 = regexp.MustCompile(`^\d{9}$`)
var reDigits8 = regexp.MustCompile(`^\d{8}$`)

// PIB validates a Serbian tax identification number (Порески идентификациони број).
// It must be exactly 9 digits and the last digit must pass the modulo-11 check.
func PIB(s string) bool {
	if !reDigits9.MatchString(s) {
		return false
	}
	p := 10
	for _, ch := range s[:8] {
		p = (p + int(ch-'0')) % 10
		if p == 0 {
			p = 10
		}
		p = (p * 2) % 11
	}
	return (11-p)%10 == int(s[8]-'0')
}

// MB validates a Serbian registration number (Матични број предузетника).
// It must be exactly 8 digits.
func MB(s string) bool {
	return reDigits8.MatchString(s)
}
