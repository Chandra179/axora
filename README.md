# Axora

## Crawl

## Storage
### crawler_urls : collection mongodb
```json
{
  "url": "string",
  "parent_url": "string | null",
  "depth": "number",
  "created_at": "datetime",
  "crawled": "boolean",
  "crawled_at": "datetime | undefined",
  "metadata": "object"
}
```

### seed_urls: collection mongodb
```json
{
  "url": "string",
  "created_at": "datetime", 
  "processed": "boolean",
  "processed_at": "datetime | undefined",
  "metadata": "object"
}
```

### Crawl depth
Depth 0 (Start):
- 100 seed URLs

Depth 1:
- Scrapy crawls each of the 100 seed URLs
- Finds links on each page (let's say average 50 links per page)
- Now has ~5,000 URLs to crawl at depth 1

Depth 2:
- Scrapy crawls each of those ~5,000 URLs from depth 1
- Finds links on each of those pages (let's say average 30 links per page)
- Now has ~150,000 URLs that would be at depth 3

With DEPTH_LIMIT = 2:
- Scrapy stops here - won't crawl those 150,000 URLs
- Total pages crawled: 100 (depth 0) + ~5,000 (depth 1) + ~150,000 (depth 2) = ~155,100 pages

### Link relevancy
 Most production systems add weighted features:
  - Anchor text: 40% weight
  - URL tokens: 30% weight
  - Surrounding context: 20% weight
  - Page authority: 10% weight

  Academic Benchmarks:

  - Google's original PageRank paper: Used TF-IDF for relevance
  - Focused crawling studies: TF-IDF consistently beats other methods
  - Commercial crawlers (Scrapy, Nutch): All use TF-IDF variants
