package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sh3lwan/jobhunter/internal/models"
)

const (
	parseURL = "http://localhost:5001/api/v1/"
)

type Parser struct {
	FilePath string
}
type ParseResponse struct {
	Text  string   `json:"text"`
	Links []string `json:"links"`
	Error string   `json:"error,omitempty"`
}

func (p *Parser) ExtractCV() (*models.CVData, error) {
	resp, err := p.Parse()

	log.Println("Parser response:", resp)
	if err != nil {
		return nil, err
	}

	return &models.CVData{
		RawText: resp.Text,
		Links:   resp.Links,
	}, nil
}

func (p *Parser) Parse() (*ParseResponse, error) {
	ext := filepath.Ext(p.FilePath)
	switch ext {
	case ".pdf":
		return extractPDF(p.FilePath)
	default:
		return nil, errors.New("invalid file extension")
	}
}

// new extractPDF → calls the python-parser microservice
func extractPDF(filePath string) (*ParseResponse, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return sendToParserService(fileBytes)
}

func sendToParserService(pdfFile []byte) (*ParseResponse, error) {
	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", "cv.pdf")
	part.Write(pdfFile)
	writer.Close()

	req, err := http.NewRequest("POST", parseURL+"parse-pdf", body)
	if err != nil {
		fmt.Printf("Error creating parser request: %v\n", err)
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}

	resp, err := client.Do(req)

	log.Printf("Parser service response: %+v\n", resp)
	if err != nil {
		return nil, fmt.Errorf("Error receiving resposne from client: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("parser failed with status code %d", resp.StatusCode)
	}

	fmt.Printf("Received response from parser service %s", resp.StatusCode)
	var result ParseResponse

	err = json.NewDecoder(resp.Body).Decode(&result)

	log.Printf("Parser service response: %+v\n", resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error decoding parser response: %v\n", err)
	}

	return &result, nil
}
