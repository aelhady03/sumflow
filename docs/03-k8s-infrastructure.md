# Phase 3: Kubernetes Infrastructure

## Overview

Deploy foundational K8s infrastructure: namespaces, Kafka cluster (Strimzi), and PostgreSQL databases.

## Namespace Organization

```
sumflow           # Application services (adder, totalizer)
sumflow-kafka     # Kafka infrastructure (isolated for stateful workloads)
sumflow-monitoring # Observability stack (Prometheus, Grafana)
```

## Directory Structure

```
k8s/
├── base/
│   └── namespace.yaml
├── infrastructure/
│   ├── kafka/
│   │   ├── strimzi-operator.yaml
│   │   ├── kafka-cluster.yaml
│   │   └── kafka-topic.yaml
│   └── postgres/
│       ├── adder-postgres.yaml
│       └── totalizer-postgres.yaml
```

## Files to Create

### Namespaces

```yaml
# k8s/base/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: sumflow
  labels:
    name: sumflow
---
apiVersion: v1
kind: Namespace
metadata:
  name: sumflow-kafka
---
apiVersion: v1
kind: Namespace
metadata:
  name: sumflow-monitoring
```

### Strimzi Kafka Operator

```yaml
# k8s/infrastructure/kafka/strimzi-operator.yaml
# Install via: kubectl create -f 'https://strimzi.io/install/latest?namespace=sumflow-kafka'
```

### Kafka Cluster

```yaml
# k8s/infrastructure/kafka/kafka-cluster.yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: Kafka
metadata:
  name: sumflow-kafka
  namespace: sumflow-kafka
spec:
  kafka:
    version: 3.7.0
    replicas: 3
    listeners:
      - name: plain
        port: 9092
        type: internal
        tls: false
    config:
      offsets.topic.replication.factor: 3
      transaction.state.log.replication.factor: 3
      default.replication.factor: 3
      min.insync.replicas: 2
      auto.create.topics.enable: false
    storage:
      type: persistent-claim
      size: 10Gi
      class: standard
    resources:
      requests:
        memory: 2Gi
        cpu: 500m
      limits:
        memory: 4Gi
        cpu: 2
  zookeeper:
    replicas: 3
    storage:
      type: persistent-claim
      size: 5Gi
      class: standard
    resources:
      requests:
        memory: 512Mi
        cpu: 250m
  entityOperator:
    topicOperator: {}
    userOperator: {}
```

### Kafka Topic

```yaml
# k8s/infrastructure/kafka/kafka-topic.yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: KafkaTopic
metadata:
  name: sums
  namespace: sumflow-kafka
  labels:
    strimzi.io/cluster: sumflow-kafka
spec:
  partitions: 6
  replicas: 3
  config:
    retention.ms: 604800000  # 7 days
    min.insync.replicas: 2
```

### PostgreSQL for Adder

```yaml
# k8s/infrastructure/postgres/adder-postgres.yaml
apiVersion: v1
kind: Secret
metadata:
  name: adder-postgres-secret
  namespace: sumflow
type: Opaque
stringData:
  POSTGRES_USER: adder
  POSTGRES_PASSWORD: adder-secret-change-me
  POSTGRES_DB: adder
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: adder-postgres-pvc
  namespace: sumflow
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: adder-postgres
  namespace: sumflow
spec:
  serviceName: adder-postgres
  replicas: 1
  selector:
    matchLabels:
      app: adder-postgres
  template:
    metadata:
      labels:
        app: adder-postgres
    spec:
      containers:
        - name: postgres
          image: postgres:16-alpine
          ports:
            - containerPort: 5432
          envFrom:
            - secretRef:
                name: adder-postgres-secret
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              memory: 256Mi
              cpu: 100m
            limits:
              memory: 512Mi
              cpu: 500m
          livenessProbe:
            exec:
              command: ["pg_isready", "-U", "adder"]
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            exec:
              command: ["pg_isready", "-U", "adder"]
            initialDelaySeconds: 5
            periodSeconds: 5
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: adder-postgres-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: adder-postgres
  namespace: sumflow
spec:
  type: ClusterIP
  ports:
    - port: 5432
  selector:
    app: adder-postgres
```

### PostgreSQL for Totalizer

```yaml
# k8s/infrastructure/postgres/totalizer-postgres.yaml
# Similar structure to adder-postgres, with:
# - name: totalizer-postgres
# - secret: totalizer-postgres-secret
# - user/db: totalizer
```

## Service Discovery

| Service | DNS | Port |
|---------|-----|------|
| Kafka Bootstrap | `sumflow-kafka-kafka-bootstrap.sumflow-kafka.svc.cluster.local` | 9092 |
| Adder Postgres | `adder-postgres.sumflow.svc.cluster.local` | 5432 |
| Totalizer Postgres | `totalizer-postgres.sumflow.svc.cluster.local` | 5432 |

## Deployment Order

1. Create namespaces
   ```bash
   kubectl apply -f k8s/base/namespace.yaml
   ```

2. Install Strimzi operator
   ```bash
   kubectl create -f 'https://strimzi.io/install/latest?namespace=sumflow-kafka'
   ```

3. Wait for operator to be ready
   ```bash
   kubectl wait --for=condition=ready pod -l name=strimzi-cluster-operator -n sumflow-kafka --timeout=300s
   ```

4. Deploy Kafka cluster
   ```bash
   kubectl apply -f k8s/infrastructure/kafka/kafka-cluster.yaml
   ```

5. Wait for Kafka to be ready
   ```bash
   kubectl wait kafka/sumflow-kafka --for=condition=Ready --timeout=600s -n sumflow-kafka
   ```

6. Create Kafka topic
   ```bash
   kubectl apply -f k8s/infrastructure/kafka/kafka-topic.yaml
   ```

7. Deploy PostgreSQL instances
   ```bash
   kubectl apply -f k8s/infrastructure/postgres/
   ```

## Verification Commands

```bash
# Check namespaces
kubectl get ns | grep sumflow

# Check Kafka cluster status
kubectl get kafka -n sumflow-kafka

# Check Kafka pods
kubectl get pods -n sumflow-kafka

# Check topic
kubectl get kafkatopic -n sumflow-kafka

# Check PostgreSQL
kubectl get pods -n sumflow -l app=adder-postgres
kubectl get pods -n sumflow -l app=totalizer-postgres

# Test Kafka connectivity
kubectl run kafka-test --rm -i --tty --image=bitnami/kafka -- \
  kafka-console-producer.sh --broker-list sumflow-kafka-kafka-bootstrap.sumflow-kafka:9092 --topic sums
```

## Resource Summary

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|-------------|-----------|----------------|--------------|
| Kafka Broker (x3) | 500m | 2000m | 2Gi | 4Gi |
| Zookeeper (x3) | 250m | 500m | 512Mi | 1Gi |
| Adder Postgres | 100m | 500m | 256Mi | 512Mi |
| Totalizer Postgres | 100m | 500m | 256Mi | 512Mi |

## Testing Checklist

- [ ] All namespaces created
- [ ] Strimzi operator running
- [ ] Kafka cluster healthy (3 brokers)
- [ ] Zookeeper ensemble healthy (3 nodes)
- [ ] Topic "sums" created with 6 partitions
- [ ] Both PostgreSQL instances accepting connections
- [ ] Can produce/consume test messages
