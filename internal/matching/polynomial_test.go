package matching

import (
	"math"
	"testing"
)

func TestEval(t *testing.T) {
	tests := []struct {
		name   string
		coeffs []float64
		x      float64
		want   float64
	}{
		{"empty", nil, 5, 0},
		{"constant", []float64{7}, 100, 7},
		{"linear 2x+3 at x=4", []float64{3, 2}, 4, 11},
		{"cubic 1+x+x²+x³ at x=2", []float64{1, 1, 1, 1}, 2, 15},
		{"negative x", []float64{0, 1, 0, -1}, -2, 6}, // x - x³ at x=-2 → -2 - (-8) = 6
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Eval(tt.coeffs, tt.x)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Eval(%v, %v) = %v, want %v", tt.coeffs, tt.x, got, tt.want)
			}
		})
	}
}

func TestSolve_MonotoneCurveHasOneRoot(t *testing.T) {
	// A typical fan curve: P(Q) = 400 - 0.05·Q  → P=300 at Q=2000
	coeffs := []float64{400, -0.05}
	x, ok := Solve(coeffs, 300, 0, 5000)
	if !ok {
		t.Fatal("expected convergence")
	}
	if math.Abs(x-2000) > 0.1 {
		t.Errorf("expected x≈2000, got %v", x)
	}
}

func TestSolve_NoSignChangeReturnsFalse(t *testing.T) {
	// P(Q) = 100 (constant). Target = 50 is unreachable in any interval.
	coeffs := []float64{100}
	_, ok := Solve(coeffs, 50, 0, 5000)
	if ok {
		t.Error("expected failure for no sign change")
	}
}

func TestSolve_TargetAtBoundary(t *testing.T) {
	coeffs := []float64{400, -0.05}
	// At Q=0, P=400. Asking for P=400 should hit the low boundary.
	x, ok := Solve(coeffs, 400, 0, 5000)
	if !ok {
		t.Fatal("expected convergence at boundary")
	}
	if math.Abs(x) > 0.1 {
		t.Errorf("expected x≈0, got %v", x)
	}
}

func TestSolve_InvalidInputs(t *testing.T) {
	cases := []struct {
		name           string
		coeffs         []float64
		target, lo, hi float64
	}{
		{"NaN target", []float64{1, 1}, math.NaN(), 0, 100},
		{"Inf target", []float64{1, 1}, math.Inf(1), 0, 100},
		{"NaN lo", []float64{1, 1}, 50, math.NaN(), 100},
		{"lo >= hi", []float64{1, 1}, 50, 100, 100},
		{"lo > hi", []float64{1, 1}, 50, 100, 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := Solve(c.coeffs, c.target, c.lo, c.hi); ok {
				t.Errorf("expected failure for %v", c.name)
			}
		})
	}
}

func TestEfficiency(t *testing.T) {
	// Q = 3000 m³/h, P = 300 Pa → useful W = (3000/3600)·300 = 250 W.
	// Shaft 0.74 kW = 740 W. η = 250/740 ≈ 0.338.
	got := Efficiency(3000, 300, 0.74)
	if math.Abs(got-0.3378) > 0.001 {
		t.Errorf("expected ≈0.338, got %v", got)
	}
}

func TestEfficiency_Clamping(t *testing.T) {
	cases := []struct {
		name                            string
		qActual, pTarget, powerKw, want float64
	}{
		{"zero power", 3000, 300, 0, 0},
		{"negative power", 3000, 300, -1, 0},
		{"zero flow", 0, 300, 0.5, 0},
		{"impossible efficiency clamps to 1", 100000, 1000, 0.001, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Efficiency(c.qActual, c.pTarget, c.powerKw)
			if math.Abs(got-c.want) > 1e-9 {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}
