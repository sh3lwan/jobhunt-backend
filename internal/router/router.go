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
	mux.HandleFunc("GET /api/v1/auth/google", h.GoogleAuthStart)
	mux.HandleFunc("GET /api/v1/auth/google/callback", h.GoogleAuthCallback)

	// Health Check
	protected("GET /api/v1/health", h.HealthCheck)

	// CV Processing
	protected("POST /api/v1/cvs", h.UploadCV)
	protected("GET /api/v1/cvs", h.ListCVs)
	protected("GET /api/v1/cvs/{id}", h.GetCV)
	protected("PUT /api/v1/cvs/{id}", h.UpdateCV)
	protected("DELETE /api/v1/cvs/{id}", h.DeleteCV)
	protected("POST /api/v1/cvs/{id}/embed", h.EmbedCV)
	protected("POST /api/v1/cvs/{id}/fetch", h.FetchJobsForCV)
	protected("POST /api/v1/cvs/{id}/retry", h.RetryCVProceassing)
	protected("POST /api/v1/cvs/{id}/match", h.EmbeddJobs)

	// Jobs
	protected("GET /api/v1/stream", h.StreamCVStatus)
	protected("GET /api/v1/jobs", h.FetchJobs)
	protected("POST /api/v1/jobs/{id}/embed", h.EmbedJob)

	// User preferences (target company size, drives auto-crawl)
	protected("GET /api/v1/preferences", h.GetPreferences)
	protected("PUT /api/v1/preferences", h.UpdatePreferences)

	// Deep evaluations (career-ops via jobbridge)
	protected("POST /api/v1/evaluations", h.RequestEvaluations)
	protected("GET /api/v1/evaluations/{cvId}/{jobId}", h.GetEvaluation)
	protected("GET /api/v1/evaluations/{cvId}/{jobId}/cv", h.GetEvaluationTailoredCv)

	// Auto-apply queue
	protected("POST /api/v1/applications", h.QueueApplications)
	protected("GET /api/v1/applications", h.ListApplications)
	protected("POST /api/v1/applications/{id}/approve", h.ApproveApplication)
	protected("POST /api/v1/applications/{id}/discard", h.DiscardApplication)
	protected("POST /api/v1/applications/{id}/inputs", h.UpdateApplicationInputs)
	protected("GET /api/v1/applications/{id}/screenshot", h.GetApplicationScreenshot)

	// Pipeline control
	protected("GET /api/v1/stats", h.PipelineStats)
	protected("GET /api/v1/scrape/tasks", h.ScrapeTasks)
	protected("POST /api/v1/scrape", h.TriggerScrape)
	protected("POST /api/v1/rerank/run", h.RunRerank)

	return mux
}
