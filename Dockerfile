FROM golang:1.25.0-bookworm

WORKDIR /app

RUN wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | apt-key add - \
    && echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list \
    && apt-get update \
    && apt-get install -y google-chrome-stable \
    && rm -rf /var/lib/apt/lists/*

# Install Rust (needed for building tokenizers)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"

# Clone and build the tokenizers C library
RUN git clone https://github.com/daulet/tokenizers.git /tmp/tokenizers \
    && cd /tmp/tokenizers \
    && make build \
    && cp libtokenizers.a /usr/local/lib/ \
    && cp tokenizers.h /usr/local/include/ \
    && ldconfig \
    && rm -rf /tmp/tokenizers

# Copy Go modules and vendor
COPY go.mod go.sum ./
COPY vendor/ vendor/
RUN go list -mod=vendor ./... > /dev/null

COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 GOOS=linux go build -mod=vendor -ldflags="-w -s" -o main ./cmd

ENV DISPLAY=:99
ENV CHROME_BIN=/usr/bin/google-chrome
ENV CHROME_PATH=/usr/bin/google-chrome

WORKDIR /app
CMD ["./main"]
EXPOSE 8080