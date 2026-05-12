// Command seed loads a small synthetic fan catalog into Postgres.
// It is idempotent: re-running it upserts rather than duplicating.
//
// Coefficients are synthetic but realistic — derived from typical
// centrifugal-fan curve shapes, NOT scraped from any vendor catalog.
// Use this only for local testing and demos.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type sampleFan struct {
	ID           string
	Manufacturer string
	Series       string
	Size         string
	Rpm          int
	ImpellerD    int
	QMin, QMax   float32
	PCoeffs      []float32
	NCoeffs      []float32
}

// Twelve fans across a realistic range of duties. Coefficients are tuned so
// that pressure decreases monotonically with flow within (QMin, QMax) and
// power increases moderately. All values synthetic.
var fans = []sampleFan{
	{"demo-100", "DemoCorp", "Demo-100", "№2.5", 2900, 250, 200, 1500,
		[]float32{260, -0.12}, []float32{0.05, 0.0002}},
	{"demo-200", "DemoCorp", "Demo-200", "№3.15", 1450, 315, 500, 2500,
		[]float32{300, -0.06}, []float32{0.15, 0.00018}},
	{"demo-300", "DemoCorp", "Demo-300", "№4", 1450, 400, 1000, 4500,
		[]float32{400, -0.05}, []float32{0.30, 0.00018}},
	{"demo-400", "DemoCorp", "Demo-400", "№5", 1450, 500, 1500, 6500,
		[]float32{500, -0.045}, []float32{0.50, 0.00020}},
	{"demo-500", "DemoCorp", "Demo-500", "№6.3", 1450, 630, 2500, 10000,
		[]float32{600, -0.035}, []float32{0.80, 0.00022}},
	{"demo-600", "DemoCorp", "Demo-600", "№8", 1000, 800, 4000, 16000,
		[]float32{700, -0.025}, []float32{1.50, 0.00018}},
	{"high-1", "HighPress", "HP-A", "compact", 2900, 280, 200, 1800,
		[]float32{800, -0.32}, []float32{0.20, 0.00050}},
	{"high-2", "HighPress", "HP-B", "mid", 2900, 400, 500, 3500,
		[]float32{1200, -0.28}, []float32{0.60, 0.00040}},
	{"low-1", "LowPress", "LP-S", "S", 950, 500, 1500, 8000,
		[]float32{180, -0.018}, []float32{0.35, 0.00015}},
	{"low-2", "LowPress", "LP-M", "M", 950, 800, 4000, 15000,
		[]float32{220, -0.012}, []float32{0.90, 0.00012}},
	{"axial-1", "AxialCo", "AX-1", "315", 1450, 315, 1500, 5500,
		[]float32{220, -0.035}, []float32{0.25, 0.00020}},
	{"axial-2", "AxialCo", "AX-2", "500", 1450, 500, 3000, 12000,
		[]float32{280, -0.020}, []float32{0.60, 0.00018}},
}

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("seed: DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("seed: pool: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("seed: begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, f := range fans {
		if _, err := tx.Exec(ctx, `
			INSERT INTO fan_models (id, manufacturer, series, size, rpm, impeller_d_mm)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				manufacturer = EXCLUDED.manufacturer,
				series       = EXCLUDED.series,
				size         = EXCLUDED.size,
				rpm          = EXCLUDED.rpm,
				impeller_d_mm = EXCLUDED.impeller_d_mm
		`, f.ID, f.Manufacturer, f.Series, f.Size, f.Rpm, f.ImpellerD); err != nil {
			log.Fatalf("seed: upsert model %s: %v", f.ID, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO fan_curves (fan_id, q_min_m3h, q_max_m3h, p_coeffs, n_coeffs)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (fan_id) DO UPDATE SET
				q_min_m3h = EXCLUDED.q_min_m3h,
				q_max_m3h = EXCLUDED.q_max_m3h,
				p_coeffs  = EXCLUDED.p_coeffs,
				n_coeffs  = EXCLUDED.n_coeffs
		`, f.ID, f.QMin, f.QMax, f.PCoeffs, f.NCoeffs); err != nil {
			log.Fatalf("seed: upsert curve %s: %v", f.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("seed: commit: %v", err)
	}
	fmt.Printf("seeded %d fans\n", len(fans))
}
