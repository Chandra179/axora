# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Axora is a comprehensive data intelligence platform for web search, crawling, and sentiment analysis. It's built with Python 3.11+ and uses MongoDB for data persistence, designed as a modular system with six core components:

1. **CLI Module** (`cli/`) - Interactive command-line interface
2. **Search Module** (`search/`) - DuckDuckGo Lite API integration for web searching  
3. **Scraper Module** (`scrapper/`) - Content extraction from discovered URLs
4. **Crawler Module** (`crawler/`) - Deep web crawling with multi-level traversal
5. **Sentiment Module** (`sentiment/`) - VADER-based sentiment analysis
6. **Storage Module** (`storage/`) - MongoDB data persistence layer

## Development Environment Setup

### Local Development
```bash
# Install dependencies
pip install -r requirements.txt

# Set up environment variables (copy from .env.example)
cp .env.example .env

# Run the CLI interface
python cli/cli.py
```

### Docker Development  
```bash
# Start all services (MongoDB, Mongo Express, and app)
docker-compose up -d

# View logs
docker-compose logs app

# Stop services
docker-compose down
```

## Common Commands

### Running the Application
- **Interactive CLI**: `python cli/cli.py`
- **Docker container**: `docker-compose up -d app`

### Database Management
- **MongoDB**: Accessed via Docker service on port 27017
- **Mongo Express UI**: http://localhost:8081 (when running via docker-compose)
- **Connection string**: Uses `MONGODB_URL` environment variable

### CLI Commands (within the interactive interface)
- `search <query>` - Perform web search and start processing pipeline
- `status <query_id>` - Check processing status for a query
- `list` - Show recent queries and their status
- `set max_urls <number>` - Configure URL limits per search
- `help` - Display available commands
- `quit` - Exit the CLI

## Architecture & Data Flow

The system follows a pipeline architecture:
1. User query → CLI Module → Search Module → DuckDuckGo API
2. Search results stored in URLs collection
3. Background processing: Scraper → Crawler → Sentiment Analysis
4. All results persisted in MongoDB with status tracking

### Key Design Patterns
- **Modular separation**: Each module handles a specific concern
- **Database-driven pipeline**: Uses MongoDB collections for state management
- **Status tracking**: Queries have status states (pending, processing, completed)
- **Concurrent processing**: Crawler uses thread pools for parallel URL processing

## Database Schema

### Collections
- **queries**: Search queries and metadata with status tracking
- **urls**: URLs, scraped content, and processing status  
- **sentiment_analysis**: Sentiment analysis results linked to URLs

### Environment Variables
- `MONGODB_URL`: Full MongoDB connection string
- `MONGODB_PORT`: MongoDB port (default: 27017)
- `MONGODB_USERNAME`/`MONGODB_PASSWORD`: Database credentials
- `APP_PORT`: Application port for containerized deployment

## Module-Specific Notes

### Search Module (`search/search.py`)
- Implements DuckDuckGo Lite search with pagination (up to 20 pages)
- Handles both GET (first page) and POST (subsequent pages) requests
- Includes rate limiting and duplicate URL filtering

### Crawler Module (`crawler/crawler.py`)
- Multi-depth crawling with configurable depth levels (default: 2)
- Robots.txt compliance checking and caching
- Domain-based crawling limits with concurrent workers (default: 5 threads)

### Storage Module (`storage/`)
- `database.py`: Core MongoDB connection management
- `queries_collection.py`: Query CRUD operations
- `urls_collection.py`: URL and content data operations
- Uses environment variables for connection configuration

### Sentiment Analysis (`sentiment/`)
- VADER sentiment analyzer for compound scoring (-1 to 1)  
- Analyzes title, description, and main content separately
- Results stored with field-specific sentiment scores

## Testing and Quality

The codebase does not currently include automated tests. When adding tests:
- Use pytest for Python testing
- Test each module in isolation with mocked dependencies
- Include integration tests for the full pipeline
- Test Docker deployment with test databases