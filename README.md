# Axora
**Data Intelligence Platform for Web Search, Crawling, and Sentiment Analysis**

Axora is a comprehensive data intelligence platform that performs web searches, scrapes content, crawls websites, and analyzes sentiment from collected data. It uses MongoDB for data persistence and provides an interactive CLI interface for easy operation.

## Architecture Overview

Axora follows a modular architecture with six core modules:

1. **CLI Module** - Interactive command-line interface
2. **Search Module** - Web search functionality using DuckDuckGo Lite
3. **Scraper Module** - Content extraction from discovered URLs
4. **Crawler Module** - Deep web crawling with multi-level traversal
5. **Sentiment Module** - Text sentiment analysis using VADER
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

### 5. Sentiment Module (`sentiment/`)

**Files:** `sentiment.py`, `vader.py`

The sentiment module provides text sentiment analysis capabilities using the VADER (Valence Aware Dictionary and sEntiment Reasoner) algorithm. It analyzes scraped content and provides sentiment scores and classifications.

**Key Features:**
- VADER sentiment analysis implementation
- Multi-field sentiment analysis (title, description, content)
- Combined sentiment scoring
- Database integration for result storage
- Modular design for multiple sentiment models

**Main Classes:**
- `SentimentAnalyzer`: Main sentiment analysis controller
- `VaderSentimentAnalyzer`: VADER-specific implementation

**Analysis Output:**
- Compound score (-1 to 1)
- Individual scores (positive, negative, neutral ratios)
- Sentiment classification (positive/negative/neutral)
- Field-specific analysis (title, description, content)

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

## Data Flow Architecture

```
1. User Query → CLI Module
2. CLI → Search Module → DuckDuckGo API
3. Search Results → Storage (URLs Collection)
4. Background Processing:
   a. Scraper Module → Content Extraction
   b. Crawler Module → Link Discovery
   c. Sentiment Module → Text Analysis
5. Results Storage → MongoDB
6. CLI → Status Reporting → User
```

## Technical Stack

- **Language**: Python 3.11+
- **Database**: MongoDB 8.0
- **Web Requests**: requests library
- **HTML Parsing**: BeautifulSoup4
- **Sentiment Analysis**: VADER Sentiment
- **Containerization**: Docker & Docker Compose
- **Database UI**: Mongo Express

## Dependencies

```
pymongo==4.6.1          # MongoDB Python driver
flask==3.0.0             # Web framework (for future API)
requests==2.31.0         # HTTP requests
beautifulsoup4==4.12.2   # HTML parsing
vaderSentiment==3.3.2    # Sentiment analysis
```

## Deployment

The application supports containerized deployment using Docker:

- **Application Container**: Python 3.11 slim base image
- **Database**: MongoDB 8.0 with persistent volumes
- **Management**: Mongo Express for database administration
- **Networking**: Bridge network for service communication

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
├── sentiment/
│   ├── sentiment.py              # Sentiment analysis controller
│   └── vader.py                  # VADER sentiment analyzer
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