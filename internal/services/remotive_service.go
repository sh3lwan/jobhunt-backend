package services

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/golang-module/carbon"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sh3lwan/jobhunter/internal/repository"
	. "github.com/sh3lwan/jobhunter/internal/repository"
)

type RemotiveService struct {
	JobService
}

func NewRemotiveService() *RemotiveService {
	srvc := JobService{
		BaseURL: "https://remotive.com/api/remote-jobs",
	}

	return &RemotiveService{srvc}
}

func (s *RemotiveService) Search(skills []string) ([]Job, error) {
	resp, err := s.call("search", strings.Join(skills[:1], ","))

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var data struct {
		Jobs []struct {
			ID          int      `json:"id"`
			URL         string   `json:"url"`
			Title       string   `json:"title"`
			Company     string   `json:"company_name"`
			Logo        string   `json:"company_logo"`
			Tags        []string `json:"tags"`
			Location    string   `json:"candidate_required_location"`
			Description string   `json:"description"`
			PublishDate string   `json:"publication_date"`
			JobType     string   `json:"job_type"`
		} `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var jobs []Job
	for _, j := range data.Jobs {
		publishts := parsePublishDate(j.PublishDate)
		jobs = append(jobs, Job{
			SourceID:    pgtype.Text{String: strconv.Itoa(j.ID), Valid: true},
			Source:      "remotive",
			Logo:        pgtype.Text{String: j.Logo, Valid: true},
			Title:       pgtype.Text{String: j.Title, Valid: true},
			Company:     pgtype.Text{String: j.Company, Valid: true},
			Location:    pgtype.Text{String: j.Location, Valid: true},
			Url:         pgtype.Text{String: j.URL, Valid: true},
			Tags:        j.Tags,
			Description: pgtype.Text{String: j.Description, Valid: true},
			PublishAt:   pgtype.Timestamptz{Time: publishts, Valid: true},
		})
	}
	return jobs, nil
}

func parsePublishDate(dateStr string) time.Time {
	// Try multiple parsing methods

	// Method 1: Use standard Go time parsing (most reliable)
	if t, err := time.Parse("2006-01-02T15:04:05", dateStr); err == nil {
		return t
	}

	// Method 2: Try with Carbon's smart parsing
	c := carbon.Parse(dateStr, "UTC")
	if !c.IsZero() {
		return c.ToStdTime()
	}

	// Method 3: Try different format variations
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	// If all parsing fails, return current time as fallback
	return time.Now()
}

func (s *RemotiveService) CollectJobs(q *repository.Queries, ctx context.Context, skills []string) ([]repository.Job, error) {
	return NewRemotiveService().Search(skills)
}
