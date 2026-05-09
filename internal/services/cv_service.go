package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

const (
	baseDir = "storage/uploads" // lives inside your project
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

func (s *CVService) HandleCVUpload(ctx context.Context, originalName string, data []byte) (int64, error) {
	ext := filepath.Ext(originalName)
	fileName := uuid.New().String() + ext

	// store file in disk inside project directory
	uploadsvr := NewUploadFileService(baseDir)
	_, err := uploadsvr.SaveFile(fileName, data)

	if err != nil {
		return 0, fmt.Errorf("❌ Failed to save CV file: %v", err)
	}

	contextUser, err := utils.GetUserFromContext(ctx)

	if err != nil {
		return 0, fmt.Errorf("❌ Failed to get user from context: %v", err)
	}

	analysis, err := s.repo.CreateCVAnalysis(ctx, repository.CreateCVAnalysisParams{
		FileName:     fileName,
		OriginalName: originalName,
		ParsedText:   pgtype.Text{Valid: false},
		Status:       "uploaded",
		UserID:       pgtype.Int8{Int64: contextUser.ID, Valid: true},
	})

	if err != nil {
		return 0, fmt.Errorf("❌ Failed to create CV analysis record: %v", err)
	}

	cvData, err := s.Parse(ctx, &analysis)

	if err != nil || cvData == nil {
		return 0, err
	}

	analysis.ParsedText = pgtype.Text{String: cvData.RawText, Valid: false}

	err = s.Analyze(&analysis)

	if err != nil {
		return 0, err
	}

	return analysis.ID, nil
}

func (s *CVService) AnalyzeCV(ctx context.Context, id int64) error {
	cv, err := s.repo.GetCVAnalysis(ctx, id)

	if err != nil {
		return fmt.Errorf("❌ Failed to get CV analysis: %v", err)
	}

	if cv.ID == 0 {
		return fmt.Errorf("❌ CV analysis with ID %d not found", id)
	}

	if cv.Status != "parsed" {
		return fmt.Errorf("❌ CV analysis with ID %d is not in parsed state", id)
	}

	return s.Analyze(&cv)
}

func (s *CVService) ListCVs(ctx context.Context, statuses []string, userId int64) ([]repository.CvAnalysis, error) {
	if (statuses == nil) || (len(statuses) == 0) {
		statuses = []string{"uploaded", "parsed", "analyzed", "error"}
	}

	return s.repo.GetAllCVAnalysis(
		ctx, repository.GetAllCVAnalysisParams{
			UserID:  pgtype.Int8{Int64: userId, Valid: true},
			Limit:   10,
			Offset:  0,
			Column3: statuses,
		})
}

func (s *CVService) GetCV(ctx context.Context, id int64) (*repository.CvAnalysis, error) {
	cv, err := s.repo.GetCVAnalysis(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("❌ Failed to get CV analysis: %v", err)
	}

	if cv.ID == 0 {
		return nil, fmt.Errorf("❌ CV analysis with ID %d not found", id)
	}

	return &cv, nil
}

func (s *CVService) Analyze(cv *repository.CvAnalysis) error {
	key := fmt.Append(nil, cv.ID)

	var cvData models.CVData
	err := json.Unmarshal([]byte(cv.ParsedText.String), &cvData)

	if err != nil {
		return fmt.Errorf("❌ Failed to unmarshal CV parsed text: %v", err)
	}

	// Create CVAnalysis message
	value, err := json.Marshal(&models.CVAnalysisJob{
		BaseJob: models.BaseJob{
			JobID:   string(key),
			JobType: "cv_analysis",
		},
		Data: models.AnalysisCVData{
			Text:  cvData.RawText,
			Links: cvData.Links,
		},
	})

	return s.producer.Send(key, value)
}

func (s *CVService) FetchJobs(skills []string) {
	go func() {

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		defer cancel()

		rmtv := NewRemotiveService()

		dbjobService := NewDBJobService(s.repo)

		jobs, err := rmtv.CollectJobs(s.repo, ctx, skills)

		if err != nil {
			fmt.Printf("Something went wrong: %v\n", err.Error())
			return
		}

		fmt.Printf("Received %d jobs\n", len(jobs))

		err = dbjobService.SaveJobs(ctx, jobs)

		if err != nil {
			fmt.Printf("Something went wrong on saving: %v\n", err)
			return
		}

		fmt.Println("Saved jobs successfully")

	}()
}

func (s *CVService) GetSkills(ctx context.Context) ([]string, error) {

	skills, err := s.repo.GetDistinctSkills(ctx)

	if err != nil {
		return nil, err
	}

	return cleanSkills(skills), nil
}

func (s *CVService) GetSkillsForCV(ctx context.Context, id int64) ([]string, error) {
	skills, err := s.repo.GetDistinctSkillsForCV(ctx, id)

	if err != nil {
		return nil, err
	}

	return cleanSkills(skills), nil
}

func (s *CVService) Parse(ctx context.Context, cvAnalysis *repository.CvAnalysis) (*models.CVData, error) {
	uploadsvr := NewUploadFileService(baseDir)

	filePath := uploadsvr.GetFilePath(cvAnalysis.FileName)

	parser := Parser{FilePath: filePath}

	cvData, err := parser.ExtractCV()

	if err != nil {
		return nil, err
	}

	cvData.ID = cvAnalysis.ID

	textResult, err := json.MarshalIndent(cvData, "", "  ")

	if err != nil {
		return nil, err
	}

	err = s.repo.UpdateCVParsedText(ctx, repository.UpdateCVParsedTextParams{
		ID:         cvAnalysis.ID,
		ParsedText: pgtype.Text{String: string(textResult), Valid: true},
	})

	if err != nil {
		return nil, err
	}

	return cvData, nil
}

func (s *CVService) HandleCVError(ctx context.Context, cvID int64, err error) error {
	obj, err := json.Marshal(&models.Error{
		Time:    time.Now(),
		Message: err.Error(),
	})

	if err != nil {
		return err
	}

	return s.repo.UpdateCVErrors(ctx, repository.UpdateCVErrorsParams{
		ID:     cvID,
		Errors: obj,
	})
}

func cleanSkills(skills []string) []string {
	var cleaned []string
	for _, skill := range skills {
		s := strings.Trim(skill, `"' `)
		if s != "" {
			cleaned = append(cleaned, s)
		}
	}
	return cleaned
}
