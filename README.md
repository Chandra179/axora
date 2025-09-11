# Axora
An intelligent web crawler that uses semantic similarity to filter relevant content based on search queries. Axora combines traditional web crawling with modern NLP techniques to collect and store only the most relevant web content.

## Crawl
relevance filter
```
on request (executed every http request) -> increment url visit
on html element matched: a[href], check if the link is visitable or not, check maximum url visit, then visit the url
then on scraped (final stage):
1. it extract html page using go-readability
2. then check if content is relevant to given topics/keywords
3. check if the content is boilerplate or not
4. chunking the text to fit into embed space using langchain-recusive-character-chunking
5. then we convert it to vector using model MpnetBaseV2
6. then we insert the vector into vector database (qdrant)
``` 