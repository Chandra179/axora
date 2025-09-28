# Single-stage build
FROM golang:1.25.0-bookworm

WORKDIR /app

# Install system dependencies including Chrome
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    libtesseract-dev \
    libleptonica-dev \
    libpng-dev \
    ca-certificates \
    tesseract-ocr-eng \
    wget \
    gnupg \
    unzip \
    curl \
    xvfb \
    && rm -rf /var/lib/apt/lists/*

# Install Google Chrome
RUN wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | apt-key add - \
    && echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update \
    && apt-get install -y google-chrome-stable \
    && rm -rf /var/lib/apt/lists/*

# Copy Go modules and vendor
COPY go.mod go.sum ./
COPY vendor/ vendor/
RUN go list -mod=vendor ./... > /dev/null

# Copy source code and build
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o main ./cmd

# Set environment variables for headless Chrome
ENV DISPLAY=:99
ENV CHROME_BIN=/usr/bin/google-chrome
ENV CHROME_PATH=/usr/bin/google-chrome

# Make sure WORKDIR matches where binary lives
WORKDIR /app
CMD ["./main"]
EXPOSE 8080