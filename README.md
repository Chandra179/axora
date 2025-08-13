# Axora
Data intelligence

## Project Structure

```
axora/
├── cli/
│   └── cli.py              # Interactive search CLI
├── search/
│   └── search.py           # DuckDuckGo search implementation
├── scrapper/
│   └── scraping.py         # Web scraping from search results
├── crawler/
│   └── crawler.py          # Deep web crawling with multi-level depth
├── sentiment/
│   ├── sentiment.py        # Sentiment analysis
│   └── vader.py            # VADER sentiment analyzer
├── storage/
│   ├── __init__.py
│   ├── database.py         # MongoDB connection and operations
│   ├── search_collection.py      # Search results storage
│   ├── scrapped_collection.py    # Scrapped data storage
│   └── crawler_collection.py     # Crawler data storage
├── requirements.txt        # Python dependencies
└── README.md               # This file
```

## Usage

### CLI

```bash
python cli/cli.py
```

Available commands:
- Type any query to search
- `help` - Show help information
- `set max_urls <number>` - Set maximum URLs to fetch
- `quit` - Exit the CLI

### Scrapping

```bash
python scrapper/scraping.py
```

### Crawler

```bash
python crawler/crawler.py
```

## Simplified Architecture

### Data Model (2 Collections Only)

**queries collection:**
```json
{
  "_id": "ObjectId",
  "question": "climate change impact",
  "timestamp": "2024-01-15T10:30:00Z",
  "status": "completed",
  "total_urls": 25,
  "processed_urls": 23,
  "avg_sentiment": 0.2,
  "summary": "Mixed sentiment on climate change discussions..."
}
```

**urls collection:**
```json
{
  "_id": "ObjectId",
  "url": "https://example.com/article",
  "query_id": "ObjectId (ref to queries)",
  "source": "search|crawl",
  "status": "processed|pending|failed",
  "content": "scraped text content",
  "sentiment_score": 0.75,
  "sentiment_label": "positive",
  "scraped_at": "2024-01-15T10:35:00Z",
  "depth": 1
}
```

### Simplified Pipeline

1. **Search** → Find URLs → Store in `urls` (deduplicated by URL+query_id)
2. **Worker Process** → Pick pending URLs → Scrape + Analyze Sentiment + Optional Crawl
3. **Update** → Mark URL as processed → Update query summary

### Benefits

- **Single source of truth** for URLs (no duplicates)
- **Temporal queries** tracked with timestamps
- **Simple status tracking** (pending/processing/completed)
- **Easy analytics** (aggregate by query_id)
- **Scalable** (add workers without complexity)

### Implementation Changes Needed

- Merge scraper/crawler into single worker
- Add URL deduplication logic
- Implement query-based processing
- Add simple job queue (MongoDB-based)