// Package api wires HTTP requests to the matching engine. Handlers stay
// thin: parse → call engine → serialize. All real work lives in
// internal/matching and internal/storage.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/goncharovart/fan-selector-api/internal/matching"
)

// Pinger is satisfied by both the real Store and a fake for tests.
type Pinger interface {
	Ping(ctx context.Context) error
}

// CandidateSource is the read path the API needs from storage.
type CandidateSource interface {
	Candidates(ctx context.Context, qTarget float64, maxN int) ([]matching.FanCandidate, error)
}

// Cache mirrors storage.Cache without importing the storage package, so
// tests can supply an in-memory fake.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Ping(ctx context.Context) error
}

// Handler owns the HTTP entrypoints.
type Handler struct {
	store               CandidateSource
	storePing           Pinger
	cache               Cache
	logger              *slog.Logger
	cacheTTL            time.Duration
	maxCandidates       int
}

// NewHandler returns a wired-up router. The two ping arguments let us
// distinguish "the store can serve queries" from "the store responds to
// pings" — useful for /readyz when we have a richer store interface.
func NewHandler(store CandidateSource, storePing Pinger, cache Cache, logger *slog.Logger, cacheTTL time.Duration, maxCandidates int) *Handler {
	return &Handler{
		store:         store,
		storePing:     storePing,
		cache:         cache,
		logger:        logger,
		cacheTTL:      cacheTTL,
		maxCandidates: maxCandidates,
	}
}

// Routes returns a chi router preconfigured with our endpoints and
// minimal middleware. Adding/removing middleware is a one-line change here
// rather than scattered across main.go.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)
	r.Get("/v1/match", h.Match)
	return r
}

// Healthz answers as soon as the process can serve HTTP. It deliberately
// performs no dependency checks — that is what /readyz is for.
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports whether Postgres and Redis are reachable. Either failure
// yields 503; the body names which dependency is down.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.storePing.Ping(ctx); err != nil {
		h.logger.WarnContext(ctx, "readyz: postgres unreachable", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "down",
			"reason": "postgres",
		})
		return
	}
	if err := h.cache.Ping(ctx); err != nil {
		h.logger.WarnContext(ctx, "readyz: redis unreachable", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "down",
			"reason": "redis",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// matchResponse is the JSON shape returned by /v1/match.
type matchResponse struct {
	OperatingPoint operatingPoint    `json:"operating_point"`
	Tolerance      float64           `json:"tolerance"`
	Matches        []matching.Match  `json:"matches"`
	Count          int               `json:"count"`
	ElapsedMs      int64             `json:"elapsed_ms"`
}

type operatingPoint struct {
	QM3h float64 `json:"q_m3h"`
	PPa  float64 `json:"p_pa"`
}

// Match is the heart of the service.
func (h *Handler) Match(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()

	req, err := parseMatchRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	key := req.CacheKey()
	if cached, hit, err := h.cache.Get(ctx, key); err == nil && hit {
		// Cached body already conforms to our response shape. Pass it
		// through untouched so the cache stays the source of truth and
		// we don't re-evaluate just to re-encode.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(cached)
		return
	}

	candidates, err := h.store.Candidates(ctx, req.QTargetM3h, h.maxCandidates)
	if err != nil {
		h.logger.ErrorContext(ctx, "match: candidate lookup failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "could not load fan catalog")
		return
	}

	matches := matching.Evaluate(ctx, req, candidates)

	resp := matchResponse{
		OperatingPoint: operatingPoint{QM3h: req.QTargetM3h, PPa: req.PTargetPa},
		Tolerance:      req.Tolerance,
		Matches:        matches,
		Count:          len(matches),
		ElapsedMs:      time.Since(start).Milliseconds(),
	}

	body, err := json.Marshal(resp)
	if err != nil {
		h.logger.ErrorContext(ctx, "match: encode response", "err", err)
		writeError(w, http.StatusInternalServerError, "encode_failed", "could not encode response")
		return
	}

	if setErr := h.cache.Set(ctx, key, body, h.cacheTTL); setErr != nil {
		// A cache write failure is not a request failure. Log and move on.
		h.logger.WarnContext(ctx, "match: cache set failed", "err", setErr)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// parseMatchRequest reads query params with strict validation. Returns a
// human-friendly error so the API stays helpful when called by hand.
func parseMatchRequest(r *http.Request) (matching.MatchRequest, error) {
	q := r.URL.Query()

	qVal, err := parseFloatRequired(q.Get("q"), "q")
	if err != nil {
		return matching.MatchRequest{}, err
	}
	if qVal <= 0 {
		return matching.MatchRequest{}, errors.New("q must be > 0")
	}

	pVal, err := parseFloatRequired(q.Get("p"), "p")
	if err != nil {
		return matching.MatchRequest{}, err
	}
	if pVal <= 0 {
		return matching.MatchRequest{}, errors.New("p must be > 0")
	}

	tol := 0.05
	if raw := q.Get("tolerance"); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return matching.MatchRequest{}, errors.New("tolerance must be a number")
		}
		if v <= 0 || v > 0.5 {
			return matching.MatchRequest{}, errors.New("tolerance must be in (0, 0.5]")
		}
		tol = v
	}

	limit := 10
	if raw := q.Get("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return matching.MatchRequest{}, errors.New("limit must be an integer")
		}
		if v < 1 || v > 50 {
			return matching.MatchRequest{}, errors.New("limit must be in [1, 50]")
		}
		limit = v
	}

	return matching.MatchRequest{
		QTargetM3h: qVal,
		PTargetPa:  pVal,
		Tolerance:  tol,
		Limit:      limit,
	}, nil
}

func parseFloatRequired(raw, name string) (float64, error) {
	if raw == "" {
		return 0, errors.New(name + " is required")
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, errors.New(name + " must be a number")
	}
	return v, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
