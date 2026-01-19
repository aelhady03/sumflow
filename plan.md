I want to add the following architecture:

- K8s cluster that have the following components:
  - Adder service
  - totalizer service
  - k6 load testing tool outside of the cluster to test the adder service and totalizer service.
  - Prometheus, Grafana with opentelemetry standard for monitoring the services (will have agent for each service).
  - Kafka cluster for communication between adder service and totalizer service.
  - outbox pattern implementation for reliable message delivery between adder service and totalizer service.
  - we should show a metric for latency of kafka message delivery from adder service to totalizer service.
