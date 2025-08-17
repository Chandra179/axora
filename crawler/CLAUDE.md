# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a distributed web crawler that uses Kafka for event-driven architecture. The crawler consumes URL seed events, fetches web content, and publishes crawl results (success/failure) back to Kafka topics.

## Architecture

The crawler follows an event-driven pipeline architecture:

- **main.py**: Application entrypoint that initializes logging, metrics, Kafka clients, and starts the worker loop
- **consumer.py**: Consumes `urls.seed` events from Kafka containing URLs to crawl
- **worker.py**: Core pipeline processor that handles: URL normalization → claim → robots.txt check → host politeness → fetch → classify → save → publish
- **fetcher.py**: HTTP client abstraction for actual web fetching (handles redirects, timeouts, retries)
- **storage.py**: MongoDB wrapper that manages crawl results and URL deduplication using fingerprints
- **robots.py**: Handles robots.txt compliance checking
- **producer.py**: Publishes crawl results to `urls.crawl.success` and `urls.crawl.failed` Kafka topics
- **config.py**: Configuration loader for config.yaml and environment variables

## Kafka Event Schema

The system uses three main Kafka topics:
- `urls.seed`: Input events with URLs to crawl
- `urls.crawl.success`: Published when crawling succeeds
- `urls.crawl.failed`: Published when crawling fails (with retry logic)

See README.md for complete event schema definitions.

## Development Environment

The project uses:
- Python (virtual environment in `venv/`)
- MongoDB for storage (configured via docker-compose.yaml)
- Kafka for event messaging
- Configuration via `src/config.yaml` and environment variables

## Infrastructure

The parent directory contains `docker-compose.yaml` which sets up MongoDB and Mongo Express for development. The crawler connects to this MongoDB instance for storing crawl results and managing URL deduplication.

## Key Design Patterns
- **URL Deduplication**: Uses fingerprinting and MongoDB indexing to prevent duplicate crawls
- **Distributed Coordination**: URL claim/release mechanism allows multiple crawler instances
- **Politeness**: Built-in host politeness delays and robots.txt compliance
- **Event-Driven**: All communication via Kafka events for loose coupling and scalability