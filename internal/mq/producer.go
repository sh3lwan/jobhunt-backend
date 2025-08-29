package mq

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
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

func (p *Producer) Send(key, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()
	
	msg := kafka.Message{Key: key, Value: body}

	log.Printf("Sending message to Kafka topic %s with key %s, body %s", p.Writer.Topic, string(key), string(body))

	return p.Writer.WriteMessages(ctx, msg)
}

func (p *Producer) Close() error {
	return p.Writer.Close()
}
