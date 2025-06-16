package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sh3lwan/jobhunter/internal/models"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
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
}

func (p *Parser) ExtractCV() (*models.CVData, error) {
	resp, err := p.Parse()

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

	fmt.Println("Sending to parser ...")
	req, err := http.NewRequest("POST", parseURL+"parse-pdf", body)
	if err != nil {
		fmt.Printf("Error creating parser request: %v\n", err)
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error receiving resposne from client: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	fmt.Printf("Received response from parser %s %s\n ", resp.Status, resp.Body)

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("parser failed with status code %d", resp.StatusCode)
		return nil, err
	}

	var result ParseResponse
	json.NewDecoder(resp.Body).Decode(&result)

	return &result, nil
}
