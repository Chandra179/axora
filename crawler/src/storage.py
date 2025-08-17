"""
Store the crawl result to mongodb, its a wrapper around mongodb.

Also handles URL deduplication:
- Maintains index on URL fingerprints for fast lookups
- Stores crawl timestamps to prevent re-crawling recent URLs
- Manages URL claim/release mechanism for distributed crawling
"""