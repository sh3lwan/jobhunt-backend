package models

import "time"

// Job represents a job posting in the system.
type Job struct {
	ID          int32     `json:"id"`          // Unique identifier for the Job
	SourceID    string    `json:"source_id"`   // Identifier from the source API
	Source      string    `json:"source"`      // Source of the job posting (e.g., LinkedIn, Indeed)
	Title       string    `json:"title"`       // Job Title
	Company     string    `json:"company"`     // Company offering the Job
	Logo        string    `json:"logo"`        // URL to the company Logo
	Location    string    `json:"location"`    // Job Location
	Url         string    `json:"url"`         // URL to the job posting
	Description string    `json:"description"` // Job Description
	Tags        []string  `json:"tags"`        // Tags associated with the job (e.g., skills, job type)
	MatchPercentage  float32   `json:"match_percentage"` // Match score for the job (if applicable)
	PublishAt   time.Time `json:"publish_at"`  // Date and time when the job was published
	CreatedAt   time.Time `json:"created_at"`  // Date and time when the job was created in the system
}
