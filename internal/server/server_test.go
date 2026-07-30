package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes_StaticResponses(t *testing.T) {
	mux := New().Routes()

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "generate single id",
			method:     http.MethodPost,
			target:     "/v1/ids",
			wantStatus: http.StatusOK,
			wantBody:   `{"ids":["018f3a2c-9e5b-7000-8000-123456789abc"]}`,
		},
		{
			name:       "generate batch ignores count",
			method:     http.MethodPost,
			target:     "/v1/ids?count=3",
			wantStatus: http.StatusOK,
			wantBody:   `{"ids":["018f3a2c-9e5b-7000-8000-123456789abc"]}`,
		},
		{
			name:       "decode id ignores path param",
			method:     http.MethodGet,
			target:     "/v1/ids/018f3a2c-9e5b-7000-8000-123456789abc",
			wantStatus: http.StatusOK,
			wantBody:   `{"id":"018f3a2c-9e5b-7000-8000-123456789abc","timestamp_ms":1714667953755,"timestamp_iso":"2024-05-02T16:39:13.755Z","version":7,"variant":"10xx","random_payload":"00000000123456789abc"}`,
		},
		{
			name:       "healthz",
			method:     http.MethodGet,
			target:     "/healthz",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name:       "readyz",
			method:     http.MethodGet,
			target:     "/readyz",
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.target, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type: got %q, want application/json", ct)
			}
			var got, want map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.wantBody), &want); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}
			if !equalJSON(got, want) {
				t.Fatalf("body: got %s, want %s", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestRoutes_MetricsNotFound(t *testing.T) {
	mux := New().Routes()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRoutes_MethodNotAllowed(t *testing.T) {
	mux := New().Routes()

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
