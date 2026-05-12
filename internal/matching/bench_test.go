package matching

import (
	"context"
	"math/rand"
	"testing"
)

// BenchmarkEval measures Horner evaluation throughput. The polynomial degree
// (4 coefficients = cubic) matches the typical shape of a published fan
// performance curve.
func BenchmarkEval(b *testing.B) {
	coeffs := []float64{400, -0.05, 1e-6, -3e-10}
	x := 3000.0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Eval(coeffs, x)
	}
}

// BenchmarkSolve measures bisection root finding on the same cubic curve.
// 60 iterations max, but most fan curves converge in 12–18.
func BenchmarkSolve(b *testing.B) {
	coeffs := []float64{400, -0.05, 1e-6, -3e-10}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Solve(coeffs, 250, 500, 6000)
	}
}

// BenchmarkEvaluate_50Candidates simulates the realistic shape of the work
// done per HTTP request after the DB prefilter has narrowed the catalog.
// 50 candidates is the high end of what we expect from a Postgres GiST range
// query on a typical duty point.
func BenchmarkEvaluate_50Candidates(b *testing.B) {
	candidates := generateCandidates(50, 42)
	req := MatchRequest{QTargetM3h: 3000, PTargetPa: 250, Tolerance: 0.10, Limit: 10}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Evaluate(context.Background(), req, candidates)
	}
}

// BenchmarkEvaluate_10Candidates is the more common case: after prefilter,
// only a handful of fans bracket the target duty point.
func BenchmarkEvaluate_10Candidates(b *testing.B) {
	candidates := generateCandidates(10, 7)
	req := MatchRequest{QTargetM3h: 3000, PTargetPa: 250, Tolerance: 0.10, Limit: 10}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Evaluate(context.Background(), req, candidates)
	}
}

// BenchmarkCacheKey ensures key hashing does not become the bottleneck under
// burst traffic. SHA-256 over a 60-byte string should be well under a µs.
func BenchmarkCacheKey(b *testing.B) {
	req := MatchRequest{QTargetM3h: 3000, PTargetPa: 250, Tolerance: 0.10, Limit: 10}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = req.CacheKey()
	}
}

func generateCandidates(n int, seed int64) []FanCandidate {
	r := rand.New(rand.NewSource(seed))
	out := make([]FanCandidate, n)
	for i := 0; i < n; i++ {
		// Realistic centrifugal-fan-shaped coefficients:
		// P(Q) starts high at low Q, drops to zero at high Q.
		pHead := 200 + r.Float64()*600
		slope := -pHead / (5000 + r.Float64()*5000)
		out[i] = FanCandidate{
			FanID:   "f" + string(rune('0'+i%10)),
			Label:   "Fan ",
			Rpm:     1450,
			QMin:    500,
			QMax:    10000,
			PCoeffs: []float64{pHead, slope, 1e-6, -3e-10},
			NCoeffs: []float64{0.3 + r.Float64()*0.4, 0.0002, 1e-8},
		}
	}
	return out
}
