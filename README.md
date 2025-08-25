# Axora

An intelligent web crawler that uses semantic similarity to filter relevant content based on search queries. Axora combines traditional web crawling with modern NLP techniques to collect and store only the most relevant web content.

## Core Components
1. **Search Engine** (`search/`): SerpAPI integration for initial URL discovery
2. **Crawler** (`crawler/`): Web crawling engine with semantic filtering
3. **Content Extractor** (`crawler/extractor.go`): Clean text extraction from HTML
4. **Relevance Filter** (`crawler/relevance.go`): Semantic similarity-based URL filtering
5. **TEI Client** (`client/`): Text Embeddings Inference client for AI model communication
6. **Storage** (`storage/`): MongoDB integration for data persistence
7. **Configuration** (`config/`): Environment-based configuration management

## Algorithm: Sentence-BERT (all-MiniLM-L6-v2)
- Optimized for 3-100 word queries (typical search length)
- Strong semantic understanding (matches concepts, not just keywords)
- Fast inference (~50ms per comparison)
- Multilingual support
- 384-dimensional embeddings

## Project Structure

```
axora/
├── cmd/main.go              # Application entry point
├── client/                  # TEI model client
├── config/                  # Configuration management
├── crawler/                 # Web crawling logic
│   ├── crawler.go          # Main crawler worker
│   ├── extractor.go        # Content extraction
│   └── relevance.go        # Semantic filtering
├── search/                  # Search engine integrations
├── storage/                 # Database operations
├── docker-compose.yaml     # Service orchestration
├── Dockerfile             # Application container
└── Makefile              # Build commands
```

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