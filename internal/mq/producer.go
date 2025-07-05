package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/segmentio/kafka-go"
	"github.com/sh3lwan/jobhunter/internal/models"
	"log"
	"time"
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
		Key:   fmt.Append(nil, cv.ID), // optional: use ID as key
		Value: value,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = kafka.Dial("tcp", "localhost:9092")
	if err != nil {
		fmt.Printf("Error connecting to localhost:9092: %v", err)
		return nil
	}

	// send message
	err = p.Writer.WriteMessages(ctx, msg)
	if err != nil {
		log.Printf("Failed to send message to kafka: %v\n", err)
		return err
	}

	log.Println("✅ Sent message to Kafka")
	return nil
}

func (p *Producer) Close() error {
	return p.Writer.Close()
}
