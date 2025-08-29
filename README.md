# Axora

An intelligent web crawler that uses semantic similarity to filter relevant content based on search queries. Axora combines traditional web crawling with modern NLP techniques to collect and store only the most relevant web content.

## Crawling (go-colly)
- Using go-colly, with configurable(depth)
- Loop detector for visiting URLs, prevent multiple visits on the same url
- Scraping html content

## Content extractor (go-readability)
- extracting html content body

## Relevance filter
Checking if html content is relevant to given query

### Cosine similarity using all-MiniLM-L6-v2
- Prefer for 7-100 word with meaning for better result
- Strong semantic understanding (matches concepts, not just keywords)
- Multilingual support
- 384-dimensional embeddings

### Keywords filter Aho-corasick
- Filtering content that only includes any of keywords, ex: "abc, neural network, llm"
- Ex: 200 words content, check if any words/multi-word in the content includes "keywords"

## Loop detection
```
Depth measures how many "hops" away you are from the starting URL:
  Start URL (depth 0)
    └── Link found on start page (depth 1)
        └── Link found on that page (depth 2)
            └── Link found on that page (depth 3)
```
### Scenario 1: Deep but No Loops
start -> page1 -> page2 -> page3 -> page4 -> page5...
- Depth limit: Stops this at depth 3
- Loop detection: Not needed (no repeats)

### Scenario 2: Shallow but Looping
start -> pageA -> pageB -> pageA -> pageB -> pageA...
- Depth limit: Only reaches depth 1-2, won't stop the loop
- Loop detection: Catches pageA/pageB being visited multiple times

### Scenario 3: Both Issues
start -> level1 -> level2 -> level3 -> back_to_level2 -> level3 -> back_to_level2...
- Depth limit: Prevents going too deep
- Loop detection: Prevents the level2 ↔ level3 ping-pong

## Collection
```json
{
  "_id" : "68aebe024b1dc2e2de726227",
  "url": "https://www.ycombinator.com/companies/mbodi-ai/jobs/ftTsxcl-founding-research-engineer",
  "content": "Embodied AI Platform for Industrial RoboticsFounding Research ...", //html body (text) still need to be cleaned
  "crawled_at": "ISODate('2025-08-27T08:16:20.374Z')"
}
```

## Vector
- Using semantic chunking embeddings (500–1000 tokens, ~100 overlap)
- Vector search finds semantically similar chunks, but you often want filters:
  ```
  language = en
  entities.ORG = "Mbodi AI"
  date > 2025-01-01
  ```

```json
{ // REQUEST to vector db
  "id": "docid_chunk0",
  "embedding": [0.024, -0.138, 0.556],
  "metadata": {
    "doc_id": "...",
    "chunk_index": 0,
    "text": "Responsibilities include designing motion planning..."
  }
}
```