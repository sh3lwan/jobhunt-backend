package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sh3lwan/jobhunter/internal/repository"
)

// RerankService sends the strongest un-reranked matches to the parser's LLM
// rerank endpoint and stores the recruiter-rubric assessment. Scores from the
// reranker take precedence over the embedding percentage in listings.
type RerankService struct {
	queries   *repository.Queries
	parserURL string
	client    *http.Client
}

type rerankAssessment struct {
	FitScore     int      `json:"fit_score"`
	SeniorityFit string   `json:"seniority_fit"`
	Strengths    []string `json:"strengths"`
	Gaps         []string `json:"gaps"`
	Explanation  string   `json:"explanation"`
}

func NewRerankService(queries *repository.Queries, parserURL string) *RerankService {
	return &RerankService{
		queries:   queries,
		parserURL: parserURL,
		// One LLM call per request; local inference can take tens of seconds,
		// and the first call after idle also pays model-load time.
		client: &http.Client{Timeout: 300 * time.Second},
	}
}

// RerankPending scores up to limit stale top matches, one LLM call each.
// Returns the number successfully reranked.
func (s *RerankService) RerankPending(ctx context.Context, limit int32) (int, error) {
	matches, err := s.queries.GetMatchesNeedingRerank(ctx, limit)

	if err != nil {
		return 0, fmt.Errorf("failed to list matches needing rerank: %w", err)
	}

	done := 0

	for _, match := range matches {
		if ctx.Err() != nil {
			return done, ctx.Err()
		}

		assessment, err := s.rerankOne(ctx, match)

		if err != nil {
			// Stop the whole batch on a request failure: a timeout means the
			// parser/LLM is struggling, and firing more requests at it only
			// piles up zombie generations. The next tick retries.
			log.Printf("Rerank failed for cv %d job %d (stopping batch): %v", match.CvID, match.JobID, err)
			return done, nil
		}

		details, err := json.Marshal(map[string]any{
			"seniority_fit": assessment.SeniorityFit,
			"strengths":     assessment.Strengths,
			"gaps":          assessment.Gaps,
			"explanation":   assessment.Explanation,
		})

		if err != nil {
			log.Printf("Failed to marshal rerank details for cv %d job %d: %v", match.CvID, match.JobID, err)
			continue
		}

		var score pgtype.Numeric
		if err := score.Scan(fmt.Sprintf("%d", assessment.FitScore)); err != nil {
			log.Printf("Invalid rerank score for cv %d job %d: %v", match.CvID, match.JobID, err)
			continue
		}

		if err := s.queries.UpdateMatchRerank(ctx, repository.UpdateMatchRerankParams{
			CvID:          match.CvID,
			JobID:         match.JobID,
			RerankScore:   score,
			RerankDetails: details,
		}); err != nil {
			log.Printf("Failed to store rerank for cv %d job %d: %v", match.CvID, match.JobID, err)
			continue
		}

		done++
	}

	return done, nil
}

func (s *RerankService) rerankOne(ctx context.Context, match repository.GetMatchesNeedingRerankRow) (*rerankAssessment, error) {
	payload, err := json.Marshal(map[string]any{
		"cv_canonical": match.CvCanonical.String,
		"cv_skills":    match.CvSkills.String,
		"job": map[string]any{
			"title":       match.Title.String,
			"company":     match.Company.String,
			"location":    match.Location.String,
			"tags":        match.Tags,
			"description": match.Description.String,
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.parserURL+"/api/v1/rerank", bytes.NewReader(payload))

	if err != nil {
		return nil, fmt.Errorf("failed to build rerank request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("rerank request failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rerank endpoint returned status %d", resp.StatusCode)
	}

	var assessment rerankAssessment

	if err := json.NewDecoder(resp.Body).Decode(&assessment); err != nil {
		return nil, fmt.Errorf("failed to decode rerank response: %w", err)
	}

	return &assessment, nil
}
