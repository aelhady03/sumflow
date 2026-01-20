# Phase 2: OpenTelemetry Observability Implementation Plan

## Overview

Add distributed tracing (OTel → Jaeger) and metrics (Prometheus) to Adder and Totalizer services, with Kafka message delivery latency tracking.

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│  Adder Service  │         │Totalizer Service│
│  gRPC :50051    │         │  HTTP :8080     │
│  metrics :9090  │         │  /metrics       │
└────────┬────────┘         └────────┬────────┘
         │ OTLP/gRPC                 │ OTLP/gRPC
         └───────────┬───────────────┘
                     ▼
              ┌──────────────┐
              │OTel Collector│
              │    :4317     │
              └──────┬───────┘
                     ▼
              ┌──────────────┐
              │    Jaeger    │  (traces)
              │   :16686     │
              └──────────────┘

Prometheus scrapes :9090 (adder) and :8080/metrics (totalizer)
```

## Design Decisions

1. **OTel for traces, Prometheus client for metrics** - simpler than routing metrics through OTel Collector
2. **Trace context in Kafka headers** - W3C TraceContext propagation
3. **Two latency metrics**:
   - `event_processing_latency` — uses `created_at` (full lifecycle: creation → consumer)
   - `kafka_delivery_latency` — uses new `published_at` field (Kafka-only: publish → consumer)

## Dependencies (via `go get`)

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/otel/propagation
go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

## Files to Create/Modify

### New Files

| File | Description |
|------|-------------|
| `pkg/telemetry/telemetry.go` | OTel tracer provider initialization |
| `pkg/telemetry/kafka_metrics.go` | Prometheus metrics for Kafka |
| `otel-collector-config.yaml` | Collector config (OTLP → Jaeger) |
| `prometheus.yml` | Prometheus scrape config |

### Modified Files

| File | Changes |
|------|---------|
| `adder/internal/outbox/event.go` | Add `PublishedAt` to JSON serialization (change `json:"-"` to `json:"published_at,omitempty"`) |
| `adder/cmd/grpc/main.go` | Add OTel init, metrics HTTP server on :9090, gRPC interceptors |
| `adder/internal/kafka/producer.go` | Set `published_at` timestamp, add tracing span, inject trace context |
| `totalizer/internal/kafka/consumer.go` | Add `PublishedAt` field to Event struct, extract trace context, record both latency metrics |
| `totalizer/cmd/api/main.go` | Add OTel init |
| `totalizer/cmd/api/routes.go` | Add `/metrics` endpoint |
| `docker-compose.yml` | Add otel-collector, jaeger, prometheus services |

## Key Implementation Details

### 1. Telemetry Provider (`pkg/telemetry/telemetry.go`)

```go
func InitTracer(ctx context.Context, cfg Config) (shutdown func(context.Context) error, error)
```
- Creates OTLP gRPC exporter
- Sets up TracerProvider with batching
- Configures W3C TraceContext propagator

### 2. Kafka Metrics (`pkg/telemetry/kafka_metrics.go`)

```go
var EventProcessingLatency = promauto.NewHistogramVec(...)   // created_at → consumer (full lifecycle)
var KafkaDeliveryLatency = promauto.NewHistogramVec(...)     // published_at → consumer (Kafka only)
var KafkaMessagesProduced = promauto.NewCounterVec(...)
var KafkaMessagesConsumed = promauto.NewCounterVec(...)
```

Buckets: 1ms, 5ms, 10ms, 25ms, 50ms, 100ms, 250ms, 500ms, 1s, 2.5s, 5s, 10s

### 3. Kafka Header Carrier (for trace propagation)

```go
type kafkaHeaderCarrier []kafka.Header
func (c *kafkaHeaderCarrier) Get(key string) string
func (c *kafkaHeaderCarrier) Set(key, value string)
func (c *kafkaHeaderCarrier) Keys() []string
```

### 4. Producer Instrumentation (`adder/internal/kafka/producer.go`)

- Set `event.PublishedAt = time.Now().UTC()` before serializing
- Start span `kafka.produce` with SpanKindProducer
- Inject trace context: `otel.GetTextMapPropagator().Inject(ctx, &headers)`
- Record `kafka_messages_produced_total` counter

### 5. Consumer Instrumentation (`totalizer/internal/kafka/consumer.go`)

- Extract trace context: `otel.GetTextMapPropagator().Extract(ctx, carrier)`
- Start span `kafka.consume` with SpanKindConsumer (linked to producer)
- Record both latencies:
  - `event_processing_latency_seconds` = `time.Since(event.CreatedAt)`
  - `kafka_delivery_latency_seconds` = `time.Since(event.PublishedAt)`
- Record `kafka_messages_consumed_total` counter

## Metrics Summary

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `event_processing_latency_seconds` | Histogram | topic, event_type | Full lifecycle: event creation → consumer |
| `kafka_delivery_latency_seconds` | Histogram | topic, event_type | Kafka only: publish → consumer |
| `kafka_messages_produced_total` | Counter | topic, status | Messages sent to Kafka |
| `kafka_messages_consumed_total` | Counter | topic, event_type, status | Messages consumed (success/error/duplicate) |

## Docker Compose Additions

```yaml
otel-collector:
  image: otel/opentelemetry-collector-contrib:latest
  ports: ["4317:4317"]

jaeger:
  image: jaegertracing/jaeger:latest
  ports: ["16686:16686"]

prometheus:
  image: prom/prometheus:latest
  ports: ["9099:9090"]
```

## Implementation Sequence

1. Create `pkg/telemetry/` package (telemetry.go, kafka_metrics.go)
2. Update Adder service (main.go, producer.go)
3. Update Totalizer service (main.go, routes.go, consumer.go)
4. Add infrastructure configs (otel-collector-config.yaml, prometheus.yml)
5. Update docker-compose.yml

## Verification

1. Start services: `docker-compose up --build`
2. Make gRPC request to Adder (e.g., via grpcurl or test client)
3. **Check Jaeger UI** at http://localhost:16686:
   - Trace should show: `grpc.server/SumNumbers` → `kafka.produce` → `kafka.consume`
   - Spans linked via trace context propagation
4. **Check Prometheus** at http://localhost:9099 - queries:
   - `event_processing_latency_seconds_bucket` (full lifecycle latency)
   - `kafka_delivery_latency_seconds_bucket` (Kafka-only latency)
   - `kafka_messages_produced_total`
   - `kafka_messages_consumed_total`
5. **Verify metrics endpoints**:
   - `curl http://localhost:9090/metrics` (adder)
   - `curl http://localhost:8080/metrics` (totalizer)
