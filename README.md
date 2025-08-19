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
