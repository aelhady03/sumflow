# Adder Service

The **Adder Service** is a gRPC-based microservice responsible for receiving two numbers, computing their sum, and publishing the result as an event to Kafka. It functions as the producer in the SumFlow pipeline and uses the Outbox Pattern to ensure reliable message delivery.

## Key Features

- Receives two numbers via gRPC
- Calculates the sum of the numbers
- Publishes the result to a Kafka topic
- Uses the Outbox Pattern for reliable event publishing
- Designed for deployment inside a Kubernetes cluster
- Integrates with Prometheus and Grafana for observability

## Endpoint

- **gRPC Service**: `SumNumbers(a int32, b int32) -> sum int32`
