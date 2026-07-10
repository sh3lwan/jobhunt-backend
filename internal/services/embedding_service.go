package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/mq"
	"github.com/sh3lwan/jobhunter/internal/repository"
)

// resendCooldown guards against re-requesting an embedding for a job that is
// still being processed by the parser (LLM parsing takes on the order of a
// minute per job, while the reconciler may run more often than that).
const resendCooldown = 30 * time.Minute

type EmbeddingService struct {
	producer *mq.Producer
	queries  *repository.Queries

	mu     sync.Mutex
	sentAt map[int32]time.Time
}

func NewEmbeddingService(producer *mq.Producer, queries *repository.Queries) *EmbeddingService {
	return &EmbeddingService{
		producer: producer,
		queries:  queries,
		sentAt:   make(map[int32]time.Time),
	}
}

func (s *EmbeddingService) SendEmbeddingRequest(key, body, embeddType string) error {
	// Send the data to the embedding queue
	msg := map[string]string{
		"job_id":   uuid.New().String(),
		"job_type": embeddType,
		"key":      key,
		"data":     body,
	}

	// Convert the map to a byte slice (JSON or any other format)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.producer.Send([]byte(key), data)
}

// RequestMissingJobEmbeddings finds jobs that have no row in jobs_embeddings
// and sends an embedding request for each. It is idempotent and safe to call
// from a cron: jobs already requested within the last 30 minutes are skipped
// (tracked in the DB via embedding_requested_at, so it survives restarts; the
// in-memory cooldown remains as a second guard within a single process).
// Returns the number of requests sent.
func (s *EmbeddingService) RequestMissingJobEmbeddings(ctx context.Context, limit int32) (int, error) {
	jobs, err := s.queries.GetJobsMissingEmbeddings(ctx, limit)

	if err != nil {
		return 0, fmt.Errorf("failed to list jobs missing embeddings: %w", err)
	}

	sent := 0
	var sentIDs []int32

	defer func() {
		if len(sentIDs) > 0 {
			if err := s.queries.MarkJobsEmbeddingRequested(ctx, sentIDs); err != nil {
				log.Printf("failed to mark %d jobs as embedding-requested: %v", len(sentIDs), err)
			}
		}
	}()

	for _, job := range jobs {
		if !s.markSent(job.ID) {
			continue
		}

		payload := models.Job{
			ID:          job.ID,
			SourceID:    job.SourceID.String,
			Source:      job.Source,
			Title:       job.Title.String,
			Company:     job.Company.String,
			Location:    job.Location.String,
			Url:         job.Url.String,
			Tags:        job.Tags,
			Description: job.Description.String,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return sent, fmt.Errorf("failed to marshal job %d: %w", job.ID, err)
		}

		key := strconv.FormatInt(int64(job.ID), 10)

		if err := s.SendEmbeddingRequest(key, string(data), models.JobEmbeddingType); err != nil {
			return sent, fmt.Errorf("failed to send embedding request for job %d: %w", job.ID, err)
		}

		sent++
		sentIDs = append(sentIDs, job.ID)
	}

	return sent, nil
}

// markSent records that an embedding request is in flight for the job and
// reports whether the caller should proceed (false while inside the cooldown).
func (s *EmbeddingService) markSent(jobID int32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	if last, ok := s.sentAt[jobID]; ok && now.Sub(last) < resendCooldown {
		return false
	}

	// Opportunistically drop expired entries so the map doesn't grow forever.
	for id, t := range s.sentAt {
		if now.Sub(t) >= resendCooldown {
			delete(s.sentAt, id)
		}
	}

	s.sentAt[jobID] = now

	return true
}
