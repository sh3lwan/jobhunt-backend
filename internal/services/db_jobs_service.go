package services

import (
	"context"
	"fmt"
	"sort"

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

	cvs, err := s.queries.GetAllCVAnalysis(ctx, repository.GetAllCVAnalysisParams{
		Limit:  1,
		Offset: 0,
	})

	if err != nil {
		return nil, fmt.Errorf("❌ Failed to list cvs: %v", err)
	}

	jobIdSet := make(map[int64]float32)
	if len(cvs) != 0 {
		// Get all cv ids
		cvIds := make([]int64, 0, len(cvs))

		for _, cv := range cvs {
			cvIds = append(cvIds, cv.ID)
			break // Only consider the first CV for now
		}
		fmt.Println("CV IDS: ", cvIds)

		// Get all cv-job pairs for the given cv ids
		cvJobPairs, err := s.queries.GetJobMatchesByCvIds(ctx, cvIds)

		if err != nil {
			return nil, fmt.Errorf("❌ Failed to get cv-job pairs: %v", err)
		}

		for _, pair := range cvJobPairs {
			//fmt.Println("Pair: ", pair.JobID, pair.Percentage)
			flt, err := pair.Percentage.Float64Value()
			if err != nil {

				fmt.Printf("❌ Failed to convert percentage to float64: %v, JobID: %d", err, pair.JobID)
				continue
			}
			jobIdSet[pair.JobID] = float32(flt.Float64)
		}
	}

	fmt.Println("PAAAAAIRRR", jobIdSet[3754]);
	var jobList = make([]models.Job, 0, len(jobs))

	for _, job := range jobs {
		j := models.Job{
			ID:              job.ID,
			SourceID:        job.SourceID.String,
			Source:          job.Source,
			Title:           job.Title.String,
			Company:         job.Company.String,
			Logo:            job.Logo.String,
			Location:        job.Location.String,
			Url:             job.Url.String,
			Tags:            job.Tags,
			Description:     job.Description.String,
			PublishAt:       job.PublishAt.Time,
			CreatedAt:       job.CreatedAt.Time,
			MatchPercentage: jobIdSet[int64(job.ID)],
		}

		jobList = append(jobList, j)
	}

	// Sort by MatchScore DESC, then newest PublishAt
	sort.SliceStable(jobList, func(i, j int) bool {
		if jobList[i].MatchPercentage == jobList[j].MatchPercentage {
			return jobList[i].PublishAt.After(jobList[j].PublishAt)
		}
		return jobList[i].MatchPercentage > jobList[j].MatchPercentage
	})

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

func (s *DBJobService) GetJobById(ctx context.Context, id int32) (*models.Job, error) {
	job, err := s.queries.GetJobById(ctx, id)

	if err != nil {
		return nil, err
	}

	resultJob := models.Job{
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
	}

	return &resultJob, nil
}
