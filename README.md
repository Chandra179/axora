# Axora (Crawl, Scrape)
conccurent web crawling system designed for concurrent, privacy-focused content collection and processing. The system leverages Tor network for anonymous crawling
![HighLevelDiagram](img/axora.png)

## Test
![HighLevelDiagram](img/test.png)

## Profiling
```
http://localhost:6060/debug/pprof/

go tool pprof http://localhost:6060/debug/pprof/allocs
(pprof) top


```