# Phase 6: Load Testing with k6

## Overview

Set up k6 load testing outside the Kubernetes cluster to stress test the Adder (gRPC) and Totalizer (REST) services.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Outside K8s Cluster                      │
│  ┌─────────────┐                                            │
│  │     k6      │                                            │
│  │ Load Tester │                                            │
│  └──────┬──────┘                                            │
└─────────┼───────────────────────────────────────────────────┘
          │
          │ gRPC / HTTP
          ▼
┌─────────────────────────────────────────────────────────────┐
│                    K8s Cluster                              │
│  ┌─────────────────┐        ┌─────────────────┐            │
│  │  Adder Service  │        │ Totalizer Svc   │            │
│  │   (gRPC:50051)  │───────>│  (HTTP:8080)    │            │
│  └─────────────────┘ Kafka  └─────────────────┘            │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
k6/
├── scripts/
│   ├── adder-grpc-test.js
│   ├── totalizer-http-test.js
│   └── full-flow-test.js
├── lib/
│   └── grpc-client.js
└── README.md
```

## Prerequisites

```bash
# Install k6
# macOS
brew install k6

# Linux
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update
sudo apt-get install k6

# Install k6 gRPC extension (for gRPC testing)
# k6 has built-in gRPC support since v0.29.0
```

## Expose Services for Testing

```bash
# Option 1: Port forward (for local testing)
kubectl port-forward svc/adder 50051:50051 -n sumflow &
kubectl port-forward svc/totalizer 8080:8080 -n sumflow &

# Option 2: NodePort services (for CI/CD)
# Modify services to type: NodePort

# Option 3: Ingress/LoadBalancer (production-like)
```

## Test Scripts

### Adder gRPC Test

```javascript
// k6/scripts/adder-grpc-test.js
import grpc from 'k6/net/grpc';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const client = new grpc.Client();
client.load(['../proto'], 'sum.proto');

const grpcErrors = new Counter('grpc_errors');
const grpcDuration = new Trend('grpc_duration', true);

export const options = {
  scenarios: {
    constant_load: {
      executor: 'constant-arrival-rate',
      rate: 100,           // 100 RPS
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
    spike_test: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      stages: [
        { duration: '1m', target: 10 },
        { duration: '1m', target: 500 },   // Spike
        { duration: '1m', target: 10 },
      ],
      preAllocatedVUs: 100,
      maxVUs: 200,
      startTime: '6m',
    },
  },
  thresholds: {
    grpc_req_duration: ['p(95)<200', 'p(99)<500'],
    grpc_errors: ['count<10'],
  },
};

export default function () {
  const addr = __ENV.ADDER_ADDR || 'localhost:50051';

  client.connect(addr, { plaintext: true });

  const x = Math.floor(Math.random() * 100);
  const y = Math.floor(Math.random() * 100);

  const start = Date.now();
  const response = client.invoke('sum.SumNumbersService/SumNumbers', {
    x: x,
    y: y,
  });
  const duration = Date.now() - start;
  grpcDuration.add(duration);

  const success = check(response, {
    'status is OK': (r) => r && r.status === grpc.StatusOK,
    'sum is correct': (r) => r && r.message.sum === x + y,
  });

  if (!success) {
    grpcErrors.add(1);
    console.error(`Error: ${response.error}`);
  }

  client.close();
  sleep(0.01); // Small delay between requests
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'results/adder-grpc-results.json': JSON.stringify(data),
  };
}
```

### Totalizer HTTP Test

```javascript
// k6/scripts/totalizer-http-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const httpErrors = new Counter('http_errors');
const httpDuration = new Trend('http_duration', true);

export const options = {
  scenarios: {
    constant_load: {
      executor: 'constant-arrival-rate',
      rate: 200,           // 200 RPS
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
    stress_test: {
      executor: 'ramping-vus',
      startVUs: 10,
      stages: [
        { duration: '2m', target: 50 },
        { duration: '3m', target: 100 },
        { duration: '2m', target: 200 },
        { duration: '1m', target: 0 },
      ],
      startTime: '6m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<100', 'p(99)<200'],
    http_errors: ['count<10'],
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  const baseUrl = __ENV.TOTALIZER_URL || 'http://localhost:8080';

  // Test healthcheck
  const healthRes = http.get(`${baseUrl}/v1/healthcheck`);
  check(healthRes, {
    'healthcheck status 200': (r) => r.status === 200,
  });

  // Test results endpoint
  const start = Date.now();
  const resultsRes = http.get(`${baseUrl}/v1/results`);
  const duration = Date.now() - start;
  httpDuration.add(duration);

  const success = check(resultsRes, {
    'results status 200': (r) => r.status === 200,
    'has result field': (r) => {
      const body = JSON.parse(r.body);
      return body.hasOwnProperty('result');
    },
  });

  if (!success) {
    httpErrors.add(1);
  }

  sleep(0.01);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'results/totalizer-http-results.json': JSON.stringify(data),
  };
}
```

### Full Flow Test

```javascript
// k6/scripts/full-flow-test.js
import grpc from 'k6/net/grpc';
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend } from 'k6/metrics';

const client = new grpc.Client();
client.load(['../proto'], 'sum.proto');

const e2eLatency = new Trend('e2e_latency', true);

export const options = {
  scenarios: {
    full_flow: {
      executor: 'constant-arrival-rate',
      rate: 50,
      timeUnit: '1s',
      duration: '10m',
      preAllocatedVUs: 50,
      maxVUs: 100,
    },
  },
  thresholds: {
    e2e_latency: ['p(95)<2000', 'p(99)<5000'], // Full flow including Kafka
  },
};

export default function () {
  const adderAddr = __ENV.ADDER_ADDR || 'localhost:50051';
  const totalizerUrl = __ENV.TOTALIZER_URL || 'http://localhost:8080';

  // Get initial total
  const initialRes = http.get(`${totalizerUrl}/v1/results`);
  const initialTotal = JSON.parse(initialRes.body).result;

  // Make gRPC call to adder
  client.connect(adderAddr, { plaintext: true });

  const x = Math.floor(Math.random() * 10) + 1;
  const y = Math.floor(Math.random() * 10) + 1;
  const expectedSum = x + y;

  const start = Date.now();

  const grpcRes = client.invoke('sum.SumNumbersService/SumNumbers', { x, y });

  check(grpcRes, {
    'gRPC status OK': (r) => r && r.status === grpc.StatusOK,
  });

  client.close();

  // Poll totalizer until total increases (with timeout)
  let newTotal = initialTotal;
  let attempts = 0;
  const maxAttempts = 50; // 5 seconds max

  while (newTotal <= initialTotal && attempts < maxAttempts) {
    sleep(0.1);
    const res = http.get(`${totalizerUrl}/v1/results`);
    newTotal = JSON.parse(res.body).result;
    attempts++;
  }

  const e2eDuration = Date.now() - start;
  e2eLatency.add(e2eDuration);

  check(null, {
    'total increased': () => newTotal > initialTotal,
    'e2e completed in time': () => attempts < maxAttempts,
  });

  sleep(0.1);
}
```

## Running Tests

```bash
# Set environment variables
export ADDER_ADDR="<k8s-node-ip>:30051"
export TOTALIZER_URL="http://<k8s-node-ip>:30080"

# Run individual tests
k6 run k6/scripts/adder-grpc-test.js
k6 run k6/scripts/totalizer-http-test.js

# Run full flow test
k6 run k6/scripts/full-flow-test.js

# Run with more VUs
k6 run --vus 100 --duration 10m k6/scripts/adder-grpc-test.js

# Run with cloud output (if using k6 Cloud)
k6 run --out cloud k6/scripts/adder-grpc-test.js
```

## Expected Metrics

| Metric | Target | Description |
|--------|--------|-------------|
| gRPC P95 latency | < 200ms | Adder response time |
| gRPC P99 latency | < 500ms | Adder tail latency |
| HTTP P95 latency | < 100ms | Totalizer response time |
| HTTP P99 latency | < 200ms | Totalizer tail latency |
| E2E P95 latency | < 2s | Full flow including Kafka |
| Error rate | < 1% | Combined error rate |

## Observing Results

While running load tests, monitor:

1. **Grafana Dashboards**
   - Kafka message latency
   - Service request rates
   - Error rates
   - Resource utilization

2. **kubectl Commands**
   ```bash
   # Watch pod resources
   kubectl top pods -n sumflow

   # Watch HPA
   kubectl get hpa -n sumflow -w

   # Check pod logs for errors
   kubectl logs -l app=adder -n sumflow --tail=100 -f
   ```

3. **Prometheus Queries**
   ```promql
   # Request rate
   rate(grpc_server_handled_total[1m])

   # Error rate
   rate(grpc_server_handled_total{grpc_code!="OK"}[1m])
   ```

## CI/CD Integration

```yaml
# .github/workflows/load-test.yml
name: Load Test
on:
  workflow_dispatch:
  schedule:
    - cron: '0 2 * * *'  # Nightly

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup k6
        uses: grafana/k6-action@v0.3.1

      - name: Run load tests
        run: |
          k6 run k6/scripts/adder-grpc-test.js
          k6 run k6/scripts/totalizer-http-test.js
        env:
          ADDER_ADDR: ${{ secrets.ADDER_ADDR }}
          TOTALIZER_URL: ${{ secrets.TOTALIZER_URL }}

      - name: Upload results
        uses: actions/upload-artifact@v4
        with:
          name: k6-results
          path: results/
```

## Testing Checklist

- [ ] k6 installed and working
- [ ] Services accessible from test machine
- [ ] gRPC test passes with expected latency
- [ ] HTTP test passes with expected latency
- [ ] Full flow test completes successfully
- [ ] Grafana shows load test traffic
- [ ] HPA scales under load
- [ ] No errors under normal load
- [ ] Graceful degradation under extreme load
