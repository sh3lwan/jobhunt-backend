package models

import "github.com/google/uuid"

const (
	cvAnalysisJobType = "cv_analysis"
)

type BaseJob struct {
	JobID   string `json:"job_id"`
	JobType string `json:"job_type"`
}

type JobEnvelope[T any] struct {
	BaseJob
	Data T `json:"data"`
}

type AnalysisCVData struct {
	Links []string `json:"links"`
	Text  string   `json:"text"`
}

type CVAnalysisJob = JobEnvelope[AnalysisCVData]

func NewCVAnalysisJob(data AnalysisCVData) *CVAnalysisJob {
	return &CVAnalysisJob{
		BaseJob: BaseJob{
			JobID:   uuid.NewString(),
			JobType: cvAnalysisJobType,
		},
		Data: data,
	}
}
