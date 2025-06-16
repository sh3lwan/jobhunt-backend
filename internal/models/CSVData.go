package models

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
