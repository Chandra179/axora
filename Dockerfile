# Build stage
FROM golang:1.25.0-alpine AS builder

WORKDIR /app

# Copy go mod, sum files and vendor directory first (better layer caching)
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Build dependencies first (separate layer for better caching)
RUN go list -mod=vendor ./... > /dev/null

# Copy source code (this layer changes most often)
COPY . .

# Build the application using vendored dependencies with build cache
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o main ./cmd

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Expose port (adjust as needed)
EXPOSE 8080

# Command to run the executable
CMD ["./main"]