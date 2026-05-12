package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/goncharovart/fan-selector-api/internal/matching"
)

// --- Fakes ---

type fakeStore struct {
	candidates []matching.FanCandidate
	pingErr    error
	queryErr   error
}

func (f *fakeStore) Candidates(_ context.Context, _ float64, _ int) ([]matching.FanCandidate, error) {
	return f.candidates, f.queryErr
}
func (f *fakeStore) Ping(_ context.Context) error { return f.pingErr }

type fakeCache struct {
	mu      sync.Mutex
	store   map[string][]byte
	pingErr error
}

func newFakeCache() *fakeCache { return &fakeCache{store: map[string][]byte{}} }

func (c *fakeCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.store[key]
	return v, ok, nil
}
func (c *fakeCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
	return nil
}
func (c *fakeCache) Ping(_ context.Context) error { return c.pingErr }

// --- Helpers ---

func newTestHandler(store *fakeStore, cache *fakeCache) *Handler {
	return NewHandler(store, store, cache, slog.New(slog.NewTextHandler(io.Discard, nil)), 5*time.Minute, 100)
}

func sampleCandidate() matching.FanCandidate {
	return matching.FanCandidate{
		FanID: "A", Label: "Fan A", Rpm: 1450, QMin: 500, QMax: 5000,
		PCoeffs: []float64{400, -0.05},
		NCoeffs: []float64{0.5, 0.0001},
	}
}

// --- Tests ---

func TestHealthz(t *testing.T) {
	h := newTestHandler(&fakeStore{}, newFakeCache())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("healthz: status %d", rec.Code)
	}
}

func TestReadyz_OK(t *testing.T) {
	h := newTestHandler(&fakeStore{}, newFakeCache())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("readyz: status %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyz_PostgresDown(t *testing.T) {
	store := &fakeStore{pingErr: errors.New("connection refused")}
	h := newTestHandler(store, newFakeCache())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when postgres down, got %d", rec.Code)
	}
}

func TestReadyz_RedisDown(t *testing.T) {
	cache := newFakeCache()
	cache.pingErr = errors.New("connection refused")
	h := newTestHandler(&fakeStore{}, cache)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when redis down, got %d", rec.Code)
	}
}

func TestMatch_HappyPath(t *testing.T) {
	store := &fakeStore{candidates: []matching.FanCandidate{sampleCandidate()}}
	h := newTestHandler(store, newFakeCache())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/match?q=2000&p=300", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Cache"); got != "MISS" {
		t.Errorf("expected X-Cache=MISS, got %q", got)
	}

	var resp matchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Matches) != 1 {
		t.Errorf("expected 1 match, got count=%d len=%d", resp.Count, len(resp.Matches))
	}
	if resp.Matches[0].FanID != "A" {
		t.Errorf("expected FanID=A, got %q", resp.Matches[0].FanID)
	}
}

func TestMatch_CacheHitOnSecondCall(t *testing.T) {
	store := &fakeStore{candidates: []matching.FanCandidate{sampleCandidate()}}
	cache := newFakeCache()
	h := newTestHandler(store, cache)

	// First call → MISS, populates cache.
	rec1 := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/v1/match?q=2000&p=300", nil))
	if rec1.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("first call should miss; got %q", rec1.Header().Get("X-Cache"))
	}

	// Second identical call → HIT.
	rec2 := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/v1/match?q=2000&p=300", nil))
	if rec2.Header().Get("X-Cache") != "HIT" {
		t.Errorf("second call should hit; got %q", rec2.Header().Get("X-Cache"))
	}
	if rec2.Body.String() != rec1.Body.String() {
		t.Errorf("cached body differs from original")
	}
}

func TestMatch_BadRequest(t *testing.T) {
	h := newTestHandler(&fakeStore{}, newFakeCache())
	cases := []string{
		"/v1/match",
		"/v1/match?q=abc&p=300",
		"/v1/match?q=-1&p=300",
		"/v1/match?q=2000&p=0",
		"/v1/match?q=2000&p=300&tolerance=0.6",
		"/v1/match?q=2000&p=300&limit=0",
		"/v1/match?q=2000&p=300&limit=51",
	}
	for _, url := range cases {
		rec := httptest.NewRecorder()
		h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d (body=%s)", url, rec.Code, rec.Body.String())
		}
	}
}

func TestMatch_StoreError(t *testing.T) {
	store := &fakeStore{queryErr: errors.New("db down")}
	h := newTestHandler(store, newFakeCache())
	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/match?q=2000&p=300", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 on store error, got %d", rec.Code)
	}
}
