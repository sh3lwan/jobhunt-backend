package models

import "time"

const (
	JobEmbeddingType = "job_analysis"
)

// MatchedJob is a job together with its match score breakdown against a CV.
type MatchedJob struct {
	Job
	CanonicalPct        *float64 `json:"canonical_pct,omitempty"`        // Overall profile similarity component
	SkillsPct           *float64 `json:"skills_pct,omitempty"`           // Skills similarity component
	ResponsibilitiesPct *float64 `json:"responsibilities_pct,omitempty"` // Responsibilities similarity component
	DomainMultiplier    *float64 `json:"domain_multiplier,omitempty"`    // 1.0 same domain, 0.4 cross-domain penalty
	RerankScore         *float64 `json:"rerank_score,omitempty"`         // LLM recruiter-rubric score (authoritative when present)
	RerankDetails       any      `json:"rerank_details,omitempty"`       // {explanation, strengths, gaps, seniority_fit}
	Embedded            bool     `json:"embedded"`                       // Whether the job has been parsed + embedded yet

	// Deep evaluation (career-ops A-G via jobbridge), when one exists for this CV+job.
	EvalScore         *float64 `json:"eval_score,omitempty"`           // 1.0-5.0 global score
	EvalDecision      string   `json:"eval_decision,omitempty"`        // Apply | Consider | Research first | Skip
	EvalStatus        string   `json:"eval_status,omitempty"`          // requested | running | done | failed
	EvalHardStops     any      `json:"eval_hard_stops,omitempty"`      // blocking gaps from the Machine Summary
	EvalTailoredCvURL string   `json:"eval_tailored_cv,omitempty"`     // non-empty when a tailored CV PDF exists
	GeoRestriction    string   `json:"geo_restriction,omitempty"`      // scrape-time detected location restriction label
	GeoSponsorship    *bool    `json:"geo_sponsorship,omitempty"`      // true offered / false refused / absent unknown
}

// Job represents a job posting in the system.
type Job struct {
	ID              int32     `json:"id"`               // Unique identifier for the Job
	SourceID        string    `json:"source_id"`        // Identifier from the source API
	Source          string    `json:"source"`           // Source of the job posting (e.g., LinkedIn, Indeed)
	Title           string    `json:"title"`            // Job Title
	Company         string    `json:"company"`          // Company offering the Job
	Logo            string    `json:"logo"`             // URL to the company Logo
	Location        string    `json:"location"`         // Job Location
	Url             string    `json:"url"`              // URL to the job posting
	Description     string    `json:"description"`      // Job Description
	Tags            []string  `json:"tags"`             // Tags associated with the job (e.g., skills, job type)
	MatchPercentage float32   `json:"match_percentage"` // Match score for the job (if applicable)
	PublishAt       time.Time `json:"publish_at"`       // Date and time when the job was published
	CreatedAt       time.Time `json:"created_at"`       // Date and time when the job was created in the system
}
