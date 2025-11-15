# SumFlow

**SumFlow** is a distributed, event-driven system composed of two Go microservices running inside a Kubernetes environment. It demonstrates a complete asynchronous data pipeline using gRPC, REST, Kafka, and the Outbox Pattern, with full observability and external load testing support.

## Overview

SumFlow consists of two core services:

### **1. Adder Service (gRPC)**

A compute-oriented microservice that receives two numbers via gRPC, calculates their sum, and publishes the result as an event to Kafka. It acts as the producer in the event pipeline and uses the Outbox Pattern to guarantee reliable message delivery.

### **2. Totalizer Service (REST)**

A REST API service that consumes sum events from Kafka, adds each incoming value to an internal running total, and persists that total to a file-based store. The service exposes an HTTP endpoint to retrieve the current aggregated total. It functions as the consumer in the pipeline.

## Architecture

SumFlow uses Kafka as the messaging backbone between the services, enabling asynchronous communication and providing scalability and fault tolerance. The system is deployed on Kubernetes and integrates with Prometheus and Grafana for metrics collection and visualization, including queue latency metrics. Performance and stress testing are conducted externally using k6.

## Key Features

- Go-based microservices with clear separation of responsibilities
- gRPC communication for fast, typed requests
- REST API interface for retrieving system state
- Kafka-based event streaming
- Outbox Pattern for reliable event publishing
- File-backed persistent totalizer storage
- Kubernetes deployment-ready
- Full observability with Prometheus and Grafana
- External load testing with k6
