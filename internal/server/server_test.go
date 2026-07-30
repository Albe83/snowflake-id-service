package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Albe83/id-service/internal/idgen"
)

var uuidV7Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestGenerateIDs_DefaultCount(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q, want application/json", ct)
	}

	var resp generateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.IDs) != 1 {
		t.Fatalf("ids len: got %d, want 1", len(resp.IDs))
	}
	if !uuidV7Re.MatchString(resp.IDs[0]) {
		t.Fatalf("id %q does not match UUIDv7", resp.IDs[0])
	}
}

func TestGenerateIDs_BatchCount(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodPost, "/v1/ids?count=3", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp generateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.IDs) != 3 {
		t.Fatalf("ids len: got %d, want 3", len(resp.IDs))
	}

	seen := make(map[string]struct{}, len(resp.IDs))
	for _, id := range resp.IDs {
		if !uuidV7Re.MatchString(id) {
			t.Fatalf("id %q does not match UUIDv7", id)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id in batch: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGenerateIDs_MaxBoundary(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodPost, "/v1/ids?count=1000", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp generateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.IDs) != 1000 {
		t.Fatalf("ids len: got %d, want 1000", len(resp.IDs))
	}

	seen := make(map[string]struct{}, len(resp.IDs))
	for _, id := range resp.IDs {
		if !uuidV7Re.MatchString(id) {
			t.Fatalf("id %q does not match UUIDv7", id)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id at max batch: %s", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGenerateIDs_InvalidCount(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	tests := []struct {
		name   string
		target string
	}{
		{"zero", "/v1/ids?count=0"},
		{"negative", "/v1/ids?count=-1"},
		{"over max", "/v1/ids?count=1001"},
		{"not integer", "/v1/ids?count=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.target, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
			}
			var resp errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if resp.Error == "" {
				t.Fatal("empty error message")
			}
		})
	}
}

func TestGenerateIDs_TimestampMatchesClock(t *testing.T) {
	fixedMS := int64(1717500204000)
	mux := New(idgen.NewGenerator(func() int64 { return fixedMS })).Routes()

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp generateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.IDs) != 1 {
		t.Fatalf("ids len: got %d, want 1", len(resp.IDs))
	}

	hexStr := strings.ReplaceAll(resp.IDs[0], "-", "")
	ts, err := strconv.ParseInt(hexStr[:12], 16, 64)
	if err != nil {
		t.Fatalf("parse timestamp: %v", err)
	}
	if ts != fixedMS {
		t.Fatalf("timestamp: got %d, want %d", ts, fixedMS)
	}
}

func TestGenerateIDs_GeneratorError_500(t *testing.T) {
	mux := New(idgen.NewGenerator(func() int64 { return 1 << 48 })).Routes()

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("empty error message")
	}
}

func TestGenerateIDs_ConcurrentNoDuplicates(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	const goroutines = 10
	const count = 1000

	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := make(map[string]struct{}, goroutines*count)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/ids?count="+strconv.Itoa(count), nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status: got %d, want %d", rec.Code, http.StatusOK)
				return
			}
			var resp generateResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Errorf("unmarshal: %v", err)
				return
			}
			mu.Lock()
			for _, id := range resp.IDs {
				if _, ok := seen[id]; ok {
					t.Errorf("duplicate id across goroutines: %s", id)
				}
				seen[id] = struct{}{}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	want := goroutines * count
	if len(seen) != want {
		t.Fatalf("unique ids: got %d, want %d", len(seen), want)
	}
}

func TestRoutes_DecodeStub(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodGet, "/v1/ids/018f3a2c-9e5b-7000-8000-123456789abc", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	want := `{"id":"018f3a2c-9e5b-7000-8000-123456789abc","timestamp_ms":1714667953755,"timestamp_iso":"2024-05-02T16:39:13.755Z","version":7,"variant":"10xx","random_payload":"00000000123456789abc"}`

	var got, wantMap map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantMap); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !equalJSON(got, wantMap) {
		t.Fatalf("body: got %s, want %s", rec.Body.String(), want)
	}
}

func TestRoutes_Healthz(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status: got %q, want %q", resp.Status, "ok")
	}
}

func TestRoutes_Readyz(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("status: got %q, want %q", resp.Status, "ok")
	}
}

func TestRoutes_MetricsNotFound(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRoutes_MethodNotAllowed(t *testing.T) {
	mux := New(idgen.NewGenerator(nil)).Routes()

	req := httptest.NewRequest(http.MethodGet, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func equalJSON(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			return false
		}
		if !deepEqual(av, bv) {
			return false
		}
	}
	return true
}

func deepEqual(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			return false
		}
		return equalJSON(av, bv)
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
