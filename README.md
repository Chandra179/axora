# Axora
**Data Intelligence Platform for Web Search, Crawling, and Sentiment Analysis**

Axora is a comprehensive data intelligence platform that performs web searches, scrapes content, crawls websites, and analyzes sentiment from collected data. It uses MongoDB for data persistence and provides an interactive CLI interface for easy operation.

## Architecture Overview
![Axora Architecture](/diagram/axora-arch.png)

Axora follows a modular architecture with six core modules:

1. **CLI Module** - Interactive command-line interface
2. **Search Module** - Web search functionality using DuckDuckGo Lite
3. **Scraper Module** - Content extraction from discovered URLs
4. **Crawler Module** - Deep web crawling with multi-level traversal
6. **Storage Module** - MongoDB-based data persistence layer

## Modules Documentation

### 1. CLI Module (`cli/`)

**Files:** `cli.py`

The CLI module provides an interactive command-line interface for the entire system. It acts as the primary user interface and orchestrates operations across all other modules.

**Key Features:**
- Interactive command-line interface with help system
- Real-time search query processing
- Query status tracking and monitoring
- Configurable URL limits per search
- Recent queries listing and management

**Main Classes:**
- `SearchCLI`: Main CLI controller class

**Usage Commands:**
- `search <query>` - Perform web search
- `status <query_id>` - Check processing status
- `list` - Show recent queries
- `set max_urls <num>` - Configure URL limits
- `help` - Display command help

### 2. Search Module (`search/`)

**Files:** `search.py`

The search module implements web search functionality using DuckDuckGo Lite API. It handles pagination, result parsing, and integrates with the storage system for persistent query tracking.

**Key Features:**
- DuckDuckGo Lite API integration
- Multi-page search result aggregation
- Automatic pagination handling (up to 20 pages)
- Duplicate URL filtering
- Query tracking and status management
- Respectful request rate limiting

**Main Classes:**
- `DDGLiteSearch`: Core search implementation

**Technical Details:**
- Uses GET requests for first page, POST for subsequent pages
- Implements HTML parsing for result extraction
- Handles redirected URLs and link normalization
- Stores search results in URLs collection for processing pipeline

### 3. Scraper Module (`scrapper/`)

**Files:** `scraping.py`

The scraper module extracts content from URLs discovered during the search phase. It processes pending URLs from the database and updates them with scraped content.

**Key Features:**
- BeautifulSoup-based HTML parsing
- Content extraction (title, description, main text)
- Link discovery and extraction
- Error handling and retry logic
- Database integration for status updates
- Content length limiting for storage efficiency

**Main Classes:**
- `WebScraper`: Main scraping controller

**Content Extraction:**
- Page titles and meta descriptions
- Main text content (cleaned and formatted)
- Internal and external links
- Response metadata (timing, content length)

### 4. Crawler Module (`crawler/`)

**Files:** `crawler.py`

The crawler module performs deep web crawling with multi-level traversal. It starts from search results and follows internal links to discover additional content within the same domain.

**Key Features:**
- Multi-depth crawling (configurable depth levels)
- Robots.txt compliance checking
- Concurrent processing with thread pool
- Domain-based crawling limits
- URL normalization and deduplication
- Internal link discovery and queueing

**Main Classes:**
- `WebCrawler`: Main crawling controller

**Technical Specifications:**
- Configurable maximum depth (default: 2 levels)
- Concurrent workers (default: 5 threads)
- Robots.txt caching and compliance
- URL validation and filtering
- Domain-specific page limits

### 6. Storage Module (`storage/`)

**Files:** `database.py`, `queries_collection.py`, `urls_collection.py`

The storage module provides MongoDB-based data persistence for all system data. It implements collections for queries, URLs, and sentiment results with proper indexing and relationship management.

**Key Features:**
- MongoDB connection management
- Collection-based data organization
- Automatic indexing for performance
- CRUD operations for all data types
- Query statistics and aggregation
- Data deduplication and integrity

**Main Classes:**
- `DatabaseManager`: Core database connection manager
- `QueriesCollection`: Query data operations
- `URLsCollection`: URL and content data operations

**Database Schema:**
- **queries**: Search queries and metadata
- **urls**: URLs, content, and processing status
- **sentiment_analysis**: Sentiment analysis results

## Project Structure

```
axora/
├── cli/
│   └── cli.py                    # Interactive search CLI
├── search/
│   └── search.py                 # DuckDuckGo search implementation
├── scrapper/
│   └── scraping.py               # Web scraping from search results
├── crawler/
│   └── crawler.py                # Deep web crawling with multi-level depth
├── storage/
│   ├── __init__.py
│   ├── database.py               # MongoDB connection and operations
│   ├── queries_collection.py     # Query data operations
│   └── urls_collection.py        # URL and content data operations
├── requirements.txt              # Python dependencies
├── Dockerfile                    # Container build configuration
├── docker-compose.yml            # Multi-service deployment
└── README.md                     # This documentation
```