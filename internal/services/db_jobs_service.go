package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

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

// LatestAnalyzedCVId returns the user's most recently analyzed CV id.
func (s *DBJobService) LatestAnalyzedCVId(ctx context.Context, userID int64) (int64, error) {
	return s.queries.GetLatestAnalyzedCVId(ctx, pgtype.Int8{Int64: userID, Valid: true})
}

// MatchedJobsFilter narrows the matched-jobs listing.
type MatchedJobsFilter struct {
	CvID          int64
	Sources       []string
	MinPercentage *float64
	MaxAgeDays    *int32
	Search        string
	Limit         int32
	Offset        int32
}

// ListMatchedJobs returns jobs with their match breakdown against the given
// CV, ordered by match percentage (jobs not yet matched come last), plus the
// total count for pagination.
func (s *DBJobService) ListMatchedJobs(ctx context.Context, filter MatchedJobsFilter) ([]models.MatchedJob, int64, error) {
	var minPct pgtype.Numeric

	if filter.MinPercentage != nil {
		if err := minPct.Scan(strconv.FormatFloat(*filter.MinPercentage, 'f', 2, 64)); err != nil {
			return nil, 0, fmt.Errorf("invalid min percentage: %w", err)
		}
	}

	var search pgtype.Text
	if filter.Search != "" {
		search = pgtype.Text{String: filter.Search, Valid: true}
	}

	var maxAge pgtype.Int4
	if filter.MaxAgeDays != nil {
		maxAge = pgtype.Int4{Int32: *filter.MaxAgeDays, Valid: true}
	}

	rows, err := s.queries.GetMatchedJobs(ctx, repository.GetMatchedJobsParams{
		CvID:          filter.CvID,
		Sources:       filter.Sources,
		MinPercentage: minPct,
		MaxAgeDays:    maxAge,
		Search:        search,
		MaxResults:    filter.Limit,
		ResultOffset:  filter.Offset,
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list matched jobs: %w", err)
	}

	total, err := s.queries.CountMatchedJobs(ctx, repository.CountMatchedJobsParams{
		CvID:          filter.CvID,
		Sources:       filter.Sources,
		MinPercentage: minPct,
		MaxAgeDays:    maxAge,
		Search:        search,
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to count matched jobs: %w", err)
	}

	jobs := make([]models.MatchedJob, 0, len(rows))

	for _, row := range rows {
		job := models.MatchedJob{
			Job: models.Job{
				ID:              row.ID,
				SourceID:        row.SourceID.String,
				Source:          row.Source,
				Title:           row.Title.String,
				Company:         row.Company.String,
				Logo:            row.Logo.String,
				Location:        row.Location.String,
				Url:             row.Url.String,
				Tags:            row.Tags,
				Description:     row.Description.String,
				PublishAt:       row.PublishAt.Time,
				CreatedAt:       row.CreatedAt.Time,
				MatchPercentage: numericToFloat32(row.Percentage),
			},
			CanonicalPct:        numericToFloatPtr(row.CanonicalPct),
			SkillsPct:           numericToFloatPtr(row.SkillsPct),
			ResponsibilitiesPct: numericToFloatPtr(row.ResponsibilitiesPct),
			DomainMultiplier:    numericToFloatPtr(row.DomainMultiplier),
			RerankScore:         numericToFloatPtr(row.RerankScore),
			Embedded:            row.Embedded,
		}

		if len(row.RerankDetails) > 0 {
			var details map[string]any
			if err := json.Unmarshal(row.RerankDetails, &details); err == nil {
				job.RerankDetails = details
			}
		}

		jobs = append(jobs, job)
	}

	return jobs, total, nil
}

func numericToFloat32(n pgtype.Numeric) float32 {
	if !n.Valid {
		return 0
	}
	v, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return float32(v.Float64)
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil {
		return nil
	}
	return &v.Float64
}

func (s *DBJobService) SaveJobs(ctx context.Context, jobs []repository.Job) error {
	for _, job := range jobs {
		err := s.queries.CreateJob(ctx, repository.CreateJobParams{
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
