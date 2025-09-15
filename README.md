# Axora
Data ready for LLMs by keywords

## Crawl & Scrape
1. Recursively find every link a[href] in the page
2. Constraint: only visits link that inlcude defined (params, path), https only
3. URL visits counter: count every URLs visits because every visited URLs could contain a[href] URLs that already visited 
4. Retry: retry on http response error
5. On HTTP response call `DownloadManager` if the HTTP content is downloadable (application/octet-stream, attachment, etc...)

## Data Provider
libgen

## Tor
1. Register Tor as Proxy client for colly
2. [TBD] IP rotation (every n req, 429, 503, 403)

## Download Manager
1. downlaod files using proxy ([NOT] do not open the downloaded file while still using TOR, it might exposed your IP)
2. support partial download for (pause, resume download), by content-range 
3. verify md5 hash