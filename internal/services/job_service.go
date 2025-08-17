package services

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/sh3lwan/jobhunter/internal/repository"
)

type JobService struct {
	BaseURL string
}

func NewJobService(baseURL string) *JobService {
	return &JobService{
		BaseURL: baseURL,
	}
}

type Searchable interface {
	Search(skills string) ([]repository.Job, error)
	CollectJobs(q *repository.Queries, ctx context.Context) ([]repository.Job, error)
	SaveJobs(q *repository.Queries, jobs []repository.Job) error
}

func (s *JobService) call(key string, params string) (*http.Response, error) {
	q := url.Values{}
	q.Set(key, params)
	fullURL := s.BaseURL + "?" + removeQuotesFromURL(q.Encode())
	return http.Get(fullURL)
}

func removeQuotesFromURL(rawURL string) string {
	return strings.ReplaceAll(rawURL, "%22", "")
}
