// Package storage holds the Postgres and Redis adapters. Both expose narrow
// interfaces that the api/matching packages depend on, so handlers and
// engines can be unit-tested with fakes.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/goncharovart/fan-selector-api/internal/matching"
)

// Store wraps a pgx connection pool and exposes the queries the API needs.
// Construct it with New; close it with Close.
type Store struct {
	pool *pgxpool.Pool
}

// New parses dsn, applies sane pool sizing, opens connections eagerly and
// pings once before returning. A nil *Store is never returned alongside a
// nil error.
func New(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("storage: empty DATABASE_URL")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: parse dsn: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("storage: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("storage: ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool. Safe to call on a nil receiver.
func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

// Ping is used by /readyz.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return errors.New("storage: not initialized")
	}
	return s.pool.Ping(ctx)
}

// Candidates returns up to maxN fans whose declared envelope brackets qTarget.
// The range comparison is done with numrange containment so it can use the
// GiST index; results are stable-ordered by id to keep tests deterministic.
func (s *Store) Candidates(ctx context.Context, qTarget float64, maxN int) ([]matching.FanCandidate, error) {
	const query = `
		SELECT m.id,
		       trim(both ' ' from (m.series || ' ' || m.size)) AS label,
		       m.rpm,
		       c.q_min_m3h,
		       c.q_max_m3h,
		       c.p_coeffs,
		       c.n_coeffs
		FROM fan_curves c
		JOIN fan_models m ON m.id = c.fan_id
		WHERE numrange(c.q_min_m3h::numeric, c.q_max_m3h::numeric, '[]')
		      @> $1::numeric
		ORDER BY m.id
		LIMIT $2
	`
	rows, err := s.pool.Query(ctx, query, qTarget, maxN)
	if err != nil {
		return nil, fmt.Errorf("storage: query candidates: %w", err)
	}
	defer rows.Close()

	out := make([]matching.FanCandidate, 0, 16)
	for rows.Next() {
		var (
			c        matching.FanCandidate
			pCoeffs  []float32
			nCoeffs  []float32
			qMin     float32
			qMax     float32
		)
		if err := rows.Scan(&c.FanID, &c.Label, &c.Rpm, &qMin, &qMax, &pCoeffs, &nCoeffs); err != nil {
			return nil, fmt.Errorf("storage: scan candidate: %w", err)
		}
		c.QMin = float64(qMin)
		c.QMax = float64(qMax)
		c.PCoeffs = floatsToFloat64(pCoeffs)
		c.NCoeffs = floatsToFloat64(nCoeffs)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iter candidates: %w", err)
	}
	return out, nil
}

func floatsToFloat64(src []float32) []float64 {
	if len(src) == 0 {
		return nil
	}
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = float64(v)
	}
	return out
}
