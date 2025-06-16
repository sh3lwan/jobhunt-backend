package mq

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"github.com/sh3lwan/jobhunter/internal/models"
	"log"
	"time"
	"fmt"
)

type Producer struct {
	Writer *kafka.Writer
}

func NewProducer(broker string, topic string) *Producer {
	return &Producer{
		Writer: &kafka.Writer{
			Addr:         kafka.TCP(broker),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireAll,
		},
	}
}

func (p *Producer) Send(cv *models.CVData) error {
	// marshal CVData into JSON
	value, err := json.Marshal(cv)
	if err != nil {
		return err
	}

	// create kafka message
	msg := kafka.Message{
		Key:   []byte(fmt.Sprint(cv.ID)), // optional: use ID as key
		Value: value,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// send message
	err = p.Writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("Failed to send message to kafka: %v\n", err)
		return err
	}

	log.Println("✅ Sent message to Kafka")
	return nil
}

func (p *Producer) Close() {
	p.Writer.Close()
}
