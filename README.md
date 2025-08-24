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

### Semantic Similarity for Link Relevancy

#### HTML Data Sources for Semantic Matching
The crawler extracts and combines the following HTML elements to form content for semantic similarity:

**Primary Sources (High Priority):**
- `<title>` tag: Page title (weight: 35%)
- `<meta name="description">` content: Page meta description (weight: 30%)
- Link anchor text: `<a href="">text</a>` content (weight: 25%)

**Secondary Sources (Medium Priority):**
- `<h1>`, `<h2>`, `<h3>` headings: Page structure context (weight: 10%)
- URL path tokens: Extract meaningful words from URL structure
- Open Graph tags: `<meta property="og:title">`, `<meta property="og:description">`

**Combined Content Format:**
```
content = title + " " + metaDescription + " " + anchorText + " " + headings
```

#### Algorithm: Sentence-BERT (all-MiniLM-L6-v2)

**Model Selection Rationale:**
- **Best for 3-100 word queries**: Optimal for typical search queries
- **Semantic understanding**: Matches "cryptocurrency forecast" with "bitcoin prediction"
- **Performance**: ~50ms per link comparison (acceptable for crawling)
- **Multilingual support**: Handles non-English content

**Implementation Approach:**
1. **Query Processing**: Convert search query to 384-dimensional embedding vector
2. **Content Processing**: Extract HTML data → combine → generate embedding vector  
3. **Similarity Calculation**: Cosine similarity between query and content vectors
4. **Relevance Decision**: Threshold-based filtering (recommended: 0.7+ for high precision)

**Advantages over TF-IDF:**
- Captures semantic relationships (synonyms, related concepts)
- Context-aware matching (handles ambiguous terms)
- Better performance on conceptual queries ("how to build AI agents")

**Technical Requirements:**
- ONNX Runtime Go package for model inference
- Pre-trained all-MiniLM-L6-v2 model (~90MB)
- Minimum 2GB RAM for efficient processing
