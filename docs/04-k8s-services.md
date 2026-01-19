# Phase 4: Service Deployments

## Overview

Deploy Adder and Totalizer services to Kubernetes with proper configurations, health checks, and autoscaling.

## Directory Structure

```
k8s/services/
├── adder/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   ├── hpa.yaml
│   └── pdb.yaml
└── totalizer/
    ├── deployment.yaml
    ├── service.yaml
    ├── configmap.yaml
    ├── hpa.yaml
    └── pdb.yaml
```

## Adder Service

### ConfigMap

```yaml
# k8s/services/adder/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: adder-config
  namespace: sumflow
data:
  GRPC_PORT: "50051"
  METRICS_PORT: "9090"
  KAFKA_BROKERS: "sumflow-kafka-kafka-bootstrap.sumflow-kafka.svc.cluster.local:9092"
  KAFKA_TOPIC: "sums"
  OTEL_ENDPOINT: "otel-collector.sumflow-monitoring.svc.cluster.local:4317"
  RELAY_INTERVAL: "100ms"
  RELAY_BATCH_SIZE: "100"
```

### Deployment

```yaml
# k8s/services/adder/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: adder
  namespace: sumflow
  labels:
    app: adder
spec:
  replicas: 2
  selector:
    matchLabels:
      app: adder
  template:
    metadata:
      labels:
        app: adder
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: adder
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
        - name: adder
          image: ghcr.io/aelhady03/sumflow-adder:latest
          ports:
            - name: grpc
              containerPort: 50051
            - name: metrics
              containerPort: 9090
          envFrom:
            - configMapRef:
                name: adder-config
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: adder-db-credentials
                  key: DATABASE_URL
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /health
              port: 9090
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 9090
            initialDelaySeconds: 5
            periodSeconds: 5
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: [ALL]
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: adder
                topologyKey: kubernetes.io/hostname
```

### Service

```yaml
# k8s/services/adder/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: adder
  namespace: sumflow
spec:
  type: ClusterIP
  ports:
    - name: grpc
      port: 50051
      targetPort: 50051
    - name: metrics
      port: 9090
      targetPort: 9090
  selector:
    app: adder
```

### HPA

```yaml
# k8s/services/adder/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: adder-hpa
  namespace: sumflow
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: adder
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
    scaleUp:
      stabilizationWindowSeconds: 0
```

### PDB

```yaml
# k8s/services/adder/pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: adder-pdb
  namespace: sumflow
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: adder
```

## Totalizer Service

### ConfigMap

```yaml
# k8s/services/totalizer/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: totalizer-config
  namespace: sumflow
data:
  HTTP_PORT: "8080"
  METRICS_PORT: "9091"
  ENV: "production"
  KAFKA_BROKERS: "sumflow-kafka-kafka-bootstrap.sumflow-kafka.svc.cluster.local:9092"
  KAFKA_TOPIC: "sums"
  KAFKA_GROUP_ID: "totalizer-group"
  OTEL_ENDPOINT: "otel-collector.sumflow-monitoring.svc.cluster.local:4317"
```

### Deployment

```yaml
# k8s/services/totalizer/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: totalizer
  namespace: sumflow
  labels:
    app: totalizer
spec:
  replicas: 2
  selector:
    matchLabels:
      app: totalizer
  template:
    metadata:
      labels:
        app: totalizer
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9091"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: totalizer
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
        - name: totalizer
          image: ghcr.io/aelhady03/sumflow-totalizer:latest
          ports:
            - name: http
              containerPort: 8080
            - name: metrics
              containerPort: 9091
          envFrom:
            - configMapRef:
                name: totalizer-config
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: totalizer-db-credentials
                  key: DATABASE_URL
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /v1/healthcheck
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /v1/healthcheck
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop: [ALL]
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchLabels:
                    app: totalizer
                topologyKey: kubernetes.io/hostname
```

### Service

```yaml
# k8s/services/totalizer/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: totalizer
  namespace: sumflow
spec:
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: metrics
      port: 9091
      targetPort: 9091
  selector:
    app: totalizer
```

## Database Secrets

```yaml
# k8s/services/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: adder-db-credentials
  namespace: sumflow
type: Opaque
stringData:
  DATABASE_URL: postgres://adder:adder-secret-change-me@adder-postgres:5432/adder?sslmode=disable
---
apiVersion: v1
kind: Secret
metadata:
  name: totalizer-db-credentials
  namespace: sumflow
type: Opaque
stringData:
  DATABASE_URL: postgres://totalizer:totalizer-secret-change-me@totalizer-postgres:5432/totalizer?sslmode=disable
```

## Dockerfile Updates

### Adder Dockerfile

```dockerfile
# adder/Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /adder ./adder/cmd/grpc

FROM alpine:3.19
RUN adduser -D -u 1000 appuser
COPY --from=builder /adder /adder
USER appuser
EXPOSE 50051 9090
ENTRYPOINT ["/adder"]
```

### Totalizer Dockerfile

```dockerfile
# totalizer/Dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /totalizer ./totalizer/cmd/api

FROM alpine:3.19
RUN adduser -D -u 1000 appuser
COPY --from=builder /totalizer /totalizer
USER appuser
EXPOSE 8080 9091
ENTRYPOINT ["/totalizer"]
```

## Deployment Commands

```bash
# Create secrets first
kubectl apply -f k8s/services/secrets.yaml

# Deploy adder
kubectl apply -f k8s/services/adder/

# Deploy totalizer
kubectl apply -f k8s/services/totalizer/

# Verify deployments
kubectl get pods -n sumflow
kubectl get svc -n sumflow

# Check logs
kubectl logs -l app=adder -n sumflow
kubectl logs -l app=totalizer -n sumflow
```

## Service Endpoints

| Service | Internal DNS | Ports |
|---------|--------------|-------|
| Adder | `adder.sumflow.svc.cluster.local` | gRPC: 50051, Metrics: 9090 |
| Totalizer | `totalizer.sumflow.svc.cluster.local` | HTTP: 8080, Metrics: 9091 |

## Testing Checklist

- [ ] Adder pods running and ready
- [ ] Totalizer pods running and ready
- [ ] gRPC health check passing
- [ ] HTTP health check passing
- [ ] Metrics endpoints accessible
- [ ] Can make gRPC call to adder
- [ ] Can make HTTP call to totalizer
- [ ] HPA functioning (scale under load)
- [ ] PDB preventing total outage during updates
