package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/segmentio/kafka-go"

	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

type Consumer struct {
	Reader  *kafka.Reader
	Pool    *pgxpool.Pool
	Queries *repository.Queries

	// ScrapeProducer publishes scraping requests (jobscrapper's
	// job-scraping-requests topic) when a CV finishes analysis, so job
	// crawling follows the skills found in the CV. Optional — nil disables.
	ScrapeProducer  *Producer
	ScrapePlatforms []string
}

func NewConsumer(
	q *repository.Queries,
	p *pgxpool.Pool,
	topic,
	broker string,
) *Consumer {
	return &Consumer{
		Queries: q,
		Pool:    p,
		Reader: kafka.NewReader(kafka.ReaderConfig{
			Topic:   topic,
			GroupID: "cv-result-group",
			Brokers: []string{broker},
		}),
	}
}

func (c *Consumer) Consume() error {
	defer c.Reader.Close()

	ctx := context.Background()

	for {
		msg, err := c.Reader.ReadMessage(context.Background())

		if err != nil {
			fmt.Printf("Error reading message: %s\n", err)

			continue
		}

		keyStr := string(msg.Key)

		key, err := strconv.ParseInt(keyStr, 10, 64)

		if err != nil {
			fmt.Printf("Error converting key to int: %s\n", err)
			continue
		}

		var body models.EmbeddingResponse
		err = json.Unmarshal(msg.Value, &body)

		if err != nil {
			fmt.Printf("Error unmarshalling message: %s\n", err)
			c.handleDLQ(ctx, msg, fmt.Errorf("Error decoding message: %s", err))
			c.Reader.CommitMessages(ctx, msg)
		}

		// Process with bounded retries
		const maxAttempts = 5
		var lastErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			log.Printf("Processing message with key %d, data: %+v (attempt %d/%d)\n", key, body, attempt, maxAttempts)
			if err := c.processMessage(ctx, key, body, msg.Value); err != nil {
				lastErr = err
				fmt.Printf("Error processing message (attempt %d/%d): %s\n", attempt, maxAttempts, err)
				// exponential-ish backoff: 1s, 2s, 4s, 8s, 16s
				time.Sleep(time.Second << (attempt - 1))
				continue
			}
			lastErr = nil
			break
		}

		if lastErr != nil {
			// Couldn’t process → send to DLQ (include error in headers)
			if err := c.handleDLQ(ctx, msg, lastErr); err != nil {
				// If DLQ also fails, DO NOT COMMIT → message will be retried after restart
				fmt.Printf("dlq failed; not committing offset: %v\n", err)
				continue
			}
		}

		// Mark message as handled (success or DLQ’d)
		if err := c.Reader.CommitMessages(ctx, msg); err != nil {
			fmt.Printf("commit failed (will be retried): %v\n", err)
			// Not committing means it can be redelivered; make your handler idempotent
		}

	}
}

func (c *Consumer) handleDLQ(ctx context.Context, msg kafka.Message, err error) error {
	fmt.Printf("Handling DLQ for message with key %s: %s\n", string(msg.Key), err)

	return nil
}

func (c *Consumer) processMessage(ctx context.Context, key int64, body models.EmbeddingResponse, rawValue []byte) error {
	tx, err := c.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted, // sane default; tweak if needed
		AccessMode: pgx.ReadWrite,
	})

	if err != nil {
		return fmt.Errorf("Error starting transaction: %s\n", err)
	}

	defer func() { _ = tx.Rollback(ctx) }() // safe if already committed

	qtx := c.Queries.WithTx(tx)

	var analyzedCV map[string]any

	switch body.Type {
	case models.JobEmbeddingType:
		log.Printf("Processing Job embedding for Job ID %d\n", key)
		params := repository.InsertJobEmbeddingParams{
			JobID: key,
			CanonicalText: pgtype.Text{
				String: body.CanonicalText,
				Valid:  true,
			},
			SkillsText: pgtype.Text{
				String: body.SkillsText,
				Valid:  true,
			},
			ResponsibilitiesText: pgtype.Text{
				String: body.ResponsibilitiesText,
				Valid:  true,
			},
			CanonicalTextEmbeddings:        createOrEmptyVector(body.CanonicalTextEmbeddings),
			SkillsTextEmbeddings:           createOrEmptyVector(body.SkillsTextEmbeddings),
			ResponsibilitiesTextEmbeddings: createOrEmptyVector(body.ResponsibilitiesTextEmbeddings),
		}

		if err := qtx.InsertJobEmbedding(
			ctx,
			params,
		); err != nil {
			return fmt.Errorf("Error inserting job embedding into database: %s\n", err)
		}

		fmt.Printf("Inserted job embedding for job ID %d\n", key)

	default:
		log.Printf("Processing CV embedding for CV ID %d\n", key)

		structured, cvFields := stripEmbeddingFields(rawValue)

		if err := qtx.UpdateCVStructuredJSON(
			ctx,
			repository.UpdateCVStructuredJSONParams{
				ID:             key,
				StructuredJson: structured,
			}); err != nil {
			return fmt.Errorf("Error updating CV structured JSON: %s\n", err)
		}

		// if has canonical text embeddings, insert/update embeddings
		if body.CanonicalTextEmbeddings != nil {
			if err := qtx.InsertCVEmbedding(
				ctx,
				repository.InsertCVEmbeddingParams{
					CvID: key,
					CanonicalText: pgtype.Text{
						String: body.CanonicalText,
						Valid:  true,
					},
					SkillsText: pgtype.Text{
						String: body.SkillsText,
						Valid:  true,
					},
					ResponsibilitiesText: pgtype.Text{
						String: body.ResponsibilitiesText,
						Valid:  true,
					},
					CanonicalTextEmbeddings:        createOrEmptyVector(body.CanonicalTextEmbeddings),
					SkillsTextEmbeddings:           createOrEmptyVector(body.SkillsTextEmbeddings),
					ResponsibilitiesTextEmbeddings: createOrEmptyVector(body.ResponsibilitiesTextEmbeddings),
				},
			); err != nil {
				return fmt.Errorf("Error inserting CV embedding into database: %s\n", err)
			}
		}

		// The CV is fully analyzed — crawl jobs matching its profile once the
		// transaction commits.
		analyzedCV = cvFields
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %s\n", err)
		return fmt.Errorf("Error committing transaction: %s\n", err)
	}

	if analyzedCV != nil {
		c.dispatchScrapeForCV(key, analyzedCV)
	}

	// Materialize match scores for any new cv/job embedding pairs. Idempotent:
	// only missing pairs are inserted, so running after every message is safe.
	if count, err := c.Queries.InsertAllMissingCvJobPairs(ctx); err != nil {
		log.Printf("Error computing cv-job matches: %s\n", err)
	} else if count > 0 {
		log.Printf("Computed %d new cv-job match pairs", count)
	}

	log.Printf("Successfully processed message with key %d", key)

	return nil
}

func createOrEmptyVector(vector []float32) pgvector.Vector {
	if len(vector) == 0 || vector == nil {
		zeros := make([]float32, 768)
		return pgvector.NewVector(zeros)
	}
	return pgvector.NewVector(vector)
}

// stripEmbeddingFields removes the bulky embedding vectors from the analysis
// result before it is stored as the CV's structured_json, and returns the
// decoded fields for downstream use. Falls back to the raw payload if it
// cannot be decoded.
func stripEmbeddingFields(rawValue []byte) ([]byte, map[string]any) {
	var fields map[string]any

	if err := json.Unmarshal(rawValue, &fields); err != nil {
		return rawValue, nil
	}

	delete(fields, "canonical_text_embeddings")
	delete(fields, "skills_text_embeddings")
	delete(fields, "responsibilities_text_embeddings")

	cleaned, err := json.Marshal(fields)
	if err != nil {
		return rawValue, fields
	}

	return cleaned, fields
}

// dispatchScrapeForCV enqueues scraping requests based on what the CV
// contains: search keywords chosen by the parser when available, otherwise
// the top technologies/skills.
func (c *Consumer) dispatchScrapeForCV(cvID int64, cvFields map[string]any) {
	if c.ScrapeProducer == nil || len(c.ScrapePlatforms) == 0 || cvFields == nil {
		return
	}

	keywords := extractSearchKeywords(cvFields)

	if len(keywords) == 0 {
		log.Printf("CV %d analyzed but no search keywords found — skipping crawl dispatch", cvID)
		return
	}

	location, _ := cvFields["location_preference"].(string)

	// Company-size preference for this CV's owner narrows the crawl to the
	// sizes the user selected. Best-effort — an error just means "all sizes".
	sizeBands, err := c.Queries.GetPreferredSizesByCvId(context.Background(), cvID)
	if err != nil {
		log.Printf("CV %d: could not load size preference (%v) — crawling all sizes", cvID, err)
		sizeBands = nil
	}

	for _, platform := range c.ScrapePlatforms {
		request := map[string]any{
			"taskId":   uuid.New().String(),
			"platform": platform,
			"skills":   keywords,
		}

		if location != "" {
			request["location"] = location
		}

		if len(sizeBands) > 0 {
			request["sizeBands"] = sizeBands
		}

		payload, err := json.Marshal(request)
		if err != nil {
			log.Printf("Error marshalling scrape request for CV %d: %s", cvID, err)
			continue
		}

		if err := c.ScrapeProducer.Send([]byte(request["taskId"].(string)), payload); err != nil {
			log.Printf("Error dispatching %s scrape for CV %d: %s", platform, cvID, err)
			continue
		}

		log.Printf("Dispatched %s scrape for CV %d with keywords %v", platform, cvID, keywords)
	}
}

// extractSearchKeywords picks the terms to crawl with, preferring the
// parser-provided search_keywords, then technologies, then skills.
func extractSearchKeywords(fields map[string]any) []string {
	const maxKeywords = 6

	for _, source := range []string{"search_keywords", "technologies", "skills"} {
		raw, ok := fields[source].([]any)
		if !ok {
			continue
		}

		var keywords []string
		for _, item := range raw {
			s, ok := item.(string)
			if !ok || strings.TrimSpace(s) == "" {
				continue
			}
			keywords = append(keywords, strings.TrimSpace(s))
			if len(keywords) == maxKeywords {
				break
			}
		}

		if len(keywords) > 0 {
			return keywords
		}
	}

	return nil
}
