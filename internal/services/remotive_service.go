package services

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/golang-module/carbon"
	"github.com/jackc/pgx/v5/pgtype"
	. "github.com/sh3lwan/jobhunter/internal/repository"
)

type RemotiveService struct {
	JobService
	url string
}

func NewRemotiveService() *RemotiveService {
	addr := "https://remotive.com/api/remote-jobs"

	return &RemotiveService{
		url: addr,
	}
}

func (s *RemotiveService) Search(skills []string) ([]Job, error) {
	skills = skills[:2]
	resp, err := s.call(s.url, "search", strings.Join(skills, ","))

	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

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
	c := carbon.ParseByFormat(dateStr, "2006-01-02T15:04:05") // parse exactly
	if c.IsZero() {
		return time.Time{} // zero time
	}
	c.SetTimezone("UTC") // set timezone explicitly
	return c.ToStdTime()
}
