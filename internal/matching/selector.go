package matching

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
)

// FanCandidate is a row coming back from the prefilter query.
// It contains everything the engine needs to evaluate one fan.
type FanCandidate struct {
	FanID    string
	Label    string
	Rpm      int
	QMin     float64
	QMax     float64
	PCoeffs  []float64 // P(Q) — static pressure polynomial
	NCoeffs  []float64 // N(Q) — shaft power polynomial (kW)
}

// Match is a single result row in the API response.
type Match struct {
	FanID              string  `json:"fan_id"`
	Label              string  `json:"label"`
	Rpm                int     `json:"rpm"`
	QAtIntersectionM3h float64 `json:"q_at_intersection_m3h"`
	PAtIntersectionPa  float64 `json:"p_at_intersection_pa"`
	PowerKw            float64 `json:"power_kw"`
	Efficiency         float64 `json:"efficiency"`
	Distance           float64 `json:"distance"`
}

// MatchRequest is the canonical, validated input to a match call.
// Two requests with the same MatchRequest must produce the same cache key.
type MatchRequest struct {
	QTargetM3h float64
	PTargetPa  float64
	Tolerance  float64
	Limit      int
}

// CacheKey returns a deterministic SHA-256 hex key for this request.
// Floats are formatted with %g (round-trip precision) to make the key stable
// across processes; the "v1|" prefix scopes the keyspace so we can bump it
// when the response shape changes.
func (r MatchRequest) CacheKey() string {
	canonical := fmt.Sprintf("v1|q=%g|p=%g|tol=%g|lim=%d",
		r.QTargetM3h, r.PTargetPa, r.Tolerance, r.Limit)
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

// Evaluate runs every candidate against the request, computes the operating
// point, drops candidates that don't intersect within tolerance, and returns
// the top `req.Limit` results sorted by efficiency desc, distance asc.
//
// The function is deliberately pure and synchronous. Candidate sets are small
// (well under 100 after the DB prefilter) and per-candidate work is a handful
// of Horner evaluations plus bisection — fan-out would add scheduling cost
// without measurable wins. If the catalog ever grows past 10K we can revisit.
func Evaluate(_ context.Context, req MatchRequest, candidates []FanCandidate) []Match {
	results := make([]Match, 0, len(candidates))

	for _, c := range candidates {
		// Skip immediately if the target is outside the manufacturer-declared
		// envelope. The DB prefilter should have caught most of these, but
		// the engine must defend itself in case it's called directly.
		if req.QTargetM3h < c.QMin || req.QTargetM3h > c.QMax {
			continue
		}

		qStar, ok := Solve(c.PCoeffs, req.PTargetPa, c.QMin, c.QMax)
		if !ok {
			continue
		}

		distance := math.Abs(qStar-req.QTargetM3h) / req.QTargetM3h
		if distance > req.Tolerance {
			continue
		}

		pStar := Eval(c.PCoeffs, qStar)
		nStar := Eval(c.NCoeffs, qStar)
		if nStar <= 0 {
			// A non-positive shaft power is nonsense; the coefficients are
			// either bad or evaluated outside their fitted range. Skip
			// rather than ship a misleading efficiency.
			continue
		}

		results = append(results, Match{
			FanID:              c.FanID,
			Label:              c.Label,
			Rpm:                c.Rpm,
			QAtIntersectionM3h: qStar,
			PAtIntersectionPa:  pStar,
			PowerKw:            nStar,
			Efficiency:         Efficiency(qStar, req.PTargetPa, nStar),
			Distance:           distance,
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Efficiency != results[j].Efficiency {
			return results[i].Efficiency > results[j].Efficiency
		}
		return results[i].Distance < results[j].Distance
	})

	if len(results) > req.Limit {
		results = results[:req.Limit]
	}
	return results
}
