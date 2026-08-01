package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh3lwan/jobhunter/internal/repository"
	"github.com/sh3lwan/jobhunter/pkg/utils"
)

// QueueApplications adds the selected jobs to the auto-apply queue. The
// jobapplier worker picks them up, fills the forms, and pauses for approval.
func (h *Handler) QueueApplications(w http.ResponseWriter, r *http.Request) {
	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var body struct {
		CvID   int64   `json:"cv_id"`
		JobIDs []int64 `json:"job_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}
	if len(body.JobIDs) == 0 {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one job_id is required"})
		return
	}

	cv := pgtype.Int8{}
	if body.CvID > 0 {
		cv = pgtype.Int8{Int64: body.CvID, Valid: true}
	}

	queued := 0
	for _, jobID := range body.JobIDs {
		if err := h.queries.QueueApplication(r.Context(), repository.QueueApplicationParams{
			UserID: user.ID,
			CvID:   cv,
			JobID:  jobID,
		}); err != nil {
			// A single failure (e.g. bad job id) shouldn't abort the batch.
			continue
		}
		queued++
	}

	utils.RespondJSON(w, http.StatusAccepted, map[string]any{
		"queued":  queued,
		"message": "Applications queued — the applier will fill them and pause for your approval.",
	})
}

type applicationView struct {
	ID           int64           `json:"id"`
	JobID        int64           `json:"job_id"`
	CvID         *int64          `json:"cv_id,omitempty"`
	Status       string          `json:"status"`
	Title        string          `json:"title"`
	Company      string          `json:"company"`
	Location     string          `json:"location"`
	URL          string          `json:"url"`
	Source       string          `json:"source"`
	FilledInputs json.RawMessage `json:"filled_inputs,omitempty"`
	Error        string          `json:"error,omitempty"`
	SubmittedAt  *time.Time      `json:"submitted_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	// True when the applier captured a snapshot at submit time. The image bytes
	// are fetched lazily via GET /applications/{id}/screenshot to keep this list
	// small.
	HasSubmissionScreenshot bool `json:"has_submission_screenshot"`
	HasResultScreenshot     bool `json:"has_result_screenshot"`
}

// ListApplications returns the user's applications with job details, plus a
// count of those awaiting approval (the in-dashboard notification signal).
func (h *Handler) ListApplications(w http.ResponseWriter, r *http.Request) {
	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntOrDefault(r.URL.Query().Get("offset"), 0)

	rows, err := h.queries.ListApplications(r.Context(), repository.ListApplicationsParams{
		UserID: user.ID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	awaiting, err := h.queries.CountAwaitingApproval(r.Context(), user.ID)
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	out := make([]applicationView, 0, len(rows))
	for _, a := range rows {
		v := applicationView{
			ID:        a.ID,
			JobID:     a.JobID,
			Status:    a.Status,
			Title:     a.Title.String,
			Company:   a.Company.String,
			Location:  a.Location.String,
			URL:       a.Url.String,
			Source:    a.Source,
			CreatedAt: a.CreatedAt.Time,
			UpdatedAt: a.UpdatedAt.Time,
		}
		if a.CvID.Valid {
			v.CvID = &a.CvID.Int64
		}
		// sqlc types the `IS NOT NULL` expressions as interface{} — coerce to bool.
		if b, ok := a.HasSubmissionScreenshot.(bool); ok {
			v.HasSubmissionScreenshot = b
		}
		if b, ok := a.HasResultScreenshot.(bool); ok {
			v.HasResultScreenshot = b
		}
		if len(a.FilledInputs) > 0 {
			v.FilledInputs = json.RawMessage(a.FilledInputs)
		}
		if a.Error.Valid {
			v.Error = a.Error.String
		}
		if a.SubmittedAt.Valid {
			t := a.SubmittedAt.Time
			v.SubmittedAt = &t
		}
		out = append(out, v)
	}

	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"applications":      out,
		"awaiting_approval": awaiting,
	})
}

// ApproveApplication authorizes submission. Only valid from awaiting_approval;
// the worker then submits and records the confirmation.
func (h *Handler) ApproveApplication(w http.ResponseWriter, r *http.Request) {
	h.transitionApplication(w, r, true)
}

// DiscardApplication drops an application before it is submitted.
func (h *Handler) DiscardApplication(w http.ResponseWriter, r *http.Request) {
	h.transitionApplication(w, r, false)
}

// UpdateApplicationInputs saves the user's edited field values before approval.
// Only valid while awaiting_approval; the applier replays exactly these values
// when it submits, so what the user approves is what gets sent.
func (h *Handler) UpdateApplicationInputs(w http.ResponseWriter, r *http.Request) {
	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid application id"})
		return
	}

	var body struct {
		FilledInputs json.RawMessage `json:"filled_inputs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}
	// Must be a JSON array of field objects — reject anything else so we never
	// store malformed inputs the applier can't replay.
	var probe []json.RawMessage
	if len(body.FilledInputs) == 0 || json.Unmarshal(body.FilledInputs, &probe) != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "filled_inputs must be a JSON array"})
		return
	}

	rows, err := h.queries.UpdateApplicationInputs(r.Context(), repository.UpdateApplicationInputsParams{
		ID:           id,
		UserID:       user.ID,
		FilledInputs: []byte(body.FilledInputs),
	})
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rows == 0 {
		utils.RespondJSON(w, http.StatusConflict, map[string]string{"error": "application is no longer editable"})
		return
	}

	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": "Saved"})
}

// GetApplicationScreenshot streams a PNG the applier captured at submit time.
// `?type=submission` (default) is the filled form that was sent — including which
// CV was attached; `?type=result` is the outcome page (success/error) after the
// Submit click. Owner-scoped; 404 if nothing was captured.
func (h *Handler) GetApplicationScreenshot(w http.ResponseWriter, r *http.Request) {
	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid application id"})
		return
	}

	var img []byte
	switch r.URL.Query().Get("type") {
	case "result":
		img, err = h.queries.GetResultScreenshot(r.Context(), repository.GetResultScreenshotParams{ID: id, UserID: user.ID})
	default: // "submission" or empty
		img, err = h.queries.GetSubmissionScreenshot(r.Context(), repository.GetSubmissionScreenshotParams{ID: id, UserID: user.ID})
	}
	if err != nil || len(img) == 0 {
		utils.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "no screenshot for this application"})
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(img)
}

func (h *Handler) transitionApplication(w http.ResponseWriter, r *http.Request, approve bool) {
	user, err := utils.GetUserFromContext(r.Context())
	if err != nil {
		utils.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		utils.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid application id"})
		return
	}

	var rows int64
	if approve {
		rows, err = h.queries.ApproveApplication(r.Context(), repository.ApproveApplicationParams{ID: id, UserID: user.ID})
	} else {
		rows, err = h.queries.DiscardApplication(r.Context(), repository.DiscardApplicationParams{ID: id, UserID: user.ID})
	}
	if err != nil {
		utils.RespondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rows == 0 {
		utils.RespondJSON(w, http.StatusConflict, map[string]string{"error": "application not in a state that allows this action"})
		return
	}

	// On approval, remember the reusable answers so future fills reuse them.
	// Best-effort: a learning failure must not fail the approval.
	if approve {
		if app, gerr := h.queries.GetApplication(r.Context(), id); gerr == nil {
			h.learnAnswers(r.Context(), user.ID, app.FilledInputs)
		}
	}

	msg := "Application discarded"
	if approve {
		msg = "Approved — the applier will submit it shortly"
	}
	utils.RespondJSON(w, http.StatusOK, map[string]string{"message": msg})
}

// learnAnswers stores the reusable field values from an approved application so
// future fills reuse them (matched by label). Only short, stable facts are
// kept — tailored free-text (cover letters, "why this company") is excluded so
// job-specific answers never leak across applications. Best-effort by design.
func (h *Handler) learnAnswers(ctx context.Context, userID int64, raw []byte) {
	if len(raw) == 0 {
		return
	}
	var inputs []struct {
		Label   string `json:"label"`
		Value   string `json:"value"`
		Type    string `json:"type"`
		Skipped bool   `json:"skipped"`
	}
	if json.Unmarshal(raw, &inputs) != nil {
		return
	}
	for _, in := range inputs {
		if in.Skipped {
			continue
		}
		v := strings.TrimSpace(in.Value)
		// Skip empties and anything essay-like: reused verbatim, a tailored
		// paragraph would be wrong on the next job.
		if v == "" || in.Type == "textarea" || len([]rune(v)) > 120 {
			continue
		}
		label := normalizeLabel(in.Label)
		if label == "" {
			continue
		}
		_ = h.queries.UpsertLearnedAnswer(ctx, repository.UpsertLearnedAnswerParams{
			UserID: userID,
			Label:  label,
			Value:  v,
		})
	}
}

// normalizeLabel lower-cases and strips trailing markers ("*", ":", "?") so the
// same field matches across forms regardless of required-asterisks or wording.
func normalizeLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimRight(s, " *:?")
	return strings.TrimSpace(s)
}
