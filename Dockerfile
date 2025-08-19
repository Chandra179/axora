# Use buildkit for faster builds
# syntax=docker/dockerfile:1

FROM python:3.11-slim

WORKDIR /app

# Install system dependencies in one layer
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies with caching
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-cache-dir -r requirements.txt

# Create non-root user early
RUN useradd --create-home --shell /bin/bash axora

# Copy application code (this layer changes most frequently)
COPY --chown=axora:axora . .

# Switch to non-root user
USER axora

# Expose port
EXPOSE 8080

# Run the crawler
CMD ["python", "main.py"]