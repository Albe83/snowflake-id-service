package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
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

func testHandler(gen *idgen.Generator, buf *bytes.Buffer) http.Handler {
	var w io.Writer = io.Discard
	if buf != nil {
		w = buf
	}
	log := slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return New(gen, log).Routes()
}

func TestGenerateIDs_DefaultCount(t *testing.T) {
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids?count=3", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids?count=1000", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(nil), nil)

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
			h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(func() int64 { return fixedMS }), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(func() int64 { return 1 << 48 }), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(nil), nil)

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
			h.ServeHTTP(rec, req)

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

func TestDecodeID_Valid(t *testing.T) {
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/ids/018f3a2c-9e5b-7000-8000-123456789abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type: got %q, want application/json", ct)
	}

	var resp decodeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ID != "018f3a2c-9e5b-7000-8000-123456789abc" {
		t.Fatalf("id: got %q, want canonical echo", resp.ID)
	}
	if resp.TimestampMs != 1714667953755 {
		t.Fatalf("timestamp_ms: got %d, want %d", resp.TimestampMs, 1714667953755)
	}
	if resp.TimestampISO != "2024-05-02T16:39:13.755Z" {
		t.Fatalf("timestamp_iso: got %q, want %q", resp.TimestampISO, "2024-05-02T16:39:13.755Z")
	}
	if resp.Version != 7 {
		t.Fatalf("version: got %d, want 7", resp.Version)
	}
	if resp.Variant != "10xx" {
		t.Fatalf("variant: got %q, want %q", resp.Variant, "10xx")
	}
	if resp.RandomPayload != "00000000123456789abc" {
		t.Fatalf("random_payload: got %q, want %q", resp.RandomPayload, "00000000123456789abc")
	}
}

func TestDecodeID_Invalid(t *testing.T) {
	h := testHandler(idgen.NewGenerator(nil), nil)

	tests := []struct {
		name string
		id   string
	}{
		{"not a uuid", "not-a-uuid"},
		{"too short", "018f3a2c-9e5b-7000-8000-123456789ab"},
		{"version 4", "018f3a2c-9e5b-4000-8000-123456789abc"},
		{"variant 00", "018f3a2c-9e5b-7000-0000-123456789abc"},
		{"uppercase", "018F3A2C-9E5B-7000-8000-123456789ABC"},
		{"non-hex", "018f3a2c-9e5b-7000-8000-123456789abz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/ids/"+tt.id, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

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

func TestDecodeID_Roundtrip(t *testing.T) {
	h := testHandler(idgen.NewGenerator(nil), nil)

	genReq := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	genRec := httptest.NewRecorder()
	h.ServeHTTP(genRec, genReq)

	var genResp generateResponse
	if err := json.Unmarshal(genRec.Body.Bytes(), &genResp); err != nil {
		t.Fatalf("generate unmarshal: %v", err)
	}
	generated := genResp.IDs[0]

	decReq := httptest.NewRequest(http.MethodGet, "/v1/ids/"+generated, nil)
	decRec := httptest.NewRecorder()
	h.ServeHTTP(decRec, decReq)

	if decRec.Code != http.StatusOK {
		t.Fatalf("decode status: got %d, want %d", decRec.Code, http.StatusOK)
	}
	var decResp decodeResponse
	if err := json.Unmarshal(decRec.Body.Bytes(), &decResp); err != nil {
		t.Fatalf("decode unmarshal: %v", err)
	}
	if decResp.ID != generated {
		t.Fatalf("id echo: got %q, want %q", decResp.ID, generated)
	}
	if decResp.Version != 7 {
		t.Fatalf("version: got %d, want 7", decResp.Version)
	}
	if decResp.Variant != "10xx" {
		t.Fatalf("variant: got %q, want %q", decResp.Variant, "10xx")
	}
	if !uuidV7Re.MatchString(decResp.ID) {
		t.Fatalf("id %q does not match UUIDv7", decResp.ID)
	}
}

func TestRoutes_Healthz(t *testing.T) {
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

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
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRoutes_MethodNotAllowed(t *testing.T) {
	h := testHandler(idgen.NewGenerator(nil), nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestLogging_AccessLog(t *testing.T) {
	var buf bytes.Buffer
	h := testHandler(idgen.NewGenerator(nil), &buf)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	logOutput := buf.String()
	for _, want := range []string{
		"level=INFO",
		"msg=request",
		"method=POST",
		"path=/v1/ids",
		"status=200",
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("log missing %q, got:\n%s", want, logOutput)
		}
	}
}

func TestLogging_HealthzNotLogged(t *testing.T) {
	var buf bytes.Buffer
	h := testHandler(idgen.NewGenerator(nil), &buf)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if buf.Len() > 0 {
		t.Fatalf("healthz should not be logged, got:\n%s", buf.String())
	}
}

func TestLogging_ReadyzNotLogged(t *testing.T) {
	var buf bytes.Buffer
	h := testHandler(idgen.NewGenerator(nil), &buf)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
	if buf.Len() > 0 {
		t.Fatalf("readyz should not be logged, got:\n%s", buf.String())
	}
}

func TestLogging_GeneratorError500(t *testing.T) {
	var buf bytes.Buffer
	h := testHandler(idgen.NewGenerator(func() int64 { return 1 << 48 }), &buf)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	logOutput := buf.String()
	if !strings.Contains(logOutput, `level=ERROR`) {
		t.Fatalf("log missing ERROR level, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, `msg="generate failed"`) {
		t.Fatalf("log missing generate failed message, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "status=500") {
		t.Fatalf("log missing status=500 access line, got:\n%s", logOutput)
	}
}

func TestLogging_BadRequest400_DebugOnly(t *testing.T) {
	var buf bytes.Buffer
	h := testHandler(idgen.NewGenerator(nil), &buf)

	req := httptest.NewRequest(http.MethodPost, "/v1/ids?count=abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}

	logOutput := buf.String()
	// testHandler uses Debug level, so the DebugContext line should be present
	if !strings.Contains(logOutput, "level=DEBUG") {
		t.Fatalf("log missing DEBUG level for bad request, got:\n%s", logOutput)
	}
	if !strings.Contains(logOutput, "bad request") {
		t.Fatalf("log missing bad request message, got:\n%s", logOutput)
	}
	// access line should still be Info
	if !strings.Contains(logOutput, "status=400") {
		t.Fatalf("log missing status=400 access line, got:\n%s", logOutput)
	}
}
