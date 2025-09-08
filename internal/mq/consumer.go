package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

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

		fmt.Printf("Received message with key: %d\n", key)

		var body models.EmbeddingResponse

		fmt.Printf("Message value: %s\n", string(msg.Value))
		err = json.Unmarshal(msg.Value, &body)

		if err != nil {
			fmt.Printf("Error unmarshalling message: %s\n", err)
			c.handleDLQ(ctx, msg, fmt.Errorf("Error decoding message: %s", err))
			//c.Reader.CommitMessages(ctx, msg)
		}

		// Process with bounded retries
		const maxAttempts = 5
		var lastErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
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

	switch body.Type {
	case "job_embedding":
		if err := qtx.InsertJobEmbedding(
			ctx,
			repository.InsertJobEmbeddingParams{
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
				CanonicalTextEmbeddings:        pgvector.NewVector(body.CanonicalTextEmbeddings),
				SkillsTextEmbeddings:           pgvector.NewVector(body.SkillsTextEmbeddings),
				ResponsibilitiesTextEmbeddings: pgvector.NewVector(body.ResponsibilitiesTextEmbeddings),
			},
		); err != nil {
			return fmt.Errorf("Error inserting job embedding into database: %s\n", err)
		}

		fmt.Printf("Inserted job embedding for job ID %d\n", key)

	default:
		// remove embeddings from json before storing
		if err := qtx.UpdateCVStructuredJSON(
			ctx,
			repository.UpdateCVStructuredJSONParams{
				ID:             key,
				StructuredJson: rawValue,
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
					CanonicalTextEmbeddings:        pgvector.NewVector(body.CanonicalTextEmbeddings),
					SkillsTextEmbeddings:           pgvector.NewVector(body.SkillsTextEmbeddings),
					ResponsibilitiesTextEmbeddings: pgvector.NewVector(body.ResponsibilitiesTextEmbeddings),
				},
			); err != nil {
				return fmt.Errorf("Error inserting CV embedding into database: %s\n", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("Error committing transaction: %s\n", err)
		return fmt.Errorf("Error committing transaction: %s\n", err)
	}

	log.Printf("Successfully processed message with key %d", key)

	return nil
}
