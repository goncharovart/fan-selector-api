package matching

import (
	"context"
	"testing"
)

func TestCacheKey_IsDeterministic(t *testing.T) {
	a := MatchRequest{QTargetM3h: 3000, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	b := MatchRequest{QTargetM3h: 3000, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	if a.CacheKey() != b.CacheKey() {
		t.Errorf("expected identical keys, got %s vs %s", a.CacheKey(), b.CacheKey())
	}
}

func TestCacheKey_DiffersForDifferentInputs(t *testing.T) {
	base := MatchRequest{QTargetM3h: 3000, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	mutants := []MatchRequest{
		{QTargetM3h: 3001, PTargetPa: 300, Tolerance: 0.05, Limit: 10},
		{QTargetM3h: 3000, PTargetPa: 301, Tolerance: 0.05, Limit: 10},
		{QTargetM3h: 3000, PTargetPa: 300, Tolerance: 0.06, Limit: 10},
		{QTargetM3h: 3000, PTargetPa: 300, Tolerance: 0.05, Limit: 11},
	}
	baseKey := base.CacheKey()
	for i, m := range mutants {
		if m.CacheKey() == baseKey {
			t.Errorf("mutant %d produced same key as base", i)
		}
	}
}

func TestEvaluate_PicksFanThatMatches(t *testing.T) {
	// Two fans, both declare envelope [500, 5000].
	// Fan A: P(Q) = 400 − 0.05·Q, N(Q) = 0.5 + 0.0001·Q  → P=300 at Q=2000, N=0.7 kW
	// Fan B: P(Q) = 500 − 0.1·Q, N(Q) = 1.0 + 0.0002·Q   → P=300 at Q=2000, N=1.4 kW
	// Both intersect (Q=2000, P=300) exactly. Fan A wins on efficiency.
	fans := []FanCandidate{
		{
			FanID: "A", Label: "Fan A", Rpm: 1450, QMin: 500, QMax: 5000,
			PCoeffs: []float64{400, -0.05},
			NCoeffs: []float64{0.5, 0.0001},
		},
		{
			FanID: "B", Label: "Fan B", Rpm: 1450, QMin: 500, QMax: 5000,
			PCoeffs: []float64{500, -0.1},
			NCoeffs: []float64{1.0, 0.0002},
		},
	}
	req := MatchRequest{QTargetM3h: 2000, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	out := Evaluate(context.Background(), req, fans)
	if len(out) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(out))
	}
	if out[0].FanID != "A" {
		t.Errorf("expected Fan A first (higher η), got %v", out[0].FanID)
	}
	if out[0].Efficiency <= out[1].Efficiency {
		t.Errorf("first result should have higher efficiency: %v vs %v",
			out[0].Efficiency, out[1].Efficiency)
	}
}

func TestEvaluate_DropsOutOfEnvelope(t *testing.T) {
	// Target Q=10000 is outside the envelope [500, 5000].
	fans := []FanCandidate{{
		FanID: "A", QMin: 500, QMax: 5000,
		PCoeffs: []float64{400, -0.05},
		NCoeffs: []float64{0.5, 0.0001},
	}}
	req := MatchRequest{QTargetM3h: 10000, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	out := Evaluate(context.Background(), req, fans)
	if len(out) != 0 {
		t.Errorf("expected zero matches, got %d", len(out))
	}
}

func TestEvaluate_DropsBeyondTolerance(t *testing.T) {
	// Fan A intersects at Q=2000 for P=300, but caller wants Q=2500.
	// Distance = 500/2500 = 0.2; tolerance = 0.05 → drop.
	fans := []FanCandidate{{
		FanID: "A", QMin: 500, QMax: 5000,
		PCoeffs: []float64{400, -0.05},
		NCoeffs: []float64{0.5, 0.0001},
	}}
	req := MatchRequest{QTargetM3h: 2500, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	out := Evaluate(context.Background(), req, fans)
	if len(out) != 0 {
		t.Errorf("expected zero matches (out of tolerance), got %d", len(out))
	}
}

func TestEvaluate_RespectsLimit(t *testing.T) {
	// Three identical fans. Limit=2 → return 2.
	makeFan := func(id string) FanCandidate {
		return FanCandidate{
			FanID: id, QMin: 500, QMax: 5000,
			PCoeffs: []float64{400, -0.05},
			NCoeffs: []float64{0.5, 0.0001},
		}
	}
	fans := []FanCandidate{makeFan("A"), makeFan("B"), makeFan("C")}
	req := MatchRequest{QTargetM3h: 2000, PTargetPa: 300, Tolerance: 0.05, Limit: 2}
	out := Evaluate(context.Background(), req, fans)
	if len(out) != 2 {
		t.Errorf("limit=2 → expected 2 matches, got %d", len(out))
	}
}

func TestEvaluate_SkipsNonPositivePower(t *testing.T) {
	// N(Q) returns zero or negative → engine must skip without panicking.
	fans := []FanCandidate{{
		FanID: "Bad", QMin: 500, QMax: 5000,
		PCoeffs: []float64{400, -0.05},
		NCoeffs: []float64{0, 0}, // N(Q)=0 everywhere
	}}
	req := MatchRequest{QTargetM3h: 2000, PTargetPa: 300, Tolerance: 0.05, Limit: 10}
	out := Evaluate(context.Background(), req, fans)
	if len(out) != 0 {
		t.Errorf("expected zero matches (bad power), got %d", len(out))
	}
}
