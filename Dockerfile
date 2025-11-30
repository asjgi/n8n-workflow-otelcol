FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o otel-automation ./cmd/main.go

# Final stage
FROM alpine:3.18

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/otel-automation .

# Create non-root user
RUN addgroup -g 1001 -S automation && \
    adduser -u 1001 -S automation -G automation

USER automation

EXPOSE 8080

CMD ["./otel-automation"]