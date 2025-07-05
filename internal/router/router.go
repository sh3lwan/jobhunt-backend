package router

import (
	"net/http"

	"github.com/sh3lwan/jobhunter/internal/handlers"
)

func NewRouter(h *handlers.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", h.HealthCheck)

	mux.HandleFunc("POST /api/v1/upload", h.UploadCV)

	mux.HandleFunc("GET /api/v1/cvs", h.ListCVs)

	mux.HandleFunc("GET /api/v1/fetch", h.FetchJobs)

	mux.HandleFunc("GET /api/v1/stream", h.StreamCVStatus)

	mux.HandleFunc("GET /api/v1/jobs", h.FetchRemotiveJobs)

	return mux
}
