FROM python:3.12.3-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application files
COPY . .

WORKDIR /app/src

# Expose port for potential API
EXPOSE 8001

# Default command (can be overridden in docker-compose)
CMD ["python", "main.py"]