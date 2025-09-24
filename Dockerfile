# Single-stage build
FROM golang:1.25.0-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    libtesseract-dev \
    libleptonica-dev \
    libjpeg62-turbo \
    libpng-dev \
    ca-certificates \
    tesseract-ocr-eng \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
COPY vendor/ vendor/
RUN go list -mod=vendor ./... > /dev/null

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o main ./cmd

# Make sure WORKDIR matches where binary lives
WORKDIR /app
CMD ["./main"]
EXPOSE 8080
