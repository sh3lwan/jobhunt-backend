package services

import (
	"context"
	"fmt"

	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

type DBJobService struct {
	queries *repository.Queries
}

func NewDBJobService(queries *repository.Queries) *DBJobService {
	return &DBJobService{
		queries: queries,
	}
}

func (s *DBJobService) ListJobs(ctx context.Context, limit, offset int32) ([]models.Job, error) {
	jobs, err := s.queries.GetAllJobs(ctx, repository.GetAllJobsParams{
		Limit:  limit,
		Offset: offset,
	})

	if err != nil {
		return nil, fmt.Errorf("❌ Failed to list jobs: %v", err)
	}

	var jobList = make([]models.Job, 0, len(jobs))

	for _, job := range jobs {
		jobList = append(jobList, models.Job{
			ID:          job.ID,
			SourceID:    job.SourceID.String,
			Source:      job.Source,
			Title:       job.Title.String,
			Company:     job.Company.String,
			Logo:        job.Logo.String,
			Location:    job.Location.String,
			Url:         job.Url.String,
			Tags:        job.Tags,
			Description: job.Description.String,
			PublishAt:   job.PublishAt.Time,
			CreatedAt:   job.CreatedAt.Time,
		})
	}

	return jobList, nil
}

func (s *DBJobService) SaveJobs(q *repository.Queries, ctx context.Context, jobs []repository.Job) error {
	for _, job := range jobs {
		err := q.CreateJob(ctx, repository.CreateJobParams{
			SourceID:    job.SourceID,
			Title:       job.Title,
			Source:      job.Source,
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
