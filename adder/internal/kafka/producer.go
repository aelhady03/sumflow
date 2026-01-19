package kafka

import (
	"context"
	"log"

	"github.com/aelhady03/sumflow/adder/internal/outbox"
	kafka "github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

// PublishEvent publishes an outbox event to Kafka
func (p *KafkaProducer) PublishEvent(ctx context.Context, event *outbox.Event) error {
	data, err := event.ToJSON()
	if err != nil {
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.AggregateID),
		Value: data,
	})

	if err != nil {
		log.Printf("kafka publish error: %v", err)
		return err
	}

	return nil
}

func (p *KafkaProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
