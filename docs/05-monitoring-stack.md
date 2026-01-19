# Phase 5: Monitoring Stack

## Overview

Deploy OpenTelemetry Collector, Prometheus, and Grafana with pre-configured dashboards for Kafka latency monitoring.

## Architecture

```
┌──────────────┐  ┌──────────────┐
│    Adder     │  │  Totalizer   │
│   :9090      │  │   :9091      │
└──────┬───────┘  └──────┬───────┘
       │ OTLP            │ OTLP
       └────────┬────────┘
                ▼
       ┌────────────────┐
       │ OTel Collector │
       │     :4317      │
       └───────┬────────┘
               │
       ┌───────┴───────┐
       ▼               ▼
┌────────────┐  ┌────────────┐
│ Prometheus │  │   Jaeger   │
│   :9090    │  │  :16686    │
└─────┬──────┘  └────────────┘
      │
      ▼
┌────────────┐
│  Grafana   │
│   :3000    │
└────────────┘
```

## Directory Structure

```
k8s/observability/
├── otel-collector.yaml
├── prometheus.yaml
├── grafana.yaml
├── jaeger.yaml (optional)
└── dashboards/
    └── kafka-latency.json
```

## OpenTelemetry Collector

```yaml
# k8s/observability/otel-collector.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
  namespace: sumflow-monitoring
data:
  config.yaml: |
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: 0.0.0.0:4317
          http:
            endpoint: 0.0.0.0:4318

    processors:
      batch:
        timeout: 10s
        send_batch_size: 1024
      memory_limiter:
        check_interval: 5s
        limit_mib: 512

    exporters:
      prometheus:
        endpoint: "0.0.0.0:8889"
        resource_to_telemetry_conversion:
          enabled: true
      debug:
        verbosity: detailed

    service:
      pipelines:
        traces:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [debug]
        metrics:
          receivers: [otlp]
          processors: [memory_limiter, batch]
          exporters: [prometheus]
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
  namespace: sumflow-monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otel-collector
  template:
    metadata:
      labels:
        app: otel-collector
    spec:
      containers:
        - name: collector
          image: otel/opentelemetry-collector-contrib:0.96.0
          args: ["--config=/etc/otel/config.yaml"]
          ports:
            - containerPort: 4317  # OTLP gRPC
            - containerPort: 4318  # OTLP HTTP
            - containerPort: 8889  # Prometheus exporter
          volumeMounts:
            - name: config
              mountPath: /etc/otel/config.yaml
              subPath: config.yaml
          resources:
            limits:
              cpu: 500m
              memory: 512Mi
            requests:
              cpu: 100m
              memory: 256Mi
      volumes:
        - name: config
          configMap:
            name: otel-collector-config
---
apiVersion: v1
kind: Service
metadata:
  name: otel-collector
  namespace: sumflow-monitoring
spec:
  ports:
    - name: otlp-grpc
      port: 4317
    - name: otlp-http
      port: 4318
    - name: prometheus
      port: 8889
  selector:
    app: otel-collector
```

## Prometheus

```yaml
# k8s/observability/prometheus.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: sumflow-monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s

    rule_files:
      - /etc/prometheus/rules/*.yml

    scrape_configs:
      - job_name: 'otel-collector'
        static_configs:
          - targets: ['otel-collector:8889']

      - job_name: 'adder'
        kubernetes_sd_configs:
          - role: pod
            namespaces:
              names: [sumflow]
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_label_app]
            action: keep
            regex: adder
          - source_labels: [__meta_kubernetes_pod_container_port_name]
            action: keep
            regex: metrics

      - job_name: 'totalizer'
        kubernetes_sd_configs:
          - role: pod
            namespaces:
              names: [sumflow]
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_label_app]
            action: keep
            regex: totalizer
          - source_labels: [__meta_kubernetes_pod_container_port_name]
            action: keep
            regex: metrics

  alerts.yml: |
    groups:
      - name: kafka_latency
        rules:
          - alert: KafkaLatencyHigh
            expr: histogram_quantile(0.99, rate(kafka_message_delivery_latency_bucket[5m])) > 500
            for: 2m
            labels:
              severity: warning
            annotations:
              summary: "Kafka P99 latency > 500ms"

          - alert: KafkaLatencyCritical
            expr: histogram_quantile(0.99, rate(kafka_message_delivery_latency_bucket[5m])) > 1000
            for: 1m
            labels:
              severity: critical
            annotations:
              summary: "Kafka P99 latency > 1000ms"

          - alert: KafkaConsumerLag
            expr: kafka_consumer_lag > 1000
            for: 5m
            labels:
              severity: warning
            annotations:
              summary: "Consumer lag > 1000 messages"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: sumflow-monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      serviceAccountName: prometheus
      containers:
        - name: prometheus
          image: prom/prometheus:v2.50.1
          args:
            - --config.file=/etc/prometheus/prometheus.yml
            - --storage.tsdb.path=/prometheus
            - --web.enable-lifecycle
          ports:
            - containerPort: 9090
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
            - name: storage
              mountPath: /prometheus
          resources:
            limits:
              cpu: 500m
              memory: 1Gi
      volumes:
        - name: config
          configMap:
            name: prometheus-config
        - name: storage
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  namespace: sumflow-monitoring
spec:
  ports:
    - port: 9090
  selector:
    app: prometheus
```

## Grafana

```yaml
# k8s/observability/grafana.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-datasources
  namespace: sumflow-monitoring
data:
  datasources.yaml: |
    apiVersion: 1
    datasources:
      - name: Prometheus
        type: prometheus
        url: http://prometheus:9090
        isDefault: true
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards-provider
  namespace: sumflow-monitoring
data:
  dashboards.yaml: |
    apiVersion: 1
    providers:
      - name: default
        folder: SumFlow
        type: file
        options:
          path: /var/lib/grafana/dashboards
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
  namespace: sumflow-monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grafana
  template:
    metadata:
      labels:
        app: grafana
    spec:
      containers:
        - name: grafana
          image: grafana/grafana:10.3.3
          ports:
            - containerPort: 3000
          env:
            - name: GF_SECURITY_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: grafana-secret
                  key: admin-password
          volumeMounts:
            - name: datasources
              mountPath: /etc/grafana/provisioning/datasources
            - name: dashboards-provider
              mountPath: /etc/grafana/provisioning/dashboards
            - name: dashboards
              mountPath: /var/lib/grafana/dashboards
          resources:
            limits:
              cpu: 500m
              memory: 512Mi
      volumes:
        - name: datasources
          configMap:
            name: grafana-datasources
        - name: dashboards-provider
          configMap:
            name: grafana-dashboards-provider
        - name: dashboards
          configMap:
            name: grafana-dashboards
---
apiVersion: v1
kind: Service
metadata:
  name: grafana
  namespace: sumflow-monitoring
spec:
  type: ClusterIP
  ports:
    - port: 3000
  selector:
    app: grafana
```

## Kafka Latency Dashboard

Key panels:
1. **P50/P95/P99 Latency** - Time series graph
2. **Message Throughput** - Produced vs consumed rate
3. **Consumer Lag** - Per partition
4. **Error Rates** - Producer and consumer errors
5. **Stat Panels** - Current P50, P95, P99, messages/sec

PromQL queries:
```promql
# P99 latency
histogram_quantile(0.99, rate(kafka_message_delivery_latency_bucket{topic="sums"}[5m]))

# Message throughput
rate(kafka_messages_produced_total{topic="sums"}[5m])
rate(kafka_messages_consumed_total{topic="sums"}[5m])

# Consumer lag
kafka_consumer_lag{topic="sums"}

# Error rate
rate(kafka_producer_errors_total{topic="sums"}[5m])
rate(kafka_consumer_errors_total{topic="sums"}[5m])
```

## Deployment Commands

```bash
# Create namespace if not exists
kubectl create ns sumflow-monitoring --dry-run=client -o yaml | kubectl apply -f -

# Create Grafana secret
kubectl create secret generic grafana-secret \
  --from-literal=admin-password=changeme123 \
  -n sumflow-monitoring

# Deploy stack
kubectl apply -f k8s/observability/

# Port forward for access
kubectl port-forward svc/grafana 3000:3000 -n sumflow-monitoring
kubectl port-forward svc/prometheus 9090:9090 -n sumflow-monitoring
```

## Service Endpoints

| Service | Internal DNS | Port |
|---------|--------------|------|
| OTel Collector | `otel-collector.sumflow-monitoring.svc.cluster.local` | 4317, 4318, 8889 |
| Prometheus | `prometheus.sumflow-monitoring.svc.cluster.local` | 9090 |
| Grafana | `grafana.sumflow-monitoring.svc.cluster.local` | 3000 |

## Testing Checklist

- [ ] OTel Collector receiving traces and metrics
- [ ] Prometheus scraping all targets
- [ ] Grafana accessible with dashboards
- [ ] Kafka latency histogram visible
- [ ] Alert rules loaded in Prometheus
- [ ] P50/P95/P99 displaying correctly
- [ ] Consumer lag tracking working
