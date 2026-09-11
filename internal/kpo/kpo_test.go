package kpo

import (
	"context"
	"errors"
	"testing"
	"time"

	"buh/internal/exchangerate"
)

type pendingRateProvider struct {
	rate RateFixture
	err  error
}

type RateFixture struct {
	Unit   int
	Middle float64
}

func (p pendingRateProvider) Fetch(_ context.Context, date time.Time, currency string) (exchangerate.Rate, error) {
	if p.err != nil {
		return exchangerate.Rate{}, p.err
	}
	return exchangerate.Rate{
		RequestedDate: date,
		EffectiveDate: date,
		Currency:      currency,
		Unit:          p.rate.Unit,
		Middle:        p.rate.Middle,
		FetchedAt:     date,
	}, nil
}

func TestConvertKeepsSourceAmountsWhenRateIsUnavailable(t *testing.T) {
	service := exchangerate.NewService(
		pendingRateProvider{err: errors.New("NBS unavailable")},
		exchangerate.NewMapCache(),
	)
	repo := NewRepoWithConverter(nil, service)
	entry := Entry{
		CollectionDate:         time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		OriginalProductRevenue: 10,
		OriginalServiceRevenue: 20,
		Currency:               "HUF",
	}

	if err := repo.convert(context.Background(), &entry); err != nil {
		t.Fatal(err)
	}
	if !entry.HasPendingRate() || entry.ProductRevenue != 0 || entry.ServiceRevenue != 0 {
		t.Fatalf("pending entry = %+v", entry)
	}
	if entry.OriginalProductRevenue != 10 || entry.OriginalServiceRevenue != 20 {
		t.Fatalf("source amounts changed: %+v", entry)
	}
}

func TestConvertResolvesPendingEntryWithoutFallback(t *testing.T) {
	service := exchangerate.NewService(
		pendingRateProvider{rate: RateFixture{Unit: 100, Middle: 29.3027}},
		exchangerate.NewMapCache(),
	)
	repo := NewRepoWithConverter(nil, service)
	entry := Entry{
		CollectionDate:         time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC),
		OriginalProductRevenue: 200,
		Currency:               "HUF",
		RateStatus:             RateStatusPending,
	}

	if err := repo.convert(context.Background(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.HasPendingRate() || entry.ProductRevenue != 58.61 || entry.RateUnit != 100 {
		t.Fatalf("resolved entry = %+v", entry)
	}
	if entry.OriginalProductRevenue != 200 {
		t.Fatalf("source amount changed: %+v", entry)
	}
}
