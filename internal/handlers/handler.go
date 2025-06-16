package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"github.com/sh3lwan/jobhunter/internal/services"
	"github.com/sh3lwan/jobhunter/pkg/utils"
)

type Handler struct {
	cvService *services.CVService
	//http.Handler
}

func NewHandler(repo *repository.Queries, producer *mq.Producer) *Handler {
	return &Handler{
		cvService: services.NewCVService(repo, producer),
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("online"))
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
			// Pull latest CVs
			cvs, err := h.cvService.ListCVs(r.Context(), nil)
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

	ctx := r.Context()
	id, err := h.cvService.HandleCVUpload(ctx, header.Filename, data)
	if err != nil {
		http.Error(w, "Upload failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	utils.RespondJSON(w, http.StatusAccepted, map[string]string{
		"message": "File uploaded and processing started 🎯",
		"id":      strconv.FormatInt(id, 10),
	})
}

func (h *Handler) ListCVs(w http.ResponseWriter, r *http.Request) {
	cvs, err := h.cvService.ListCVs(r.Context(), nil)

	if err != nil {
		fmt.Println(err)
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"cvs": cvs,
	})
}

func (h *Handler) FetchJobs(w http.ResponseWriter, r *http.Request) {

	statuses := []string{"parsed", "analyzed"}
	cvs, err := h.cvService.ListCVs(r.Context(), statuses)

	if err != nil {
		fmt.Println(err)
		return
	}

	for _, cv := range cvs {
		h.cvService.Analyze(cv)
		break

	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"cvs": cvs,
	})
}
