package router

import (
	"net/http"

	"github.com/sh3lwan/jobhunter/internal/handlers"
	"github.com/sh3lwan/jobhunter/internal/middleware"
)

func NewRouter(h *handlers.Handler, authMiddleware *middleware.AuthMiddleware) *http.ServeMux {
	mux := http.NewServeMux()
	protected := func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
		mux.Handle(pattern, authMiddleware.RequireAuth(http.HandlerFunc(handler)))
	}

	// Public routes
	mux.HandleFunc("POST /api/v1/auth/login", h.Authenticate)

	// Health Check
	protected("GET /api/v1/health", h.HealthCheck)

	// CV Processing
	protected("POST /api/v1/cvs", h.UploadCV)
	protected("GET /api/v1/cvs", h.ListCVs)
	protected("POST /api/v1/cvs/{id}/embed", h.EmbedCV)
	protected("POST /api/v1/cvs/{id}/fetch", h.FetchJobsForCV)
	protected("POST /api/v1/cvs/{id}/retry", h.RetryCVProceassing)
	protected("POST /api/v1/cvs/{id}/match", h.EmbeddJobs)

	// Jobs
	protected("GET /api/v1/stream", h.StreamCVStatus)
	protected("GET /api/v1/jobs", h.FetchJobs)
	protected("POST /api/v1/jobs/{id}/embed", h.EmbedJob)

	return mux
}
