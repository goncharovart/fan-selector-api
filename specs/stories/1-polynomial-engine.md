# Story 1 — Polynomial engine

## Why
Every match request evaluates a polynomial (`P(Q)`, `N(Q)`) many times. The
engine has to be fast, numerically stable, and side-effect-free so it can be
unit-tested without DB/cache.

## Acceptance criteria
1. Function `Eval(coeffs []float64, x float64) float64` evaluates a polynomial
   `a0 + a1·x + a2·x² + …` using Horner's method.
2. Function `Solve(coeffs []float64, target float64, lo, hi float64) (float64, bool)`
   finds `x` in `[lo, hi]` where `Eval(coeffs, x) == target` using bisection.
   Returns `(x, true)` on convergence, `(0, false)` if no sign change in the
   interval. Tolerance: `|f(x_mid) - target| < 1e-3`, max 60 iterations.
3. `Efficiency(qActual, pTarget, powerKw float64) float64` returns
   `(qActual · pTarget) / (3600 · powerKw · 1000)`, clamped to `[0, 1]`.
4. Tests cover: cubic with one root, monotone-decreasing curve, no-root case,
   target at boundary, NaN / Inf inputs.
5. All in `internal/matching/`. No imports from other internal packages.

## Out of scope
Newton's method (bisection is enough for monotone-decreasing fan curves).
