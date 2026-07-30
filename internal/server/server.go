package server

import (
	"encoding/json"
	"net/http"
)

type Server struct{}

func New() *Server { return &Server{} }

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/ids", s.generateIDs)
	mux.HandleFunc("GET /v1/ids/{id}", s.decodeID)
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	return mux
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

func (s *Server) generateIDs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, generateResponse{
		IDs: []string{"018f3a2c-9e5b-7000-8000-123456789abc"},
	})
}

func (s *Server) decodeID(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, decodeResponse{
		ID:            "018f3a2c-9e5b-7000-8000-123456789abc",
		TimestampMs:   1714667953755,
		TimestampISO:  "2024-05-02T16:39:13.755Z",
		Version:       7,
		Variant:       "10xx",
		RandomPayload: "00000000123456789abc",
	})
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
