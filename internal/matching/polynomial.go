// Package matching evaluates fan performance polynomials and locates duty-point
// intersections. All functions in this file are pure — they read no shared
// state and have no I/O — so they unit-test cleanly without infrastructure.
package matching

import "math"

// Eval evaluates the polynomial a0 + a1·x + a2·x² + … at x using Horner's
// method. Coefficients are in ascending-degree order: coeffs[0] is the
// constant term. An empty coefficient slice returns 0.
func Eval(coeffs []float64, x float64) float64 {
	if len(coeffs) == 0 {
		return 0
	}
	// Horner: ((aN·x + aN-1)·x + aN-2)·x + … + a0
	result := coeffs[len(coeffs)-1]
	for i := len(coeffs) - 2; i >= 0; i-- {
		result = result*x + coeffs[i]
	}
	return result
}

// Solve finds x in [lo, hi] such that Eval(coeffs, x) ≈ target, using bisection.
// Returns (x, true) when |f(x) − target| < 1e-3 within 60 iterations.
// Returns (0, false) when there is no sign change in the interval, when lo ≥ hi,
// or when inputs are NaN/Inf.
//
// Bisection is sufficient for fan P–Q curves because they are monotone
// decreasing in the operating range. Newton's method would converge faster
// but adds the risk of overshoot near boundaries; we trade speed for
// robustness.
func Solve(coeffs []float64, target, lo, hi float64) (float64, bool) {
	if math.IsNaN(target) || math.IsInf(target, 0) {
		return 0, false
	}
	if math.IsNaN(lo) || math.IsNaN(hi) || lo >= hi {
		return 0, false
	}

	f := func(x float64) float64 { return Eval(coeffs, x) - target }
	flo, fhi := f(lo), f(hi)

	// No sign change → no guaranteed root in the interval.
	if flo*fhi > 0 {
		return 0, false
	}
	// Handle boundary hits.
	if math.Abs(flo) < 1e-3 {
		return lo, true
	}
	if math.Abs(fhi) < 1e-3 {
		return hi, true
	}

	const (
		eps     = 1e-3
		maxIter = 60
	)
	for i := 0; i < maxIter; i++ {
		mid := (lo + hi) / 2
		fmid := f(mid)
		if math.Abs(fmid) < eps {
			return mid, true
		}
		if flo*fmid < 0 {
			hi = mid
		} else {
			lo = mid
			flo = fmid
		}
	}
	return (lo + hi) / 2, true
}

// Efficiency computes useful aeraulic power divided by shaft power.
// qActual is the operating-point flow in m³/h, pTarget is the static pressure
// in Pa, powerKw is the shaft power at qActual in kW.
//
// Useful power (W) = Q (m³/s) · P (Pa) = (Q/3600) · P.
// Shaft power (W) = powerKw · 1000.
// Result is clamped to [0, 1] — a value above 1 indicates bad coefficients
// (e.g., extrapolation outside the fitted range), and we do not propagate it.
func Efficiency(qActual, pTarget, powerKw float64) float64 {
	if powerKw <= 0 || qActual <= 0 || pTarget <= 0 {
		return 0
	}
	usefulW := (qActual / 3600.0) * pTarget
	shaftW := powerKw * 1000.0
	eta := usefulW / shaftW
	if eta < 0 {
		return 0
	}
	if eta > 1 {
		return 1
	}
	return eta
}
