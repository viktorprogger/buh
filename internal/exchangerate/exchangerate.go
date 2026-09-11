// Package exchangerate provides historical currency conversion with a persistent cache.
package exchangerate

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrUnavailable         = errors.New("exchange rate unavailable")
	ErrCacheMiss           = errors.New("exchange rate not cached")
	ErrUnsupportedCurrency = errors.New("unsupported currency")
)

// NBS publishes these ISO 4217 codes in its daily middle-rate CSV. Keep the
// list deterministic: the three most commonly used codes are first, followed
// by the remaining published codes alphabetically.
var currencyCodes = []string{"RSD", "EUR", "USD", "AED", "AUD", "BAM", "BEF", "BGN", "BYN", "CAD", "CHF", "CNY", "CZK", "DEM", "DKK", "ESP", "FIM", "FRF", "GBP", "GRD", "HUF", "IEP", "INR", "ITL", "JPY", "KWD", "LUF", "MKD", "NOK", "PLN", "PTE", "RON", "RUB", "SEK", "TRY"}

var supported = func() map[string]struct{} {
	m := make(map[string]struct{}, len(currencyCodes))
	for _, code := range currencyCodes {
		m[code] = struct{}{}
	}
	return m
}()

func Currencies() []string { return append([]string(nil), currencyCodes...) }

func ValidateCurrency(currency string) error {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if _, ok := supported[currency]; !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedCurrency, currency)
	}
	return nil
}

type Rate struct {
	RequestedDate time.Time
	EffectiveDate time.Time
	Currency      string
	Unit          int
	Middle        float64
	FetchedAt     time.Time
	SourceURL     string
}

type Provider interface {
	Fetch(context.Context, time.Time, string) (Rate, error)
}

type Cache interface {
	Get(context.Context, time.Time, string) (Rate, error)
	Put(context.Context, Rate) error
}

type Conversion struct {
	Amount float64
	RSD    float64
	Rate   Rate
}

type Service struct {
	provider Provider
	cache    Cache
}

func NewService(provider Provider, cache Cache) *Service {
	return &Service{provider: provider, cache: cache}
}

func (s *Service) Convert(ctx context.Context, date time.Time, currency string, amount float64) (Conversion, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if err := ValidateCurrency(currency); err != nil {
		return Conversion{}, err
	}
	if currency == "RSD" {
		rate := Rate{RequestedDate: date, EffectiveDate: date, Currency: currency, Unit: 1, Middle: 1}
		return Conversion{Amount: amount, RSD: amount, Rate: rate}, nil
	}
	if s == nil || s.cache == nil || s.provider == nil {
		return Conversion{}, fmt.Errorf("%w: %s", ErrUnavailable, currency)
	}

	rate, err := s.cache.Get(ctx, date, currency)
	if err != nil {
		if !errors.Is(err, ErrCacheMiss) {
			return Conversion{}, fmt.Errorf("%w: cache: %v", ErrUnavailable, err)
		}
		rate, err = s.provider.Fetch(ctx, date, currency)
		if err != nil {
			return Conversion{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		if err := s.cache.Put(ctx, rate); err != nil {
			return Conversion{}, fmt.Errorf("%w: cache: %v", ErrUnavailable, err)
		}
	}
	if rate.Unit <= 0 || rate.Middle <= 0 || math.IsNaN(rate.Middle) || math.IsInf(rate.Middle, 0) {
		return Conversion{}, fmt.Errorf("%w: invalid %s rate", ErrUnavailable, currency)
	}
	return Conversion{Amount: amount, RSD: amount * rate.Middle / float64(rate.Unit), Rate: rate}, nil
}

// MapCache is a deterministic in-memory cache useful for tests and small callers.
type MapCache struct{ rates map[string]Rate }

func NewMapCache() *MapCache { return &MapCache{rates: make(map[string]Rate)} }

func (c *MapCache) Get(_ context.Context, date time.Time, currency string) (Rate, error) {
	rate, ok := c.rates[date.Format("2006-01-02")+":"+currency]
	if !ok {
		return Rate{}, ErrCacheMiss
	}
	return rate, nil
}

func (c *MapCache) Put(_ context.Context, rate Rate) error {
	if c.rates == nil {
		c.rates = make(map[string]Rate)
	}
	c.rates[rate.RequestedDate.Format("2006-01-02")+":"+rate.Currency] = rate
	return nil
}
