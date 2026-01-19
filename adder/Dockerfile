# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o /app/bin/adder ./adder/cmd/grpc

# Runtime stage
FROM alpine:3.19

# Add ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/adder .

# Expose gRPC port
EXPOSE 50051

# Run as non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Run the application
ENTRYPOINT ["/bin/sh", "-c"]
CMD ["./adder -port=${GRPC_PORT} -kafka-brokers=${KAFKA_BROKERS} -kafka-topic=${KAFKA_TOPIC}"]
