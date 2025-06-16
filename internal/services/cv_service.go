package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sh3lwan/jobhunter/internal/models"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

type CVService struct {
	repo     *repository.Queries
	producer *mq.Producer
}

func NewCVService(repo *repository.Queries, producer *mq.Producer) *CVService {
	return &CVService{
		repo:     repo,
		producer: producer,
	}
}

func (s *CVService) HandleCVUpload(ctx context.Context, filename string, data []byte) (int64, error) {
	fileID := uuid.New()
	ext := filepath.Ext(filename)
	tmpPath := filepath.Join(os.TempDir(), fileID.String()+ext)

	err := os.WriteFile(tmpPath, data, 0644)
	if err != nil {
		return 0, err
	}

	analysis, err := s.repo.CreateCVAnalysis(ctx, repository.CreateCVAnalysisParams{
		FileName:     fileID.String() + ext,
		OriginalName: filename,
		ParsedText:   pgtype.Text{Valid: false},
		Status:       "uploaded",
	})

	if err != nil {
		fmt.Printf("Error creating CV analysis: %v\n", err)
		return 0, err
	}

	parser := Parser{FilePath: tmpPath}

	cvData, err := parser.ExtractCV()
	if err != nil {
		fmt.Printf("Error parsing CV: %v\n", err)
		return 0, err
	}
	cvData.ID = analysis.ID

	textResult, err := json.Marshal(cvData)
	if err != nil {
		fmt.Printf("Error marshalling CV data: %v\n", err)
		return 0, err
	}
	err = s.repo.UpdateCVStatus(ctx, repository.UpdateCVStatusParams{
		ID:         analysis.ID,
		ParsedText: pgtype.Text{String: string(textResult), Valid: true},
		Status:     "parsed",
	})

	if err != nil {
		fmt.Printf("Error updating CV status: %v\n", err)
		return 0, err
	}

	err = s.producer.Send(cvData)
	if err != nil {
		return 0, err
	}

	return analysis.ID, nil
}

func (s *CVService) ListCVs(ctx context.Context, statuses []string) ([]repository.CvAnalysis, error) {
	if (statuses == nil) || (len(statuses) == 0) {
		statuses = []string{"uploaded", "parsed", "analyzed", "error"}
	}

	return s.repo.GetAllCVAnalysis(
		ctx, repository.GetAllCVAnalysisParams{
			Limit:   10,
			Offset:  0,
			Column3: statuses,
		})
}

func (s *CVService) Analyze(cv repository.CvAnalysis) error {
	s.producer.Send(&models.CVData{
		ID:      cv.ID,
		RawText: cv.ParsedText.String,
	})

	return nil
}
