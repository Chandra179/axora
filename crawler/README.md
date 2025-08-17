# Crawler

## Kafka topic names
### urls.seed : Events for new seed URLs you want to push into the pipeline.
```json
{
  "event_id": "uuid",
  "event_type": "urls.seed",
  "occurred_at": "2025-08-17T06:00:00Z",
  "source": "orchestrator",
  "payload": {
    "url_id": "uuid",
    "url": "https://example.com/page",
  }
}
```


### urls.crawl.success : Events emitted when a URL has been successfully crawled (raw HTML + metadata).
```json
{
  "event_id": "uuid",
  "event_type": "urls.crawl.success",
  "source": "crawler",
  "payload": {
    "url": "https://example.com/page",
    "content_type": "text/html",
    "content_length": 58731,
    "fetched_at": "2025-08-17T06:01:10Z"
  },
  "metadata": {
    "fetch_duration_ms": 742,
    "user_agent": "MyCrawler/1.0"
  }
}
```

### urls.crawl.failed : Errors like unreachable host, timeout, robots disallow.
```json
{
  "event_id": "uuid",
  "event_type": "urls.crawl.failed",
  "source": "crawler",
  "payload": {
    "reason": "timeout",
    "retryable": true,
    "next_attempt_at": "2025-08-17T06:10:00Z"
  },
  "metadata": {
    "attempt": 2,
    "producer_hostname": "crawler-01"
  }
}
```

