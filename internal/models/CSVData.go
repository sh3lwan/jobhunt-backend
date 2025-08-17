package models

import "time"

type CVData struct {
	ID             int64    `json:"id"` // Unique identifier for tracking
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Phone          string   `json:"phone"`
	Summary        string   `json:"summary"`
	Links          []string `json:"links"`
	Skills         []string `json:"skills"`
	Experience     []string `json:"experience"` // Could be structured later
	Education      []string `json:"education"`
	Certifications []string `json:"certifications"`
	Languages      []string `json:"languages"`
	RawText        string   `json:"raw_text"` // full CV text dump
}
type ExperienceItem struct {
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Location    string   `json:"location"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	Tasks       []string `json:"tasks"`
	Description string   `json:"description"`
	TechStack   []string `json:"tech_stack"`
}

type ResponseCVData struct {
	Name         string   `json:"name"`
	Portfolio    string   `json:"portfolio"`
	Linkedin     string   `json:"linkedin"`
	Github       string   `json:"github"`
	Skills       []string `json:"skills"`
	Technologies []string `json:"technologies"`
	Projects     []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"projects"`
	Education []struct {
		University string `json:"university"`
		Degree     string `json:"degree"`
		StartDate  string `json:"start_date"`
		EndDate    string `json:"end_date"`
	} `json:"education"`
	Experience []ExperienceItem `json:"experience"`
	Embeddings []float32        `json:"embeddings"`
}

type Error struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
	Time   time.Time `json:"time"` // ISO 8601 format
}
