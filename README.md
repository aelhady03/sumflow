# SumFlow

**SumFlow** is a distributed, event-driven system composed of two Go microservices. It demonstrates a complete asynchronous data pipeline using gRPC, REST, Kafka, and the Outbox Pattern, with distributed tracing and metrics collection.

## Overview

SumFlow consists of two core services:

### **1. Adder Service (gRPC)**

A compute-oriented microservice that receives two numbers via gRPC, calculates their sum, and publishes the result as an event to Kafka. It acts as the producer in the event pipeline and uses the Outbox Pattern to guarantee reliable message delivery.

### **2. Totalizer Service (REST)**

A REST API service that consumes sum events from Kafka, adds each incoming value to an internal running total, and persists that total to PostgreSQL. The service exposes an HTTP endpoint to retrieve the current aggregated total and supports idempotent event processing with deduplication.

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│  Adder Service  │         │Totalizer Service│
│  gRPC :50051    │         │  HTTP :8080     │
│  metrics :9090  │         │  /metrics       │
└────────┬────────┘         └────────┬────────┘
         │                           │
         │    ┌───────────┐          │
         └───>│   Kafka   │<─────────┘
              │  "sums"   │
              └─────┬─────┘
                    │
         ┌──────────┴──────────┐
         │ OTLP/gRPC           │ OTLP/gRPC
         ▼                     ▼
  ┌──────────────┐      ┌────────────┐
  │OTel Collector│─────>│   Jaeger   │ (traces)
  │    :4317     │      │   :16686   │
  └──────────────┘      └────────────┘

  Prometheus scrapes :9090 (adder) and :8080/metrics (totalizer)
```

## Implemented Features

### Phase 1: Outbox Pattern
- Transactional outbox for reliable, exactly-once message delivery
- Background relay polling unpublished events from PostgreSQL
- Consumer-side deduplication with processed events tracking
- PostgreSQL storage for totals (replacing file-based storage)

### Phase 2: OpenTelemetry Observability
- Distributed tracing with OpenTelemetry → Jaeger
- W3C TraceContext propagation through Kafka headers
- Prometheus metrics for Kafka latency tracking:
  - `event_processing_latency_seconds` — full lifecycle (creation → consumer)
  - `kafka_delivery_latency_seconds` — Kafka-only (publish → consumer)
  - `kafka_messages_produced_total` / `kafka_messages_consumed_total`

## Quick Start

```bash
# Start all services
docker compose up --build -d

# Send a gRPC request
grpcurl -plaintext -d '{"x": 5, "y": 3}' localhost:50051 sum.SumNumbersService/SumNumbers

# Check the running total
curl http://localhost:8080/v1/results

# View traces
open http://localhost:16686

# View metrics
open http://localhost:9099
```

## Endpoints

| Service | Endpoint | Description |
|---------|----------|-------------|
| Adder gRPC | `localhost:50051` | `sum.SumNumbersService/SumNumbers` |
| Adder Metrics | `localhost:9090/metrics` | Prometheus metrics |
| Totalizer API | `localhost:8080/v1/results` | Get current total |
| Totalizer Metrics | `localhost:8080/metrics` | Prometheus metrics |
| Jaeger UI | `localhost:16686` | Distributed traces |
| Prometheus | `localhost:9099` | Metrics queries |

## Roadmap

The following features are planned for future implementation:

### Phase 3: Kubernetes Infrastructure
- Namespace organization (sumflow, sumflow-kafka, sumflow-monitoring)
- Strimzi Kafka operator deployment
- PostgreSQL StatefulSets with persistent storage

### Phase 4: Kubernetes Service Deployments
- Deployment manifests with health checks
- Horizontal Pod Autoscaler (HPA)
- Pod Disruption Budgets (PDB)
- Security contexts and RBAC

### Phase 5: Kubernetes Monitoring Stack
- OTel Collector, Prometheus, Grafana on K8s
- Pre-configured Kafka latency dashboards
- Alerting rules for latency and consumer lag

### Phase 6: Load Testing
- k6 test scripts for gRPC and HTTP endpoints
- Full end-to-end flow testing
- CI/CD integration for automated load tests

See the `docs/` directory for detailed implementation plans.
