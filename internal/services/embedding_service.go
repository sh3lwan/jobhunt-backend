package services

import "github.com/sh3lwan/jobhunter/internal/mq"

type EmbeddingService struct {
	producer *mq.Producer
}

func NewEmbeddingService(producer *mq.Producer) *EmbeddingService {
	return &EmbeddingService{
		producer: producer,
	}
}

func (s *EmbeddingService) SendEmbeddingRequest(data string) error {

	// Convert the data to a byte slice
	dataBytes := []byte(data)
	// Send the data to the embedding queue
	return  s.producer.Send([]byte("embedding_request"), dataBytes)
}

