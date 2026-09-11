package exchangerate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeProvider struct {
	calls int
	rate  Rate
	err   error
}

func (p *fakeProvider) Fetch(_ context.Context, date time.Time, currency string) (Rate, error) {
	p.calls++
	if p.err != nil {
		return Rate{}, p.err
	}
	p.rate.RequestedDate = date
	p.rate.Currency = currency
	return p.rate, nil
}

type failingCache struct{ err error }

func (c failingCache) Get(context.Context, time.Time, string) (Rate, error) { return Rate{}, c.err }
func (c failingCache) Put(context.Context, Rate) error                      { return nil }

func TestNBSProviderFetchesArchiveCSV(t *testing.T) {
	const listID = "5eca19c1-7893-4cf2-9e51-caa08687e5c2"
	var downloadSeen bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ExchangeRateWebApp/ExchangeRate/IndexByDate":
			if got := r.URL.Query().Get("ExchangeRateListTypeID"); got != "3" {
				t.Fatalf("list type = %q, want 3", got)
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<a href="/ExchangeRateWebApp/ExchangeRate/Download?ExchangeRateListID=` + listID + `&Format=csv">CSV</a>`))
		case "/ExchangeRateWebApp/ExchangeRate/Download":
			downloadSeen = true
			if got := r.URL.Query().Get("ExchangeRateListID"); got != listID {
				t.Fatalf("list id = %q, want %q", got, listID)
			}
			if got := r.URL.Query().Get("Format"); got != "csv" {
				t.Fatalf("format = %q, want csv", got)
			}
			_, _ = w.Write([]byte("121,01.07.2025,978,EMU,EUR,1,117.1446\n121,01.07.2025,348,Hungary,HUF,100,29.3027\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewNBSProvider(server.Client())
	provider.BaseURL = server.URL
	got, err := provider.Fetch(context.Background(), time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC), "EUR")
	if err != nil {
		t.Fatal(err)
	}
	if !downloadSeen || got.Middle != 117.1446 || got.Unit != 1 || got.EffectiveDate.Format("2006-01-02") != "2025-07-01" {
		t.Fatalf("unexpected rate: %+v", got)
	}
}

func TestParseCSVPreservesCurrencyUnit(t *testing.T) {
	got, err := ParseCSV(strings.NewReader("Бр курсне листе,Датум,Валута,Назив земље,Ознака,Важи за,Средњи\n121,01.07.2025,348,Hungary,HUF,100,29.3027\n"), "HUF")
	if err != nil {
		t.Fatal(err)
	}
	if got.Unit != 100 || got.Middle != 29.3027 {
		t.Fatalf("unexpected parsed rate: %+v", got)
	}
}

func TestServiceConvertsByPublishedUnitAndCaches(t *testing.T) {
	provider := &fakeProvider{rate: Rate{Unit: 100, Middle: 29.3027, EffectiveDate: time.Now()}}
	cache := NewMapCache()
	service := NewService(provider, cache)
	date := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)

	first, err := service.Convert(context.Background(), date, "EUR", 200)
	if err != nil {
		t.Fatal(err)
	}
	if first.RSD != 58.6054 || provider.calls != 1 {
		t.Fatalf("first conversion = %+v, calls = %d", first, provider.calls)
	}
	second, err := service.Convert(context.Background(), date, "EUR", 100)
	if err != nil {
		t.Fatal(err)
	}
	if second.RSD != 29.3027 || provider.calls != 1 {
		t.Fatalf("cached conversion = %+v, calls = %d", second, provider.calls)
	}
}

func TestServiceRSDDoesNotUseProvider(t *testing.T) {
	provider := &fakeProvider{err: errors.New("network should not be called")}
	service := NewService(provider, NewMapCache())
	got, err := service.Convert(context.Background(), time.Now(), "RSD", 12.34)
	if err != nil || got.RSD != 12.34 || provider.calls != 0 {
		t.Fatalf("RSD conversion = %+v, err = %v, calls = %d", got, err, provider.calls)
	}
}

func TestCurrenciesUseRequiredOrderAndValidatePublishedCodes(t *testing.T) {
	want := []string{"RSD", "EUR", "USD", "AED", "AUD", "BAM", "BEF", "BGN", "BYN", "CAD", "CHF", "CNY", "CZK", "DEM", "DKK", "ESP", "FIM", "FRF", "GBP", "GRD", "HUF", "IEP", "INR", "ITL", "JPY", "KWD", "LUF", "MKD", "NOK", "PLN", "PTE", "RON", "RUB", "SEK", "TRY"}
	currencies := Currencies()
	if strings.Join(currencies, ",") != strings.Join(want, ",") {
		t.Fatalf("currencies = %v, want %v", currencies, want)
	}
	for i := 4; i < len(currencies); i++ {
		if currencies[i-1] > currencies[i] {
			t.Fatalf("currency tail is not sorted: %v", currencies)
		}
	}
	for _, currency := range currencies {
		if err := ValidateCurrency(strings.ToLower(currency)); err != nil {
			t.Fatalf("ValidateCurrency(%q): %v", currency, err)
		}
	}
	if err := ValidateCurrency("XXX"); !errors.Is(err, ErrUnsupportedCurrency) {
		t.Fatalf("ValidateCurrency(XXX) = %v, want unsupported currency", err)
	}
}

func TestServiceDoesNotFallbackOnCacheError(t *testing.T) {
	provider := &fakeProvider{rate: Rate{Unit: 1, Middle: 100}}
	service := NewService(provider, failingCache{err: errors.New("database unavailable")})
	_, err := service.Convert(context.Background(), time.Now(), "EUR", 1)
	if !errors.Is(err, ErrUnavailable) || provider.calls != 0 {
		t.Fatalf("err = %v, provider calls = %d", err, provider.calls)
	}
}
