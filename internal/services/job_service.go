package services

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sh3lwan/jobhunter/internal/repository"
)

type JobService struct {
}

type Searchable interface {
	Search(skills string) ([]repository.Job, error)
}

func (s *JobService) call(baseUrl string, key string, params string) (*http.Response, error) {

	// Add query params dynamically
	q := url.Values{}
	q.Set(key, params)
	fullURL := baseUrl + "?" + removeQuotesFromURL(q.Encode())
	fmt.Println(fullURL)
	return http.Get(fullURL)
}

func removeQuotesFromURL(rawURL string) string {
	return strings.ReplaceAll(rawURL, "%22", "")
}

func CollectAndSaveJobs(q *repository.Queries, ctx context.Context) error {

	cvService := NewCVService(q, nil)

	skills, err := cvService.GetSkills(ctx)

	if err != nil {
		return err
	}

	remotive := NewRemotiveService()

	jobs, err := remotive.Search(skills)

	for _, job := range jobs {
		err := q.CreateJob(ctx, repository.CreateJobParams{
			SourceID:    job.SourceID,
			Title:       job.Title,
			Company:     job.Company,
			Url:         job.Url,
			Description: job.Description,
			Tags:        job.Tags,
		})

		if err != nil {
			return fmt.Errorf("❌ Failed to insert job %s: %v", job.Title.String, err)
		}
	}
	return nil
}
