package services

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

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

	values := url.Values{}
	values.Add(key, params)
	fullURL := s.BaseURL + "?" + q.Encode()
	// Set a timeout for the HTTP request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the GET request
	return client.Get(fullURL)
}

func removeQuotesFromURL(rawURL string) string {
	return strings.ReplaceAll(rawURL, "%22", "")
}
