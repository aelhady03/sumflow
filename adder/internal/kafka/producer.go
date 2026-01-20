package kafka

import (
	"context"
	"log"
	"time"

	"github.com/aelhady03/sumflow/adder/internal/outbox"
	"github.com/aelhady03/sumflow/pkg/telemetry"
	kafka "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("kafka-producer")

// kafkaHeaderCarrier implements propagation.TextMapCarrier for Kafka headers
type kafkaHeaderCarrier []kafka.Header

func (c *kafkaHeaderCarrier) Get(key string) string {
	for _, h := range *c {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c *kafkaHeaderCarrier) Set(key, value string) {
	*c = append(*c, kafka.Header{Key: key, Value: []byte(value)})
}

func (c *kafkaHeaderCarrier) Keys() []string {
	keys := make([]string, len(*c))
	for i, h := range *c {
		keys[i] = h.Key
	}
	return keys
}

type KafkaProducer struct {
	writer *kafka.Writer
	topic  string
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	return &KafkaProducer{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
		topic: topic,
	}
}

// PublishEvent publishes an outbox event to Kafka with tracing and metrics
func (p *KafkaProducer) PublishEvent(ctx context.Context, event *outbox.Event) error {
	// Start span
	ctx, span := tracer.Start(ctx, "kafka.produce",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", p.topic),
			attribute.String("messaging.message_id", event.ID.String()),
		),
	)
	defer span.End()

	// Set published_at timestamp
	now := time.Now().UTC()
	event.PublishedAt = &now

	// Serialize event
	data, err := event.ToJSON()
	if err != nil {
		telemetry.KafkaMessagesProduced.WithLabelValues(p.topic, "error").Inc()
		span.RecordError(err)
		return err
	}

	// Inject trace context into headers
	var headers kafkaHeaderCarrier
	otel.GetTextMapPropagator().Inject(ctx, &headers)

	// Publish message
	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(event.AggregateID),
		Value:   data,
		Headers: headers,
	})

	if err != nil {
		telemetry.KafkaMessagesProduced.WithLabelValues(p.topic, "error").Inc()
		span.RecordError(err)
		log.Printf("kafka publish error: %v", err)
		return err
	}

	telemetry.KafkaMessagesProduced.WithLabelValues(p.topic, "success").Inc()
	return nil
}

func (p *KafkaProducer) Close() error {
	if p.writer != nil {
		return p.writer.Close()
	}
	return nil
}
