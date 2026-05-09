package services

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/sh3lwan/jobhunter/internal/mq"
)

type EmbeddingService struct {
	producer *mq.Producer
}

func NewEmbeddingService(producer *mq.Producer) *EmbeddingService {
	return &EmbeddingService{
		producer: producer,
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
