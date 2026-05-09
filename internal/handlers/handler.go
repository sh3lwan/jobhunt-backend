package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
}

func NewHandler(queries *repository.Queries, producer *mq.Producer, authService *services.AuthService) *Handler {
	return &Handler{
		cvService:        services.NewCVService(queries, producer),
		dbJobService:     services.NewDBJobService(queries),
		embeddingService: services.NewEmbeddingService(producer),
		authService:      authService,
	}
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

func (h *Handler) FetchJobs(w http.ResponseWriter, r *http.Request) {
	// Fetch jobs from db
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10" // Default limit
	}

	// Convert limit to int
	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid limit parameter",
		})
		return
	}

	offset := r.URL.Query().Get("offset")
	if offset == "" {
		offset = "0" // Default offset
	}
	// If offset is provided, you can use it in the query
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Limit must be greater than 0." + err.Error(),
		})
		return
	}

	// Fetch jobs from the database
	jobs, err := h.dbJobService.ListJobs(r.Context(), int32(limitInt), int32(offsetInt))

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
		// Send the job data for embedding
		err = h.embeddingService.SendEmbeddingRequest(string(cv.ID), cv.ParsedText.String, models.CVEmbeddingType)

		if err != nil {
			h.cvService.HandleCVError(r.Context(), cv.ID, err)
			utils.RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"status": "success", "msg": "CV processing retried"})

}

func (h *Handler) EmbeddJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.dbJobService.ListJobs(r.Context(), 100, 0)

	jobs = jobs[:1] // For testing purposes, limit to 10 jobs

	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	//go func() {

	for _, job := range jobs {
		// Convert job to JSON
		data, err := json.Marshal(job)
		if err != nil {
			utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "Failed to marshal job: " + err.Error(),
			})
			return
		}

		err = h.embeddingService.SendEmbeddingRequest(string(job.ID), string(data), models.JobEmbeddingType)

		if err != nil {
			log.Println("Failed to send embedding request for job ID", job.ID, ":", err)
			continue
		}

		log.Println("Sent job for embedding:", job.ID, job.Title)
	}
	//}()

	utils.RespondJSON(w, http.StatusOK, map[string]string{
		"message": "Jobs embedded initiated successfully",
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

	err = h.embeddingService.SendEmbeddingRequest(string(job.ID), string(data), models.JobEmbeddingType)

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

	err = h.embeddingService.SendEmbeddingRequest(string(cvId), cv.ParsedText.String, models.CVEmbeddingType)

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
