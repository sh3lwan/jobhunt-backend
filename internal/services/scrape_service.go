package services

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/sh3lwan/jobhunter/internal/mq"
)

// ScrapeService dispatches scraping requests to jobscrapper's Kafka topic.
// The worker on the other side upserts the task record, so requests enqueued
// here show up in job_fetch_tasks like any other.
type ScrapeService struct {
	producer *mq.Producer
}

func NewScrapeService(producer *mq.Producer) *ScrapeService {
	return &ScrapeService{producer: producer}
}

// Dispatch enqueues one scraping request and returns its task id.
func (s *ScrapeService) Dispatch(platform string, skills []string, location string) (string, error) {
	if s.producer == nil {
		return "", fmt.Errorf("scrape dispatch is not configured")
	}

	taskID := uuid.New().String()

	request := map[string]any{
		"taskId":   taskID,
		"platform": platform,
		"skills":   skills,
	}

	if location != "" {
		request["location"] = location
	}

	payload, err := json.Marshal(request)

	if err != nil {
		return "", fmt.Errorf("failed to marshal scrape request: %w", err)
	}

	if err := s.producer.Send([]byte(taskID), payload); err != nil {
		return "", fmt.Errorf("failed to enqueue scrape request: %w", err)
	}

	return taskID, nil
}
