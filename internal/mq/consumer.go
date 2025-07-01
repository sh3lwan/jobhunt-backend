package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pgvector/pgvector-go"
	"github.com/segmentio/kafka-go"
	"github.com/sh3lwan/jobhunter/internal/models"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"strconv"
)

type Consumer struct {
	Reader  *kafka.Reader
	Queries *repository.Queries
}

func NewConsumer(q *repository.Queries, topic, broker string) *Consumer {
	return &Consumer{
		Queries: q,
		Reader: kafka.NewReader(kafka.ReaderConfig{
			Topic:   topic,
			GroupID: "cv-result-group",
			Brokers: []string{broker},
		}),
	}
}

func (c *Consumer) Consume() {
	defer c.Reader.Close()
	for {

		msg, err := c.Reader.ReadMessage(context.Background())
		if err != nil {
			fmt.Printf("Error reading message: %s\n", err)
		}

		keyStr := string(msg.Key)
		key, err := strconv.ParseInt(keyStr, 10, 64)

		if err != nil {
			fmt.Printf("Error converting key to int: %s\n", err)
		}

		fmt.Printf("Received message: %s\n", string(msg.Value))

		var body models.ResponseCVData
		err = json.Unmarshal(msg.Value, &body)
		fmt.Printf("Received body: %+v\n", body)
		if err != nil {
			fmt.Printf("Error unmarshalling message: %s\n", err)
		}

		err = c.Queries.UpdateCVStructuredJSON(
			context.Background(),
			repository.UpdateCVStructuredJSONParams{
				ID:             key,
				StructuredJson: msg.Value,
			})

		err = c.Queries.InsertCVEmbedding(
			context.Background(),
			repository.InsertCVEmbeddingParams{
				CvID:      key,
				Embedding: pgvector.NewVector(body.Embeddings),
			},
		)

		if err != nil {
			fmt.Printf("Error updating cv: %s\n", err)
		}
	}
}
