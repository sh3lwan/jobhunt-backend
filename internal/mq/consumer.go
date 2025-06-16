package mq

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/sh3lwan/jobhunter/internal/repository"
	"strconv"
)

type Consumer struct {
	Reader  *kafka.Reader
	Queries *repository.Queries
}

func NewConsumer(q *repository.Queries) *Consumer {
	return &Consumer{
		Queries: q,
		Reader: kafka.NewReader(kafka.ReaderConfig{
			Topic:   "cv-result",
			GroupID: "cv-result-group",
			Brokers: []string{"localhost:9092"},
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

		//fmt.Printf("Received message: %s\n", string(msg.Value))
		err = c.Queries.UpdateCVStructuredJSON(
			context.Background(),
			repository.UpdateCVStructuredJSONParams{
				ID:             key,
				StructuredJson: msg.Value,
			})
		if err != nil {
			fmt.Printf("Error updating cv: %s\n", err)
		}
	}
}
