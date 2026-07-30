package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Albe83/id-service/internal/idgen"
)

type Server struct {
	gen *idgen.Generator
	log *slog.Logger
}

func New(gen *idgen.Generator, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{gen: gen, log: log}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/ids", s.generateIDs)
	mux.HandleFunc("GET /v1/ids/{id}", s.decodeID)
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	return s.loggingMiddleware(mux)
}

type generateResponse struct {
	IDs []string `json:"ids"`
}

type decodeResponse struct {
	ID            string `json:"id"`
	TimestampMs   int64  `json:"timestamp_ms"`
	TimestampISO  string `json:"timestamp_iso"`
	Version       int    `json:"version"`
	Variant       string `json:"variant"`
	RandomPayload string `json:"random_payload"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) generateIDs(w http.ResponseWriter, r *http.Request) {
	count := 1
	if q := r.URL.Query().Get("count"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n < 1 || n > idgen.MaxBatchSize {
			s.log.DebugContext(r.Context(), "bad request: invalid count",
				"count", q, "error", err)
			writeJSON(w, http.StatusBadRequest, errorResponse{
				Error: "count must be between 1 and " + strconv.Itoa(idgen.MaxBatchSize),
			})
			return
		}
		count = n
	}

	ids, err := s.gen.NextIDs(count)
	if err != nil {
		s.log.ErrorContext(r.Context(), "generate failed",
			"error", err, "count", count)
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error: err.Error(),
		})
		return
	}

	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = idgen.String(id)
	}
	writeJSON(w, http.StatusOK, generateResponse{IDs: out})
}

func (s *Server) decodeID(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("id")

	id, err := idgen.Parse(raw)
	if err != nil {
		s.log.DebugContext(r.Context(), "decode bad request",
			"id", raw, "error", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: err.Error(),
		})
		return
	}

	d := idgen.Decode(id)
	writeJSON(w, http.StatusOK, decodeResponse{
		ID:            raw,
		TimestampMs:   d.TimestampMs,
		TimestampISO:  time.UnixMilli(d.TimestampMs).UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		Version:       d.Version,
		Variant:       d.Variant,
		RandomPayload: d.RandomPayload,
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		s.log.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"bytes", rec.bytes,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("json encode failed", "error", err)
	}
}
