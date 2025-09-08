package services

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
)

const (
	analyzeURL = "http://localhost:5001/api/v1/"
)

type AnalyzeService struct {
	URL string
}

func NewAnalyzeService() *AnalyzeService {
	return &AnalyzeService{
		URL: analyzeURL,
	}
}

func (s *AnalyzeService) SubmitCVAnalysisJob(rawText string, links []string) (string, error) {
	// Prepare the request body
	body := bytes.NewBuffer([]byte(`{
		"text": "` + rawText + `",
		"links": ` + fmt.Sprintf("%q", links) + `
		}`))

	resp, err := http.Post(s.URL+"jobs/cv", "application/json", body)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to submit CV analysis job, status code: %d", resp.StatusCode)
	}

	var result []byte
	// Read response body
	_, err = resp.Body.Read(result)

	if err != nil {
		return "", err
	}

	log.Printf("Analyze service response: %+v\n", string(result))

	return string(result), nil
}
