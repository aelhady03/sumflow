package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Latency buckets: 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s
var latencyBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// EventProcessingLatency measures full lifecycle latency from event creation to consumer processing.
// Uses the created_at timestamp from the event.
var EventProcessingLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "event_processing_latency_seconds",
		Help:    "Full lifecycle latency from event creation to consumer processing (seconds)",
		Buckets: latencyBuckets,
	},
	[]string{"topic", "event_type"},
)

// KafkaDeliveryLatency measures Kafka-only latency from publish to consumer processing.
// Uses the published_at timestamp from the event.
var KafkaDeliveryLatency = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "kafka_delivery_latency_seconds",
		Help:    "Kafka delivery latency from publish to consumer processing (seconds)",
		Buckets: latencyBuckets,
	},
	[]string{"topic", "event_type"},
)

// KafkaMessagesProduced counts messages sent to Kafka.
var KafkaMessagesProduced = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_produced_total",
		Help: "Total number of messages produced to Kafka",
	},
	[]string{"topic", "status"},
)

// KafkaMessagesConsumed counts messages consumed from Kafka.
var KafkaMessagesConsumed = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "kafka_messages_consumed_total",
		Help: "Total number of messages consumed from Kafka",
	},
	[]string{"topic", "event_type", "status"},
)
