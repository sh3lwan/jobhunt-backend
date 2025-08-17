package mq

import (
	"context"
	"github.com/segmentio/kafka-go"
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

func (p *Producer) Send(key, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()
	
	msg := kafka.Message{Key: key, Value: body}

	return p.Writer.WriteMessages(ctx, msg)
}

func (p *Producer) Close() error {
	return p.Writer.Close()
}
