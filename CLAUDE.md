# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Build and Run
- `make run` - Build and run the full stack with Docker Compose (includes services rebuild)
- `make go` - Start services without rebuild
- `make ins` - Update and vendor Go dependencies (`go mod tidy && go mod vendor`)

### Go Commands
- `go build -mod=vendor -o main ./cmd` - Build the main application
- `go mod tidy` - Update dependencies
- `go mod vendor` - Vendor dependencies

### Docker Operations
- `docker compose up -d --build` - Build and start all services
- `docker compose up -d` - Start services without rebuild
- `docker compose down` - Stop all services

## Architecture Overview

Axora is an intelligent web crawler that uses semantic similarity to filter relevant content. The system consists of several components working together:

### Core Components
- **Main Application** (`cmd/main.go`): Entry point that orchestrates all components
- **Configuration** (`config/`): Environment-based configuration management for MongoDB, SerpAPI, and TEI model settings
- **Search Engine** (`search/`): SerpAPI integration for initial URL discovery
- **Crawler** (`crawler/`): Web crawling with semantic relevance filtering using Colly
- **Content Extractor** (`crawler/extractor.go`): Clean text extraction from HTML using go-readability
- **Relevance Filter** (`crawler/relevance.go`): Semantic similarity-based URL filtering using Sentence-BERT embeddings
- **TEI Client** (`client/`): Text Embeddings Inference client for AI model communication
- **Storage** (`storage/`): MongoDB integration for data persistence

### Service Architecture (Docker Compose)
- **axora-tei-model**: HuggingFace Text Embeddings Inference server running all-MiniLM-L6-v2 model on port 8000
- **axora-mongodb**: MongoDB database with admin interface
- **axora-mongo-express**: Web-based MongoDB admin interface
- **axora-crawler**: Main application container

### Key Technologies
- **Go 1.25.0** with vendored dependencies
- **Sentence-BERT (all-MiniLM-L6-v2)**: 384-dimensional embeddings for semantic similarity
- **MongoDB**: Document storage for crawled content and metadata
- **Colly v2**: Web crawling framework with rate limiting and async support
- **Docker Compose**: Service orchestration with health checks

### Environment Configuration
The application requires several environment variables for MongoDB connection, SerpAPI key, and service ports. All configuration is handled through the `config/` package which validates required environment variables on startup.

### Semantic Filtering
The relevance filter uses embeddings to compare content similarity against search queries with a configurable threshold (default 0.7). This allows the crawler to focus on semantically relevant content rather than just keyword matches.