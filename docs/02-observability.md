# Phase 2: OpenTelemetry Observability

## Overview

Add distributed tracing and metrics to both services using OpenTelemetry, with specific focus on Kafka message delivery latency.

## Architecture

```
┌──────────────────┐     ┌──────────────────┐
│  Adder Service   │     │ Totalizer Service│
│  ┌────────────┐  │     │  ┌────────────┐  │
│  │ OTel SDK   │  │     │  │ OTel SDK   │  │
│  │ - Traces   │  │     │  │ - Traces   │  │
│  │ - Metrics  │  │     │  │ - Metrics  │  │
│  └─────┬──────┘  │     │  └─────┬──────┘  │
└────────┼─────────┘     └────────┼─────────┘
         │                        │
         │    OTLP/gRPC          │
         └───────────┬───────────┘
                     ▼
           ┌─────────────────┐
           │  OTel Collector │
           │  - Batch        │
           │  - Export       │
           └────────┬────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌───────────────┐       ┌───────────────┐
│  Prometheus   │       │    Jaeger     │
│  (Metrics)    │       │   (Traces)    │
└───────┬───────┘       └───────────────┘
        │
        ▼
┌───────────────┐
│    Grafana    │
│  (Dashboards) │
└───────────────┘
```

## Kafka Latency Tracking

### How It Works

1. **Producer** adds `produced_at` timestamp to message payload
2. **Consumer** calculates `latency = time.Since(produced_at)`
3. Latency recorded as histogram metric with topic label

### Message Structure

```go
type SumMessage struct {
    Sum        int       `json:"sum"`
    ProducedAt time.Time `json:"produced_at"`  // For latency calculation
    TraceID    string    `json:"trace_id"`     // For trace correlation
    SpanID     string    `json:"span_id"`
}
```

### Latency Metric

```go
kafka.message.delivery.latency (histogram)
  Labels: topic
  Buckets: 1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000 ms
```

## Files to Create

### Shared Package

| File | Description |
|------|-------------|
| `pkg/telemetry/telemetry.go` | OTel provider initialization |
| `pkg/telemetry/kafka_metrics.go` | Kafka-specific metrics |

### Adder Service

| File | Changes |
|------|---------|
| `cmd/grpc/main.go` | Add OTel init, metrics endpoint |
| `internal/kafka/producer.go` | Add timestamp, trace context |

### Totalizer Service

| File | Changes |
|------|---------|
| `cmd/api/main.go` | Add OTel init, HTTP middleware |
| `internal/kafka/consumer.go` | Calculate and record latency |

## Key Code

### Telemetry Provider

```go
// pkg/telemetry/telemetry.go
func NewProvider(ctx context.Context, cfg Config) (*Provider, error) {
    res, _ := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(cfg.ServiceName),
            semconv.ServiceVersion(cfg.ServiceVersion),
        ),
    )

    traceExporter, _ := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(cfg.OTelEndpoint),
        otlptracegrpc.WithInsecure(),
    )

    metricExporter, _ := otlpmetricgrpc.New(ctx,
        otlpmetricgrpc.WithEndpoint(cfg.OTelEndpoint),
        otlpmetricgrpc.WithInsecure(),
    )

    // Configure providers...
    otel.SetTracerProvider(tracerProvider)
    otel.SetMeterProvider(meterProvider)

    return &Provider{...}, nil
}
```

### Kafka Metrics

```go
// pkg/telemetry/kafka_metrics.go
type KafkaMetrics struct {
    messageLatency   metric.Float64Histogram
    messagesProduced metric.Int64Counter
    messagesConsumed metric.Int64Counter
    producerErrors   metric.Int64Counter
    consumerErrors   metric.Int64Counter
}

func (m *KafkaMetrics) RecordMessageConsumed(ctx context.Context, topic string, producedAt time.Time) {
    latencyMs := float64(time.Since(producedAt).Milliseconds())

    m.messagesConsumed.Add(ctx, 1, metric.WithAttributes(attribute.String("topic", topic)))
    m.messageLatency.Record(ctx, latencyMs, metric.WithAttributes(attribute.String("topic", topic)))
}
```

### Instrumented Producer

```go
func (p *KafkaProducer) Publish(ctx context.Context, sum int) error {
    ctx, span := p.tracer.Start(ctx, "kafka.produce")
    defer span.End()

    spanCtx := trace.SpanContextFromContext(ctx)

    msg := SumMessage{
        Sum:        sum,
        ProducedAt: time.Now().UTC(),
        TraceID:    spanCtx.TraceID().String(),
        SpanID:     spanCtx.SpanID().String(),
    }

    // Publish and record metric...
    p.metrics.RecordMessageProduced(ctx, p.topic)
    return nil
}
```

### Instrumented Consumer

```go
func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) {
    ctx, span := c.tracer.Start(ctx, "kafka.consume")
    defer span.End()

    var sumMsg SumMessage
    json.Unmarshal(msg.Value, &sumMsg)

    // Record latency metric
    c.metrics.RecordMessageConsumed(ctx, c.topic, sumMsg.ProducedAt)

    // Process message...
}
```

## Metrics Endpoints

| Service | Port | Path |
|---------|------|------|
| Adder | 9090 | /metrics |
| Totalizer | 9091 | /metrics |

## Custom Metrics Summary

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `kafka.message.delivery.latency` | Histogram | topic | End-to-end latency (ms) |
| `kafka.messages.produced.total` | Counter | topic | Messages published |
| `kafka.messages.consumed.total` | Counter | topic | Messages consumed |
| `kafka.producer.errors.total` | Counter | topic, error_type | Producer failures |
| `kafka.consumer.errors.total` | Counter | topic, error_type | Consumer failures |
| `kafka.consumer.lag` | Gauge | topic, partition | Consumer lag |

## Alert Rules

```yaml
# P99 latency > 500ms for 2 minutes
- alert: KafkaMessageLatencyHigh
  expr: histogram_quantile(0.99, rate(kafka_message_delivery_latency_bucket[5m])) > 500
  for: 2m
  labels:
    severity: warning

# P99 latency > 1000ms (critical)
- alert: KafkaMessageLatencyCritical
  expr: histogram_quantile(0.99, rate(kafka_message_delivery_latency_bucket[5m])) > 1000
  for: 1m
  labels:
    severity: critical

# Consumer lag > 1000 messages
- alert: KafkaConsumerLagHigh
  expr: kafka_consumer_lag > 1000
  for: 5m
  labels:
    severity: warning
```

## Dependencies

```go
// Add to go.mod
go.opentelemetry.io/otel v1.24.0
go.opentelemetry.io/otel/sdk v1.24.0
go.opentelemetry.io/otel/sdk/metric v1.24.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.24.0
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.24.0
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.49.0
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0
github.com/prometheus/client_golang v1.19.0
```

## Testing Checklist

- [ ] Traces appear in Jaeger for gRPC calls
- [ ] Traces appear for Kafka produce/consume
- [ ] `kafka.message.delivery.latency` histogram populated in Prometheus
- [ ] Latency dashboard shows P50/P95/P99 correctly
- [ ] Alert fires when latency threshold exceeded
- [ ] Service health endpoints return 200
