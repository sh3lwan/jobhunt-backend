package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"github.com/sh3lwan/jobhunter/internal/services"
	"github.com/sh3lwan/jobhunter/pkg/utils"
)

type Handler struct {
	cvService        *services.CVService
	dbJobService     *services.DBJobService
	embeddingService *services.EmbeddingService
	authService      *services.AuthService
	rerankService    *services.RerankService
	scrapeService    *services.ScrapeService
	googleService    *services.GoogleOAuthService
	queries          *repository.Queries
}

func NewHandler(
	queries *repository.Queries,
	producer *mq.Producer,
	authService *services.AuthService,
	rerankService *services.RerankService,
	scrapeService *services.ScrapeService,
	googleService *services.GoogleOAuthService,
) *Handler {
	return &Handler{
		cvService:        services.NewCVService(queries, producer),
		dbJobService:     services.NewDBJobService(queries),
		embeddingService: services.NewEmbeddingService(producer, queries),
		authService:      authService,
		rerankService:    rerankService,
		scrapeService:    scrapeService,
		googleService:    googleService,
		queries:          queries,
	}
}

const googleStateCookie = "g_oauth_state"

// GoogleAuthStart redirects the browser to Google's consent screen. If Google
// OAuth isn't configured, it bounces back to the frontend login with an error
// so the button degrades cleanly instead of hitting a dead route.
func (h *Handler) GoogleAuthStart(w http.ResponseWriter, r *http.Request) {
	if !h.googleService.Configured() {
		http.Redirect(w, r, h.googleService.FrontendURL()+"/login?error=google_not_configured", http.StatusFound)
		return
	}

	state, err := h.googleService.NewState()
	if err != nil {
		http.Redirect(w, r, h.googleService.FrontendURL()+"/login?error=google_failed", http.StatusFound)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     googleStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.googleService.AuthCodeURL(state), http.StatusFound)
}

// GoogleAuthCallback completes the flow and redirects to the frontend with a
// token fragment the SPA stores in localStorage.
func (h *Handler) GoogleAuthCallback(w http.ResponseWriter, r *http.Request) {
	frontend := h.googleService.FrontendURL()

	fail := func(reason string) {
		http.Redirect(w, r, frontend+"/login?error="+reason, http.StatusFound)
	}

	query := r.URL.Query()

	if query.Get("error") != "" {
		fail("google_denied")
		return
	}

	// CSRF: state must match the cookie set at start.
	cookie, err := r.Cookie(googleStateCookie)
	if err != nil || cookie.Value == "" || cookie.Value != query.Get("state") {
		fail("google_state_mismatch")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: googleStateCookie, Value: "", Path: "/", MaxAge: -1})

	code := query.Get("code")
	if code == "" {
		fail("google_no_code")
		return
	}

	token, err := h.googleService.HandleCallback(r.Context(), code)
	if err != nil {
		fail("google_failed")
		return
	}

	// Token in the URL fragment: never sent to the server, read client-side.
	http.Redirect(w, r, frontend+"/auth/callback#token="+token, http.StatusFound)
}

// PipelineStats powers the dashboard KPIs and the Sources page.
func (h *Handler) PipelineStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.queries.GetPipelineStats(r.Context())

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sources, err := h.queries.GetJobsBySource(r.Context())

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type sourceStat struct {
		Source        string    `json:"source"`
		Total         int64     `json:"total"`
		Embedded      int64     `json:"embedded"`
		LastCollected time.Time `json:"last_collected"`
	}

	sourceStats := make([]sourceStat, 0, len(sources))
	for _, s := range sources {
		sourceStats = append(sourceStats, sourceStat{
			Source:        s.Source,
			Total:         s.Total,
			Embedded:      s.Embedded,
			LastCollected: s.LastCollected.Time,
		})
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"total_jobs":       stats.TotalJobs,
		"embedded_jobs":    stats.EmbeddedJobs,
		"total_matches":    stats.TotalMatches,
		"reranked_matches": stats.RerankedMatches,
		"total_cvs":        stats.TotalCvs,
		"analyzed_cvs":     stats.AnalyzedCvs,
		"jobs_last_24h":    stats.JobsLast24h,
		"sources":          sourceStats,
	})
}

// ScrapeTasks lists recent scraping task records.
func (h *Handler) ScrapeTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.queries.GetRecentScrapeTasks(r.Context(), 25)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type task struct {
		TaskID    string    `json:"task_id"`
		Platform  string    `json:"platform"`
		Skills    []string  `json:"skills"`
		Location  string    `json:"location"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	out := make([]task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, task{
			TaskID:    t.TaskID,
			Platform:  t.Platform,
			Skills:    t.Skills,
			Location:  t.Location.String,
			Status:    t.Status,
			CreatedAt: t.CreatedAt.Time,
			UpdatedAt: t.UpdatedAt.Time,
		})
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{"tasks": out})
}

// TriggerScrape enqueues an on-demand scraping run.
func (h *Handler) TriggerScrape(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform string   `json:"platform"`
		Skills   []string `json:"skills"`
		Location string   `json:"location"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	validPlatforms := map[string]bool{"greenhouse": true, "remotive": true, "linkedin": true}

	if !validPlatforms[req.Platform] {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "platform must be one of: greenhouse, remotive, linkedin"})
		return
	}

	if len(req.Skills) == 0 {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one skill is required"})
		return
	}

	taskID, err := h.scrapeService.Dispatch(req.Platform, req.Skills, req.Location)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.RespondJSON(w, http.StatusAccepted, map[string]string{
		"task_id": taskID,
		"message": "Scrape queued",
	})
}

// RunRerank scores a batch of pending matches on demand (the scheduler also
// does this every 2 minutes; this endpoint exists for the dashboard button).
func (h *Handler) RunRerank(w http.ResponseWriter, r *http.Request) {
	done, err := h.rerankService.RerankPending(r.Context(), 6)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"reranked": done,
		"message":  fmt.Sprintf("Reranked %d matches", done),
	})
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	conn, err := kafka.Dial("tcp", os.Getenv("KAFKA_BROKER"))

	if err != nil {
		utils.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "error",
			"error":  "Kafka is unreachable: " + err.Error(),
		})
		return
	}

	conn.Close()

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"msg":    "Service is healthy",
	})
}

func (h *Handler) StreamCVStatus(w http.ResponseWriter, r *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Prevent client timeout (optional)
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Poll every X seconds or hook with Kafka later
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			user, err := utils.GetUserFromContext(r.Context())

			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				continue
			}
			cvs, err := h.cvService.ListCVs(r.Context(), nil, user.ID)
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				continue
			}

			jsonData, err := json.Marshal(cvs)
			if err != nil {
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
				flusher.Flush()
				continue
			}

			// Stream it
			fmt.Fprintf(w, "data: %s\n\n", jsonData)
			flusher.Flush()
		}
	}
}

func (h *Handler) UploadCV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("cv")
	if err != nil {
		http.Error(w, "Missing file in request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	id, err := h.cvService.HandleCVUpload(
		r.Context(),
		header.Filename,
		data,
	)

	if err != nil {
		// Record the error in the database
		h.cvService.HandleCVError(r.Context(), id, err)

		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to upload CV: " + err.Error(),
		})
		return
	}

	utils.RespondJSON(w, http.StatusAccepted, map[string]string{
		"message": "File uploaded and processing started 🎯",
		"id":      strconv.FormatInt(id, 10),
	})
}

func (h *Handler) ListCVs(w http.ResponseWriter, r *http.Request) {
	user, err := utils.GetUserFromContext(r.Context())

	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Unauthorized: " + err.Error(),
		})
		return
	}

	cvs, err := h.cvService.ListCVs(r.Context(), nil, user.ID)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	if len(cvs) == 0 {
		utils.RespondJSON(w, http.StatusOK, map[string]any{
			"cvs": []repository.CvAnalysis{},
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"cvs": cvs,
	})
}

// GetCV returns a single CV analysis, scoped to the requesting user. Lets the
// detail page load on direct navigation instead of relying on in-memory state.
func (h *Handler) GetCV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid CV id"})
		return
	}

	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	cv, err := h.cvService.GetCV(r.Context(), id)
	if err != nil {
		utils.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "CV not found"})
		return
	}

	if cv.UserID.Valid && cv.UserID.Int64 != user.ID {
		utils.RespondJSON(w, http.StatusForbidden, map[string]string{"error": "Not your CV"})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{"cv": cv})
}

// UpdateCV persists a user edit of the parsed CV JSON. The request body is the
// edited structured CV object; it replaces structured_json verbatim.
func (h *Handler) UpdateCV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid CV id"})
		return
	}

	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read body"})
		return
	}

	// Validate it's a JSON object before storing.
	var probe map[string]any
	if err := json.Unmarshal(body, &probe); err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Body must be a JSON object"})
		return
	}

	if err := h.cvService.UpdateStructuredJSON(r.Context(), id, user.ID, body); err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "CV updated"})
}

func (h *Handler) FetchJobs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := parseIntOrDefault(query.Get("limit"), 20)
	offset := parseIntOrDefault(query.Get("offset"), 0)

	filter := services.MatchedJobsFilter{
		Search: query.Get("q"),
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if sources := query.Get("source"); sources != "" {
		for _, s := range strings.Split(sources, ",") {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				filter.Sources = append(filter.Sources, trimmed)
			}
		}
	}

	if minMatch := query.Get("min_match"); minMatch != "" {
		if v, err := strconv.ParseFloat(minMatch, 64); err == nil {
			filter.MinPercentage = &v
		}
	}

	// Scope matches to the requested CV, defaulting to the user's most
	// recently analyzed one.
	if cvParam := query.Get("cv_id"); cvParam != "" {
		if id, err := strconv.ParseInt(cvParam, 10, 64); err == nil {
			filter.CvID = id
		}
	} else if user, err := utils.GetUserFromContext(r.Context()); err == nil {
		if id, err := h.dbJobService.LatestAnalyzedCVId(r.Context(), user.ID); err == nil {
			filter.CvID = id
		}
	}

	jobs, total, err := h.dbJobService.ListMatchedJobs(r.Context(), filter)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"jobs":   jobs,
		"total":  total,
		"cv_id":  filter.CvID,
		"limit":  limit,
		"offset": offset,
	})

}

func parseIntOrDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func (h *Handler) FetchRemotiveJobs(w http.ResponseWriter, r *http.Request) {

	skills, err := h.cvService.GetSkills(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	remotive := services.NewRemotiveService()

	jobs, err := remotive.Search(skills)
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"jobs": jobs,
	})
}

func (h *Handler) FetchJobsForCV(w http.ResponseWriter, r *http.Request) {
	cv_id := r.PathValue("id")

	id, err := strconv.ParseInt(cv_id, 10, 64)

	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Invalid CV id"})
		return
	}

	skills, err := h.cvService.GetSkillsForCV(r.Context(), id)

	//utils.RespondJSON(w, http.StatusOK, map[string]string{"status": "success", "msg": "Skills fetched", "skills": strings.Join(skills, ",")})
	//return
	//go h.cvService.FetchJobs(skills)

	//utils.RespondJSON(w, http.StatusOK, map[string]string{"status": "succes", "msg": "Fetch job started"})
	remotive := services.NewRemotiveService()

	jobs, err := remotive.Search(skills)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Save jobs to DB
	err = h.dbJobService.SaveJobs(r.Context(), jobs)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"jobs": jobs,
	})
}

func (h *Handler) RetryCVProceassing(w http.ResponseWriter, r *http.Request) {
	cv_id := r.PathValue("id")

	id, err := strconv.ParseInt(cv_id, 10, 64)

	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Invalid CV id"})
		return
	}

	cv, err := h.cvService.GetCV(r.Context(), id)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
		return
	}

	if cv.Status == "uploaded" {
		_, err = h.cvService.Parse(r.Context(), cv)

		if err != nil {
			h.cvService.HandleCVError(r.Context(), cv.ID, err)
			utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"status": "error", "message": err.Error()})
			return
		}
	}

	if cv.Status == "parsed" {
		err = h.cvService.Analyze(cv)
		if err != nil {
			h.cvService.HandleCVError(r.Context(), cv.ID, err)
			utils.RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if cv.Status == "analyzed" {
		// Re-run the analysis pipeline; Analyze builds the {text, links}
		// envelope the parser expects from the stored parsed_text.
		err = h.cvService.Analyze(cv)

		if err != nil {
			h.cvService.HandleCVError(r.Context(), cv.ID, err)
			utils.RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"status": "success", "msg": "CV processing retried"})

}

func (h *Handler) EmbeddJobs(w http.ResponseWriter, r *http.Request) {
	count, err := h.embeddingService.RequestMissingJobEmbeddings(r.Context(), 100)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Embedding requested for %d jobs", count),
	})

}

func (h *Handler) EmbedJob(w http.ResponseWriter, r *http.Request) {
	job_id := r.PathValue("id")

	id, err := strconv.ParseInt(job_id, 10, 32)

	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"status": "error", "message": "Invalid job id"})
		return
	}
	fmt.Println(int32(id))

	job, err := h.dbJobService.GetJobById(r.Context(), int32(id))

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Convert job to JSON
	data, err := json.Marshal(job)
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to marshal job: " + err.Error(),
		})
		return
	}

	err = h.embeddingService.SendEmbeddingRequest(strconv.FormatInt(int64(job.ID), 10), string(data), models.JobEmbeddingType)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to send embedding request: " + err.Error(),
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Jobs embedded successfully",
	})

}

func (h *Handler) Authenticate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&creds)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	u, err := h.authService.ValidateUser(r.Context(), creds.Username, creds.Password)

	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{
			"error": err.Error(),
		})
		return
	}

	token, err := h.authService.Generate(u)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Failed to generate token",
		})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

func (h *Handler) EmbedCV(w http.ResponseWriter, r *http.Request) {
	cvId := r.PathValue("id")

	id, err := strconv.ParseInt(cvId, 10, 64)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	cv, err := h.cvService.GetCV(r.Context(), id)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	// Analyze builds the {text, links} envelope the parser expects — sending
	// raw parsed_text here would arrive with the wrong field names and be
	// silently dropped by the worker.
	err = h.cvService.Analyze(cv)

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return

	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "CV analysis happening now",
	})
}
