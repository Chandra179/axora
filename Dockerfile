FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install Python dependencies
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY . .

# Create non-root user
RUN useradd --create-home --shell /bin/bash axora
RUN chown -R axora:axora /app
USER axora

# Expose port (if needed for health checks or metrics)
EXPOSE 8000

# Run the crawler
CMD ["python", "main.py"]