package router

import (
	"net/http"

	"github.com/sh3lwan/jobhunter/internal/handlers"
)

func NewRouter(h *handlers.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", h.HealthCheck)

	mux.HandleFunc("POST /api/v1/upload", h.UploadCV)
	mux.HandleFunc("GET /api/v1/stream", h.StreamCVStatus)
	mux.HandleFunc("GET /api/v1/jobs", h.FetchJobs)

	// CVs
	mux.HandleFunc("GET /api/v1/cvs", h.ListCVs)
	mux.HandleFunc("POST /api/v1/cvs/{id}/fetch", h.FetchJobsForCV);
	mux.HandleFunc("POST /api/v1/cvs/{id}/retry", h.RetryCVProceassing)
	mux.HandleFunc("POST /api/v1/cvs/{id}/match", h.EmbeddJobs)

	return mux
}
