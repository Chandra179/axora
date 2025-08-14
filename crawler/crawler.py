#!/usr/bin/env python3
"""
Web crawling module that retrieves data from the database and performs deep crawling operations.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import requests
from bs4 import BeautifulSoup
from typing import List, Dict, Set, Optional
import time
from urllib.parse import urljoin, urlparse, urlunparse
from urllib.robotparser import RobotFileParser
from storage.database import DatabaseManager
from storage.urls_collection import URLsCollection
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from requests.exceptions import RequestException, Timeout, ConnectionError, HTTPError
from pybloom_live import BloomFilter


@dataclass
class CrawlerConfig:
    """Configuration class for web crawler settings."""
    max_depth: int = 2
    max_workers: int = 5
    request_timeout: int = 10
    max_content_length: int = 10000
    max_internal_links: int = 50
    max_external_links: int = 10
    max_links_per_page: int = 10
    max_link_text_length: int = 100
    max_context_length: int = 200
    depth_delay: float = 2.0
    user_agent: str = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
    
    # Bloom filter settings
    bloom_capacity: int = 100000
    bloom_error_rate: float = 0.01
    batch_size: int = 50
    
    # URL filtering settings
    skip_extensions: Set[str] = field(default_factory=lambda: {
        '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx',
        '.zip', '.tar', '.gz', '.rar', '.7z', '.mp3', '.mp4',
        '.avi', '.mov', '.wmv', '.jpg', '.jpeg', '.png', '.gif', '.bmp'
    })
    
    spam_patterns: List[str] = field(default_factory=lambda: [
        'login', 'register', 'cart', 'checkout', 'admin', 'wp-admin'
    ])


@dataclass
class LinkInfo:
    """Information about a discovered link."""
    url: str
    text: str
    context: str = ""
    domain: str = ""


@dataclass
class CrawlResult:
    """Result of crawling a single URL."""
    url: str
    normalized_url: str
    status: str  # 'success', 'error', 'skipped', 'blocked'
    depth: int
    search_query: str
    crawled_at: float
    
    # Success fields
    title: str = ""
    description: str = ""
    keywords: str = ""
    content: str = ""
    word_count: int = 0
    internal_links: List[LinkInfo] = field(default_factory=list)
    external_links: List[LinkInfo] = field(default_factory=list)
    response_time: float = 0.0
    content_length: int = 0
    
    # Error/skip fields
    error: str = ""
    reason: str = ""

    def to_dict(self) -> Dict:
        """Convert to dictionary for database storage."""
        result = {
            'url': self.url,
            'normalized_url': self.normalized_url,
            'status': self.status,
            'depth': self.depth,
            'search_query': self.search_query,
            'crawled_at': self.crawled_at
        }
        
        if self.status == 'success':
            result.update({
                'title': self.title,
                'description': self.description,
                'keywords': self.keywords,
                'content': self.content,
                'word_count': self.word_count,
                'internal_links': [{
                    'url': link.url,
                    'text': link.text,
                    'context': link.context
                } for link in self.internal_links],
                'external_links': [{
                    'url': link.url,
                    'text': link.text,
                    'domain': link.domain
                } for link in self.external_links],
                'response_time': self.response_time,
                'content_length': self.content_length
            })
        elif self.error:
            result['error'] = self.error
        elif self.reason:
            result['reason'] = self.reason
            
        return result


class WebCrawler:
    """Web crawler that performs deep crawling from stored search results."""
    
    def __init__(self, db_connection_string: str = None, config: CrawlerConfig = None):
        """
        Initialize the web crawler.
        
        Args:
            db_connection_string: MongoDB connection string
            config: Crawler configuration object
        """
        self.config = config or CrawlerConfig()
        self.db_manager = DatabaseManager(db_connection_string)
        self.urls_collection = URLsCollection(self.db_manager)
        
        # Session for HTTP requests
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': self.config.user_agent
        })
        
        # Crawling state
        self.robots_cache: Dict[str, Optional[RobotFileParser]] = {}
        
        # Bloom filter for fast URL deduplication
        self.url_bloom_filter = BloomFilter(
            capacity=self.config.bloom_capacity,
            error_rate=self.config.bloom_error_rate
        )
        
        # In-memory URL cache for current crawl session
        self.seen_urls: Set[str] = set()
    
    def _normalize_url(self, url: str) -> str:
        """Normalize URL by removing fragments and query parameters."""
        parsed = urlparse(url)
        normalized = urlunparse((
            parsed.scheme,
            parsed.netloc.lower(),
            parsed.path.rstrip('/') or '/',
            '',  # params
            '',  # query
            ''   # fragment
        ))
        return normalized
    
    def _is_valid_url(self, url: str) -> bool:
        """Check if URL is valid for crawling."""
        if not url or not url.startswith(('http://', 'https://')):
            return False
        
        parsed = urlparse(url)
        
        # Skip common non-content URLs
        if any(parsed.path.lower().endswith(ext) for ext in self.config.skip_extensions):
            return False
        
        # Skip common spam patterns
        if any(pattern in parsed.path.lower() for pattern in self.config.spam_patterns):
            return False
        
        return True
    
    def _check_robots_txt(self, url: str) -> bool:
        """Check if URL is allowed by robots.txt."""
        try:
            parsed = urlparse(url)
            base_url = f"{parsed.scheme}://{parsed.netloc}"
            
            if base_url not in self.robots_cache:
                robots_url = urljoin(base_url, '/robots.txt')
                rp = RobotFileParser()
                rp.set_url(robots_url)
                try:
                    rp.read()
                    self.robots_cache[base_url] = rp
                except (RequestException, ConnectionError, Timeout, HTTPError, OSError) as e:
                    # If robots.txt can't be read, assume crawling is allowed
                    print(f"Warning: Could not read robots.txt for {base_url}: {e}")
                    self.robots_cache[base_url] = None
            
            robots = self.robots_cache[base_url]
            if robots is None:
                return True
            
            return robots.can_fetch(self.session.headers['User-Agent'], url)
        except (ValueError, AttributeError) as e:
            print(f"Warning: Error parsing URL or checking robots.txt for {url}: {e}")
            return True
    
    def _extract_content_from_soup(self, soup: BeautifulSoup) -> tuple[str, str, str, str]:
        """Extract title, description, keywords, and text content from BeautifulSoup object."""
        # Remove script and style elements
        for script in soup(["script", "style", "nav", "footer", "aside"]):
            script.decompose()
        
        # Extract title
        title = soup.find('title')
        title_text = title.get_text().strip() if title else ""
        
        # Extract meta description
        meta_desc = soup.find('meta', attrs={'name': 'description'})
        description = meta_desc.get('content', '').strip() if meta_desc else ""
        
        # Extract meta keywords
        meta_keywords = soup.find('meta', attrs={'name': 'keywords'})
        keywords = meta_keywords.get('content', '').strip() if meta_keywords else ""
        
        # Extract main text content (optimized)
        text_content = soup.get_text()
        # Simple whitespace normalization - much faster than nested comprehensions
        text = ' '.join(text_content.split())
        
        return title_text, description, keywords, text[:self.config.max_content_length]
    
    def _extract_links_from_soup(self, soup: BeautifulSoup, base_url: str, context_text: str) -> tuple[List[LinkInfo], List[LinkInfo]]:
        """Extract internal and external links from BeautifulSoup object."""
        parsed_base = urlparse(base_url)
        base_domain = parsed_base.netloc
        internal_links = []
        external_links = []
        
        for link in soup.find_all('a', href=True):
            href = link['href'].strip()
            if not href or href.startswith('#'):
                continue
            
            absolute_url = urljoin(base_url, href)
            parsed_link = urlparse(absolute_url)
            link_text = link.get_text().strip()[:self.config.max_link_text_length]
            
            if parsed_link.netloc == base_domain:
                # Internal link
                if self._is_valid_url(absolute_url) and len(internal_links) < self.config.max_internal_links:
                    internal_links.append(LinkInfo(
                        url=absolute_url,
                        text=link_text,
                        context=context_text[:self.config.max_context_length]
                    ))
            else:
                # External link
                if (href.startswith(('http://', 'https://')) and 
                    len(external_links) < self.config.max_external_links):
                    external_links.append(LinkInfo(
                        url=absolute_url,
                        text=link_text,
                        domain=parsed_link.netloc
                    ))
        
        return internal_links, external_links
    
    def _create_error_result(self, url: str, normalized_url: str, status: str, 
                            depth: int, search_query: str, error: str = "", reason: str = "") -> CrawlResult:
        """Create a CrawlResult for error/skip cases."""
        return CrawlResult(
            url=url,
            normalized_url=normalized_url,
            status=status,
            depth=depth,
            search_query=search_query,
            crawled_at=time.time(),
            error=error,
            reason=reason
        )
    
    def _batch_validate_urls(self, urls: List[str]) -> Dict[str, bool]:
        """
        Validate multiple URLs against database in batch for efficiency.
        
        Args:
            urls: List of normalized URLs to validate
            
        Returns:
            Dictionary mapping URL to validation status (True = should crawl)
        """
        if not urls:
            return {}
        
        # Check database for existing URLs in batch
        existing_status = self.urls_collection.batch_check_urls_exist(urls)
        
        # Return inverted status (True = should crawl = not existing)
        return {url: not exists for url, exists in existing_status.items()}

    def _crawl_level_batch(self, url_level: List[tuple]) -> tuple[List[CrawlResult], List[tuple]]:
        """
        Crawl a batch of URLs at the same depth level with optimized batch processing.
        
        Args:
            url_level: List of (url_info, depth) tuples for current level
            
        Returns:
            Tuple of (crawled_results, next_level_urls)
        """
        level_results = []
        next_level = []
        
        # Pre-filter URLs using batch database validation
        urls_to_validate = []
        url_mapping = {}  # Map normalized URL back to original info
        
        for url_info, depth in url_level:
            normalized_url = self._normalize_url(url_info['url'])
            if self._is_valid_url(normalized_url) and self._should_crawl_url(normalized_url):
                urls_to_validate.append(normalized_url)
                url_mapping[normalized_url] = (url_info, depth)
        
        # Batch validate against database if we have URLs that passed bloom filter
        if urls_to_validate:
            validation_results = self._batch_validate_urls(urls_to_validate)
            valid_urls = [(url_mapping[url], url) for url, should_crawl in validation_results.items() if should_crawl]
        else:
            valid_urls = []
        
        with ThreadPoolExecutor(max_workers=self.config.max_workers) as executor:
            # Submit only pre-validated URLs for parallel processing
            future_to_url = {}
            for (url_info, depth), normalized_url in valid_urls:
                url = url_info['url']
                search_query = url_info.get('search_query', '')
                future = executor.submit(self.crawl_url, url, depth, search_query)
                future_to_url[future] = (url_info, depth)
            
            # Collect results and prepare next level
            for future in as_completed(future_to_url):
                url_info, depth = future_to_url[future]
                try:
                    result = future.result()
                    level_results.append(result)
                    
                    # Collect next level URLs if successful
                    if result.status == 'success' and result.internal_links:
                        # Limit internal links to prevent explosive growth
                        limited_links = result.internal_links[:self.config.max_links_per_page]
                        
                        for link in limited_links:
                            next_level.append(({
                                'url': link.url,
                                'search_query': result.search_query
                            }, depth + 1))
                        
                        # Store internal links in database for future reference
                        link_dicts = [{'url': link.url, 'text': link.text, 'context': link.context} 
                                    for link in limited_links]
                        self.urls_collection.add_urls_from_crawl(
                            result.search_query, 
                            link_dicts, 
                            depth + 1
                        )
                    
                    print(f"Crawled: {result.url} (Status: {result.status})")
                    
                except Exception as e:
                    print(f"Error crawling {url_info['url']}: {e}")
        
        return level_results, next_level
    
    def _should_crawl_url(self, normalized_url: str) -> bool:
        """
        Fast URL deduplication using bloom filter + database batch check.
        
        Args:
            normalized_url: The normalized URL to check
            
        Returns:
            True if URL should be crawled, False if already seen/exists
        """
        # Fast O(1) in-memory check
        if normalized_url in self.seen_urls:
            return False
            
        # Fast O(1) probabilistic check with bloom filter
        if normalized_url in self.url_bloom_filter:
            return False  # Probably already crawled
        
        # Add to bloom filter and in-memory cache
        self.url_bloom_filter.add(normalized_url)
        self.seen_urls.add(normalized_url)
        return True

    def crawl_url(self, url: str, depth: int = 0, search_query: str = "") -> CrawlResult:
        """
        Crawl content from a single URL.
        
        Args:
            url: URL to crawl
            depth: Current crawl depth
            search_query: Original search query
            
        Returns:
            CrawlResult object containing crawled data
        """
        normalized_url = self._normalize_url(url)
        
        # Validate URL
        if not self._is_valid_url(normalized_url):
            return self._create_error_result(url, normalized_url, 'skipped', 
                                           depth, search_query, reason='Invalid URL')
        
        # Fast deduplication check using bloom filter
        if not self._should_crawl_url(normalized_url):
            return self._create_error_result(url, normalized_url, 'skipped',
                                           depth, search_query, reason='Already processed')
        
        # Check robots.txt
        if not self._check_robots_txt(normalized_url):
            return self._create_error_result(url, normalized_url, 'blocked',
                                           depth, search_query, reason='Blocked by robots.txt')
        
        try:
            # Perform HTTP request
            response = self.session.get(normalized_url, timeout=self.config.request_timeout)
            response.raise_for_status()
            
            # Only process HTML content
            content_type = response.headers.get('content-type', '').lower()
            if 'text/html' not in content_type:
                return self._create_error_result(url, normalized_url, 'skipped',
                                               depth, search_query, reason=f'Non-HTML content: {content_type}')
            
            soup = BeautifulSoup(response.content, 'html.parser')
            
            # Extract content using helper method
            title, description, keywords, text = self._extract_content_from_soup(soup)
            
            # Extract links using helper method
            internal_links, external_links = self._extract_links_from_soup(soup, normalized_url, text)
            
            return CrawlResult(
                url=url,
                normalized_url=normalized_url,
                status='success',
                title=title,
                description=description,
                keywords=keywords,
                content=text,
                word_count=len(text.split()),
                internal_links=internal_links,
                external_links=external_links,
                depth=depth,
                search_query=search_query,
                response_time=response.elapsed.total_seconds(),
                content_length=len(response.content),
                crawled_at=time.time()
            )
            
        except (RequestException, ConnectionError, Timeout, HTTPError) as e:
            return self._create_error_result(url, normalized_url, 'error',
                                           depth, search_query, error=str(e))
        except Exception as e:
            return self._create_error_result(url, normalized_url, 'error',
                                           depth, search_query, error=f"Unexpected error: {str(e)}")
    
    def crawl_from_search_results_stream(self, max_urls: int = 10, max_pages_per_domain: int = 5):
        """
        Stream crawling results for memory efficiency (generator version).
        
        Args:
            query: Optional query filter for search results
            max_urls: Maximum seed URLs to start crawling from
            max_pages_per_domain: Maximum pages to crawl per domain
            
        Yields:
            CrawlResult objects one at a time as they are processed
        """
        print(f"Starting streaming crawl with max_depth={self.config.max_depth}, max_workers={self.config.max_workers}")
        
        search_results = self.urls_collection.get_pending_urls()
        
        # Extract seed URLs using generator approach
        seed_urls = []
        domain_counts = {}
        
        for url_doc in search_results:
            if len(seed_urls) >= max_urls:
                break
                
            url = url_doc.get('url')
            if url:
                domain = urlparse(url).netloc
                if domain_counts.get(domain, 0) < max_pages_per_domain:
                    seed_urls.append({
                        'url': url,
                        'title': url_doc.get('title', ''),
                        'snippet': url_doc.get('snippet', ''),
                        'search_query': url_doc.get('query_id', '')
                    })
                    domain_counts[domain] = domain_counts.get(domain, 0) + 1
        
        print(f"Found {len(seed_urls)} seed URLs from {len(domain_counts)} domains")
        
        # Stream crawling results level by level
        current_level = [(info, 0) for info in seed_urls]
        total_processed = 0
        
        for current_depth in range(self.config.max_depth + 1):
            if not current_level:
                break
                
            print(f"Streaming depth {current_depth} with {len(current_level)} URLs...")
            
            # Process and yield results from current level
            next_level = []
            
            with ThreadPoolExecutor(max_workers=self.config.max_workers) as executor:
                future_to_url = {
                    executor.submit(self.crawl_url, url_info['url'], depth, url_info.get('search_query', '')): (url_info, depth)
                    for url_info, depth in current_level
                }
                
                for future in as_completed(future_to_url):
                    url_info, depth = future_to_url[future]
                    try:
                        result = future.result()
                        total_processed += 1
                        
                        # Collect next level URLs if successful and not at max depth
                        if (result.status == 'success' and 
                            current_depth < self.config.max_depth and 
                            result.internal_links):
                            
                            limited_links = result.internal_links[:self.config.max_links_per_page]
                            for link in limited_links:
                                next_level.append(({
                                    'url': link.url,
                                    'search_query': result.search_query
                                }, depth + 1))
                            
                            # Store internal links in database
                            link_dicts = [{'url': link.url, 'text': link.text, 'context': link.context} 
                                        for link in limited_links]
                            self.urls_collection.add_urls_from_crawl(
                                result.search_query, 
                                link_dicts, 
                                depth + 1
                            )
                        
                        print(f"Crawled [{total_processed}]: {result.url} (Status: {result.status})")
                        yield result
                        
                    except Exception as e:
                        print(f"Error crawling {url_info['url']}: {e}")
            
            # Prepare for next depth
            current_level = next_level if current_depth < self.config.max_depth else []
            
            # Add delay between depth levels
            if current_level and current_depth < self.config.max_depth:
                time.sleep(self.config.depth_delay)
        
        print(f"Streaming crawl completed. Total URLs processed: {total_processed}")


def main():
    """Example usage of the web crawler."""
    config = CrawlerConfig(max_depth=3, max_workers=3)
    crawler = WebCrawler(config=config)
    
    try:
        print("Starting web crawling from search results...")
        
        # Perform streaming crawling for memory efficiency
        print("Using streaming crawl for memory efficiency...")
        crawled_count = 0
        successful_count = 0
        failed_count = 0
        skipped_count = 0
        
        # Process results as they come in without storing all in memory
        for result in crawler.crawl_from_search_results_stream(max_urls=50, max_pages_per_domain=3):
            crawled_count += 1
            if result.status == 'success':
                successful_count += 1
            elif result.status == 'error':
                failed_count += 1
            else:
                skipped_count += 1
        
        if crawled_count > 0:
            print("\nStreaming Crawl Statistics:")
            print(f"- Total URLs processed: {crawled_count}")
            print(f"- Successful crawls: {successful_count}")
            print(f"- Failed crawls: {failed_count}")
            print(f"- Skipped crawls: {skipped_count}")
        else:
            print("No data was crawled")
            
    except Exception as e:
        print(f"Error during crawling: {e}")
    finally:
        crawler.close()


if __name__ == "__main__":
    main()