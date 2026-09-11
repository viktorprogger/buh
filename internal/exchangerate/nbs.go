package exchangerate

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const nbsBaseURL = "https://webappcenter.nbs.rs"

type NBSProvider struct {
	Client      *http.Client
	BaseURL     string
	IndexURL    string
	DownloadURL string
}

func NewNBSProvider(client *http.Client) *NBSProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &NBSProvider{Client: client, BaseURL: nbsBaseURL}
}

func (p *NBSProvider) Fetch(ctx context.Context, date time.Time, currency string) (Rate, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := ValidateCurrency(currency); err != nil {
		return Rate{}, err
	}
	base := strings.TrimRight(p.BaseURL, "/")
	indexURL := p.IndexURL
	if indexURL == "" {
		indexURL = base + "/ExchangeRateWebApp/ExchangeRate/IndexByDate"
	}
	query := url.Values{
		"Date":                   {date.Format("2.1.2006.")},
		"ExchangeRateListTypeID": {"3"},
		"isSearchExecuted":       {"true"},
	}
	indexURL += "?" + query.Encode()

	body, err := p.get(ctx, indexURL)
	if err != nil {
		return Rate{}, err
	}
	listID := extractListID(string(body))
	if listID == "" {
		return Rate{}, fmt.Errorf("exchange rate list id missing")
	}

	downloadURL := p.DownloadURL
	if downloadURL == "" {
		downloadURL = base + "/ExchangeRateWebApp/ExchangeRate/Download?ExchangeRateListID=" + url.QueryEscape(listID) + "&Format=csv"
	} else {
		separator := "?"
		if strings.Contains(downloadURL, "?") {
			separator = "&"
		}
		downloadURL += separator + "ExchangeRateListID=" + url.QueryEscape(listID) + "&Format=csv"
	}
	csvBody, err := p.get(ctx, downloadURL)
	if err != nil {
		return Rate{}, err
	}
	rate, err := ParseCSV(strings.NewReader(string(csvBody)), currency)
	if err != nil {
		return Rate{}, err
	}
	rate.RequestedDate = date
	rate.SourceURL = downloadURL
	rate.FetchedAt = time.Now()
	return rate, nil
}

func (p *NBSProvider) get(ctx context.Context, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("NBS status %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<20))
}

var listIDRE = regexp.MustCompile(`(?i)ExchangeRateListID=([0-9a-f-]{36})`)

func extractListID(s string) string {
	match := listIDRE.FindStringSubmatch(s)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

// ParseCSV parses the NBS CSV export. Its columns are list number, date,
// numeric currency code, country, ISO currency code, unit, and middle rate.
func ParseCSV(r io.Reader, currency string) (Rate, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return Rate{}, fmt.Errorf("currency is required")
	}
	reader := csv.NewReader(r)
	reader.Comma = ','
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Rate{}, err
		}
		if len(row) > 0 {
			row[0] = strings.TrimPrefix(row[0], "\ufeff")
		}
		if len(row) < 7 || strings.ToUpper(strings.TrimSpace(row[4])) != currency {
			continue
		}
		unit, err := strconv.Atoi(strings.TrimSpace(row[5]))
		if err != nil || unit <= 0 {
			return Rate{}, fmt.Errorf("invalid unit for %s", currency)
		}
		middle, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(row[6]), ",", ".", 1), 64)
		if err != nil {
			return Rate{}, fmt.Errorf("invalid middle rate for %s: %w", currency, err)
		}
		effective, err := parseDate(strings.TrimSpace(row[1]))
		if err != nil {
			return Rate{}, err
		}
		return Rate{Currency: currency, Unit: unit, Middle: middle, EffectiveDate: effective}, nil
	}
	return Rate{}, fmt.Errorf("currency %s not found", currency)
}

func parseDate(value string) (time.Time, error) {
	for _, layout := range []string{"02.01.2006.", "02.01.2006", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid effective date %q", value)
}
