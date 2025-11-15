# Totalizer Service

The **Totalizer Service** is a REST-based microservice responsible for consuming sum events from Kafka and maintaining a running total. Each incoming sum is added to the current stored value, and the updated result is persisted to a local file. The service exposes an HTTP endpoint to retrieve the latest total at any time.

## Key Features

- Consumes sum messages from Kafka
- Aggregates values into a persistent file-based store
- Exposes the `/result` endpoint to return the current total
- Designed for deployment inside a Kubernetes cluster with full observability support
