// Package ips implements parsing, validation, and generation of IPS NBS QR codes
// as specified by the National Bank of Serbia.
// Format: pipe-delimited key:value fields, e.g. K:PR|V:01|C:1|R:...|N:...|I:RSD1000,00|...
package ips

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PaymentCode is the type of IPS payment (K field).
type PaymentCode string

const (
	CodePR PaymentCode = "PR" // regular payment
	CodePT PaymentCode = "PT" // point-of-sale payment
	CodePK PaymentCode = "PK" // donation
	CodeEK PaymentCode = "EK" // advance payment
)

// Payment holds all fields of an IPS NBS QR code.
// Fields follow the official NBS IPS specification (February 2023).
type Payment struct {
	K  PaymentCode // code type (PR/PT/PK/EK)
	V  string      // version — must be "01"
	C  string      // code — must be "1"
	R  string      // payee account, 18 digits (racun primaoca)
	N  string      // payee name, max 70 chars (naziv primaoca)
	I  string      // amount, e.g. "RSD1000,00" (iznos)
	SF string      // payment code, 3 digits, first digit 1 or 2 (sifra placanja)
	S  string      // payment purpose, max 35 chars (svrha placanja)
	RO string      // reference number, max 35 chars (poziv na broj)
	O  string      // payer account (racun platioca)
	P  string      // payer name, max 70 chars (naziv platioca)
	M  string      // merchant category code, 4 digits
	JS string      // one-time code, 5 digits TOTP (jednokratna sifra)
	RL string      // supplementary payee reference, max 140 chars (referenca primaoca)
	RP string      // POS transaction reference, 19 digits (referenca placanja)
}

// Amount holds a parsed currency and decimal value from the I field.
type Amount struct {
	Currency string // ISO 4217, e.g. "RSD"
	Value    string // decimal string with dot separator, e.g. "1000.00"
}

// ParseAmount parses the I field value, e.g. "RSD1000,00" → {Currency:"RSD", Value:"1000.00"}.
func ParseAmount(s string) (Amount, error) {
	if len(s) < 5 {
		return Amount{}, fmt.Errorf("amount field too short: %q", s)
	}
	currency := s[:3]
	rest := strings.ReplaceAll(s[3:], ",", ".")
	if _, err := strconv.ParseFloat(rest, 64); err != nil {
		return Amount{}, fmt.Errorf("invalid amount value in %q: %w", s, err)
	}
	return Amount{Currency: currency, Value: rest}, nil
}

// FormatAmount formats a decimal value and currency into the IPS I field format.
// value must use a dot as decimal separator (e.g. "1000.00").
func FormatAmount(currency, value string) string {
	return currency + strings.ReplaceAll(value, ".", ",")
}

var (
	reAccount = regexp.MustCompile(`^\d{18}$`)
	reAmount  = regexp.MustCompile(`^[A-Z]{3}\d+,\d{2}$`)
	reSF      = regexp.MustCompile(`^[12]\d{2}$`)
	reMCC     = regexp.MustCompile(`^\d{4}$`)
	reJS      = regexp.MustCompile(`^\d{5}$`)
	reRP      = regexp.MustCompile(`^\d{19}$`)
)

// IsIPS reports whether s looks like an IPS NBS QR code string.
func IsIPS(s string) bool {
	return strings.HasPrefix(s, "K:") && strings.Contains(s, "|V:") && strings.Contains(s, "|C:")
}

// Parse parses the pipe-delimited IPS NBS QR code string into a Payment.
// It returns an error only for malformed input (missing colon in a field).
// Use Validate to check business rules.
func Parse(s string) (*Payment, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "|")
	fields := make(map[string]string, len(parts))
	for _, part := range parts {
		idx := strings.Index(part, ":")
		if idx < 0 {
			return nil, fmt.Errorf("invalid field (no colon): %q", part)
		}
		fields[part[:idx]] = part[idx+1:]
	}
	return &Payment{
		K:  PaymentCode(fields["K"]),
		V:  fields["V"],
		C:  fields["C"],
		R:  fields["R"],
		N:  fields["N"],
		I:  fields["I"],
		SF: fields["SF"],
		S:  fields["S"],
		RO: fields["RO"],
		O:  fields["O"],
		P:  fields["P"],
		M:  fields["M"],
		JS: fields["JS"],
		RL: fields["RL"],
		RP: fields["RP"],
	}, nil
}

// ValidationError is a list of validation errors for a Payment.
type ValidationError []string

func (e ValidationError) Error() string {
	return strings.Join(e, "; ")
}

// Validate checks that the Payment conforms to the IPS NBS specification.
// Returns a ValidationError (a []string) listing all violations, or nil if valid.
func (p *Payment) Validate() error {
	var errs []string

	switch p.K {
	case CodePR, CodePT, CodePK, CodeEK:
	default:
		errs = append(errs, fmt.Sprintf("K: unknown code type %q (must be PR, PT, PK, or EK)", p.K))
	}
	if p.V != "01" {
		errs = append(errs, fmt.Sprintf("V: expected \"01\", got %q", p.V))
	}
	if p.C != "1" {
		errs = append(errs, fmt.Sprintf("C: expected \"1\", got %q", p.C))
	}
	if !reAccount.MatchString(p.R) {
		errs = append(errs, fmt.Sprintf("R: payee account must be 18 digits, got %q", p.R))
	}
	if p.N == "" {
		errs = append(errs, "N: payee name is required")
	} else if len([]rune(p.N)) > 70 {
		errs = append(errs, fmt.Sprintf("N: payee name exceeds 70 characters (%d)", len([]rune(p.N))))
	}
	if p.I == "" {
		errs = append(errs, "I: amount is required")
	} else if !reAmount.MatchString(p.I) {
		errs = append(errs, fmt.Sprintf("I: invalid amount format %q (expected e.g. RSD1000,00)", p.I))
	}
	if p.K == CodePR && p.SF == "" {
		errs = append(errs, "SF: payment code is required for PR payments")
	}
	if p.SF != "" && !reSF.MatchString(p.SF) {
		errs = append(errs, fmt.Sprintf("SF: must be 3 digits with first digit 1 or 2, got %q", p.SF))
	}
	if len([]rune(p.S)) > 35 {
		errs = append(errs, fmt.Sprintf("S: payment purpose exceeds 35 characters (%d)", len([]rune(p.S))))
	}
	if len([]rune(p.RO)) > 35 {
		errs = append(errs, fmt.Sprintf("RO: reference number exceeds 35 characters (%d)", len([]rune(p.RO))))
	}
	if p.O != "" && !reAccount.MatchString(p.O) {
		errs = append(errs, fmt.Sprintf("O: payer account must be 18 digits, got %q", p.O))
	}
	if len([]rune(p.P)) > 70 {
		errs = append(errs, fmt.Sprintf("P: payer name exceeds 70 characters (%d)", len([]rune(p.P))))
	}
	if p.M != "" && !reMCC.MatchString(p.M) {
		errs = append(errs, fmt.Sprintf("M: merchant category code must be 4 digits, got %q", p.M))
	}
	if p.JS != "" && !reJS.MatchString(p.JS) {
		errs = append(errs, fmt.Sprintf("JS: one-time code must be 5 digits, got %q", p.JS))
	}
	if len([]rune(p.RL)) > 140 {
		errs = append(errs, fmt.Sprintf("RL: payee reference exceeds 140 characters (%d)", len([]rune(p.RL))))
	}
	if p.RP != "" && !reRP.MatchString(p.RP) {
		errs = append(errs, fmt.Sprintf("RP: POS reference must be 19 digits, got %q", p.RP))
	}

	if len(errs) == 0 {
		return nil
	}
	return ValidationError(errs)
}

// String serializes the Payment back to the canonical IPS NBS QR string format.
// Fields with empty values are omitted. Field order follows the NBS specification.
func (p *Payment) String() string {
	var sb strings.Builder
	write := func(tag, val string) {
		if val == "" {
			return
		}
		if sb.Len() > 0 {
			sb.WriteByte('|')
		}
		sb.WriteString(tag)
		sb.WriteByte(':')
		sb.WriteString(val)
	}
	write("K", string(p.K))
	write("V", p.V)
	write("C", p.C)
	write("R", p.R)
	write("N", p.N)
	write("I", p.I)
	write("SF", p.SF)
	write("S", p.S)
	write("RO", p.RO)
	write("O", p.O)
	write("P", p.P)
	write("M", p.M)
	write("JS", p.JS)
	write("RL", p.RL)
	write("RP", p.RP)
	return sb.String()
}

// FieldDisplay is a single field for human-readable output.
type FieldDisplay struct {
	Tag   string
	Name  string
	Value string
}

// Fields returns all non-empty fields as a human-readable list.
func (p *Payment) Fields() []FieldDisplay {
	all := []FieldDisplay{
		{"K", "Payment type", codeDesc(p.K)},
		{"V", "Version", p.V},
		{"C", "Code", p.C},
		{"R", "Payee account", p.R},
		{"N", "Payee name", p.N},
		{"I", "Amount", fmtAmount(p.I)},
		{"SF", "Payment code (šifra plaćanja)", p.SF},
		{"S", "Payment purpose", p.S},
		{"RO", "Reference number (poziv na broj)", p.RO},
		{"O", "Payer account", p.O},
		{"P", "Payer name", p.P},
		{"M", "Merchant category code", p.M},
		{"JS", "One-time code", p.JS},
		{"RL", "Payee reference", p.RL},
		{"RP", "POS transaction reference", p.RP},
	}
	out := all[:0]
	for _, f := range all {
		if f.Value != "" {
			out = append(out, f)
		}
	}
	return out
}

func codeDesc(k PaymentCode) string {
	switch k {
	case CodePR:
		return "PR (regular payment)"
	case CodePT:
		return "PT (point-of-sale payment)"
	case CodePK:
		return "PK (donation)"
	case CodeEK:
		return "EK (advance payment)"
	default:
		return string(k)
	}
}

func fmtAmount(i string) string {
	if i == "" {
		return ""
	}
	a, err := ParseAmount(i)
	if err != nil {
		return i
	}
	return fmt.Sprintf("%s %s", a.Value, a.Currency)
}
