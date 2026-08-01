package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/sh3lwan/jobhunter/internal/repository"
	"github.com/sh3lwan/jobhunter/pkg/utils"
)

// RequestEvaluations queues deep career-ops evaluations for the given jobs.
// The jobbridge watch worker picks them up, runs the A-G evaluation via
// claude -p, and (for strong scores) generates a tailored CV PDF.
func (h *Handler) RequestEvaluations(w http.ResponseWriter, r *http.Request) {
	if _, err := utils.GetUserFromContext(r.Context()); err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var body struct {
		CvID   int64   `json:"cv_id"`
		JobIDs []int32 `json:"job_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CvID == 0 || len(body.JobIDs) == 0 {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "cv_id and at least one job_id are required"})
		return
	}

	requested := 0
	for _, jobID := range body.JobIDs {
		rows, err := h.queries.RequestJobEvaluation(r.Context(), repository.RequestJobEvaluationParams{
			CvID:  body.CvID,
			JobID: jobID,
		})
		if err != nil {
			continue
		}
		// rows == 0 means an evaluation is already requested/running — fine.
		requested += int(rows)
	}

	utils.RespondJSON(w, http.StatusAccepted, map[string]any{
		"requested": requested,
		"message":   "Evaluations queued — jobbridge will process them shortly.",
	})
}

// GetEvaluation returns the stored evaluation for a CV+job pair, including the
// full report markdown when the report file is readable from this host.
func (h *Handler) GetEvaluation(w http.ResponseWriter, r *http.Request) {
	if _, err := utils.GetUserFromContext(r.Context()); err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	cvID, err1 := strconv.ParseInt(r.PathValue("cvId"), 10, 64)
	jobID, err2 := strconv.ParseInt(r.PathValue("jobId"), 10, 32)
	if err1 != nil || err2 != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cv or job id"})
		return
	}

	eval, err := h.queries.GetJobEvaluation(r.Context(), repository.GetJobEvaluationParams{
		CvID:  cvID,
		JobID: int32(jobID),
	})
	if err != nil {
		utils.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "no evaluation for this cv/job"})
		return
	}

	resp := map[string]any{
		"cv_id":          eval.CvID,
		"job_id":         eval.JobID,
		"status":         eval.Status,
		"evaluator":      eval.Evaluator,
		"model":          eval.Model.String,
		"final_decision": eval.FinalDecision.String,
	}
	if eval.Score.Valid {
		if v, err := eval.Score.Float64Value(); err == nil {
			resp["score"] = v.Float64
		}
	}
	if len(eval.MachineSummary) > 0 {
		resp["machine_summary"] = json.RawMessage(eval.MachineSummary)
	}
	if eval.Error.Valid && eval.Error.String != "" {
		resp["error"] = eval.Error.String
	}
	if eval.EvaluatedAt.Valid {
		resp["evaluated_at"] = eval.EvaluatedAt.Time
	}
	if eval.TailoredCvPath.Valid && eval.TailoredCvPath.String != "" {
		resp["has_tailored_cv"] = true
	}
	// The report lives on the jobbridge host's filesystem; in local/single-host
	// deployments that's this machine, so inline it when readable.
	if eval.ReportPath.Valid && eval.ReportPath.String != "" {
		if md, err := os.ReadFile(eval.ReportPath.String); err == nil {
			resp["report_markdown"] = string(md)
		}
	}

	utils.RespondJSON(w, http.StatusOK, resp)
}

// GetEvaluationTailoredCv streams the tailored CV PDF generated for this pair.
func (h *Handler) GetEvaluationTailoredCv(w http.ResponseWriter, r *http.Request) {
	if _, err := utils.GetUserFromContext(r.Context()); err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	cvID, err1 := strconv.ParseInt(r.PathValue("cvId"), 10, 64)
	jobID, err2 := strconv.ParseInt(r.PathValue("jobId"), 10, 32)
	if err1 != nil || err2 != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cv or job id"})
		return
	}

	eval, err := h.queries.GetJobEvaluation(r.Context(), repository.GetJobEvaluationParams{
		CvID:  cvID,
		JobID: int32(jobID),
	})
	if err != nil || !eval.TailoredCvPath.Valid || eval.TailoredCvPath.String == "" {
		utils.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "no tailored CV for this cv/job"})
		return
	}

	pdf, err := os.ReadFile(eval.TailoredCvPath.String)
	if err != nil {
		utils.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "tailored CV file not readable"})
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=tailored-cv.pdf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}
