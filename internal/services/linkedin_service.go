package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"golang.org/x/net/html"
)

type LinkedInService struct {
	JobService
}

func NewLinkedInService() *LinkedInService {
	return &LinkedInService{
		JobService{
			BaseURL: "https://linkedin.com/in/search/?keywords=",
		},
	}
}

func (s *LinkedInService) CollectJobs(q *repository.Queries, ctx context.Context, skills []string) ([]repository.Job, error) {
	query := strings.Join(skills, "%20")

	url := s.BaseURL + query

	response, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := response.Header.Get("Retry-After")
		fmt.Printf("HTTP 429: Retry after %s seconds\n", retryAfter)
		return nil, fmt.Errorf("rate limited: retry after %s seconds", retryAfter)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 HTTP response: %d", response.StatusCode)
	}

	tokenizer := html.NewTokenizer(response.Body)
	var jobs []repository.Job
	var jobLinks []string

	for {
		tokenType := tokenizer.Next()

		if tokenType == html.ErrorToken {
			fmt.Println("Token Type Error")
			break
		}

		token := tokenizer.Token()

		if tokenType == html.StartTagToken && token.Data == "a" {
			for _, attr := range token.Attr {
				if attr.Key == "href" && strings.Contains(attr.Val, "/jobs/view/") {
					jobLinks = append(jobLinks, attr.Val)
				}
			}
		}
	}

	if len(jobLinks) > 0 {
		for _, jobUrl := range jobLinks {
			fmt.Println("Job URL: " + jobUrl)

			jobPage := fetchPage(jobUrl)

			job, err := extractJob(jobPage, jobUrl)

			if err != nil {
				return nil, err
			}

			jobs = append(jobs, job)
		}
	}

	//dbJobService := services.NewDBJobService(q)

	//err = dbJobService.SaveJobs(q, ctx, jobs)
//	if err != nil {
//		return nil, fmt.Errorf("error saving jobs: %w", err)
//	}

	return jobs, nil
}

func extractJob(body, url string) (repository.Job, error) {

	// Fallback if label matching fails
	findBetween := func(body, startTag, endTag string) string {
		start := strings.Index(body, startTag)
		if start == -1 {
			return ""
		}
		start += len(startTag)
		end := strings.Index(body[start:], endTag)
		if end == -1 {
			return ""
		}
		return strings.TrimSpace(stripHTML(body[start : start+end]))
	}

	title := findBetween(body, `<h1 class="top-card-layout__title`, "</h1>")
	company := findBetween(body, `<a class="topcard__org-name-link`, "</a>")
	if company == "" {
		company = findBetween(body, `<span class="topcard__flavor`, "</span>")
	}
	logo := extractImageURL(body)
	location := findBetween(body, `<span class="topcard__flavor--bullet`, "</span>")
	description := findBetween(body, `<div class="description__text`, "</div>")

	sourceID := extractLinkedInJobID(url)

	now := time.Now()
	var publishAt, createdAt pgtype.Timestamptz
	_ = publishAt.Scan(now)
	_ = createdAt.Scan(now)

	return repository.Job{
		SourceID:    pgtype.Text{String: sourceID, Valid: true},
		Source:      "linkedin",
		Title:       pgtype.Text{String: title, Valid: title != ""},
		Company:     pgtype.Text{String: company, Valid: company != ""},
		Logo:        pgtype.Text{String: logo, Valid: logo != ""},
		Location:    pgtype.Text{String: location, Valid: location != ""},
		Url:         pgtype.Text{String: url, Valid: true},
		Tags:        []string{},
		Description: pgtype.Text{String: description, Valid: description != ""},
		PublishAt:   publishAt,
		CreatedAt:   createdAt,
	}, nil
}

func fetchPage(url string) string {
	response, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer response.Body.Close()

	buf := new(strings.Builder)
	_, _ = io.Copy(buf, response.Body)

	return buf.String()
}

func extractLinkedInJobID(url string) string {
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractImageURL(body string) string {
	r := regexp.MustCompile(`<img[^>]+class="topcard__flavor--logo"[^>]+src="([^"]+)"`)
	match := r.FindStringSubmatch(body)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func stripHTML(input string) string {
	re := regexp.MustCompile(`<.*?>`)
	return re.ReplaceAllString(input, "")
}
