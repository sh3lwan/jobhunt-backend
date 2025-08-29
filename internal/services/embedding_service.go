package services

import (
	"encoding/json"

	"github.com/sh3lwan/jobhunter/internal/mq"
)

const (
	JobEmbeddingType = "job_embedding"
)

type EmbeddingService struct {
	producer *mq.Producer
}

func NewEmbeddingService(producer *mq.Producer) *EmbeddingService {
	return &EmbeddingService{
		producer: producer,
	}
}

func (s *EmbeddingService) SendEmbeddingRequest(key, data []byte) error {
	// Send the data to the embedding queue
	msg := map[string]string {
		"id":  string(key),
		"raw_text": string(data),
		"type": JobEmbeddingType,
	}
	// Convert the map to a byte slice (JSON or any other format)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.producer.Send(key, data)
}
