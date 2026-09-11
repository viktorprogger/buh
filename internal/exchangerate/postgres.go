package exchangerate

import (
	"context"
	"database/sql"
	"time"
)

type PostgresCache struct{ DB *sql.DB }

func (c *PostgresCache) Get(ctx context.Context, date time.Time, currency string) (Rate, error) {
	var rate Rate
	err := c.DB.QueryRowContext(ctx, `
		SELECT requested_date, effective_date, currency, unit, middle_rate, fetched_at, source_url
		FROM exchange_rates
		WHERE requested_date = $1 AND currency = $2`, date, currency).
		Scan(&rate.RequestedDate, &rate.EffectiveDate, &rate.Currency, &rate.Unit, &rate.Middle, &rate.FetchedAt, &rate.SourceURL)
	if err == sql.ErrNoRows {
		return Rate{}, ErrCacheMiss
	}
	return rate, err
}

func (c *PostgresCache) Put(ctx context.Context, rate Rate) error {
	_, err := c.DB.ExecContext(ctx, `
		INSERT INTO exchange_rates (requested_date, effective_date, currency, unit, middle_rate, fetched_at, source_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (requested_date, currency) DO NOTHING`,
		rate.RequestedDate, rate.EffectiveDate, rate.Currency, rate.Unit, rate.Middle,
		rate.FetchedAt, rate.SourceURL,
	)
	return err
}
