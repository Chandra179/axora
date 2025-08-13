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
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, field
from requests.exceptions import RequestException, Timeout, ConnectionError, HTTPError


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
        self.visited_urls: Set[str] = set()
        self.robots_cache: Dict[str, Optional[RobotFileParser]] = {}
        self.lock = threading.Lock()
    
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
        
        # Extract main text content
        text_content = soup.get_text()
        lines = (line.strip() for line in text_content.splitlines())
        chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
        text = ' '.join(chunk for chunk in chunks if chunk)
        
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
        
        # Check if already visited
        with self.lock:
            if normalized_url in self.visited_urls:
                return self._create_error_result(url, normalized_url, 'skipped',
                                               depth, search_query, reason='Already visited')
            self.visited_urls.add(normalized_url)
        
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
    
    def crawl_from_search_results(self, query: str = None, max_urls: int = 10, 
                                max_pages_per_domain: int = 5) -> List[CrawlResult]:
        """
        Perform deep crawling from stored search results.
        
        Args:
            query: Optional query filter for search results
            max_urls: Maximum seed URLs to start crawling from
            max_pages_per_domain: Maximum pages to crawl per domain
            
        Returns:
            List of CrawlResult objects
        """
        print(f"Starting crawl with max_depth={self.config.max_depth}, max_workers={self.config.max_workers}")
        
        # Get search results from database
        if query:
            search_results = self.urls_collection.get_urls_by_query(query)
        else:
            search_results = self.urls_collection.get_pending_urls()
        
        # Extract seed URLs from the URL collection structure
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
        
        # Perform crawling
        all_crawled_data = []
        urls_to_crawl = [(info, 0) for info in seed_urls]  # (url_info, depth)
        
        for current_depth in range(self.config.max_depth + 1):
            print(f"Crawling depth {current_depth}...")
            
            # Filter URLs for current depth
            current_urls = [(info, depth) for info, depth in urls_to_crawl if depth == current_depth]
            
            if not current_urls:
                break
            
                # Crawl URLs concurrently
            crawled_batch = []
            
            with ThreadPoolExecutor(max_workers=self.config.max_workers) as executor:
                future_to_url = {}
                
                for url_info, depth in current_urls:
                    url = url_info['url']
                    search_query = url_info.get('search_query', '')
                    future = executor.submit(self.crawl_url, url, depth, search_query)
                    future_to_url[future] = (url_info, depth)
                
                for future in as_completed(future_to_url):
                    url_info, depth = future_to_url[future]
                    try:
                        result = future.result()
                        crawled_batch.append(result)
                        
                        # If successful and not at max depth, add internal links for next depth
                        if (result.status == 'success' and 
                            current_depth < self.config.max_depth and 
                            result.internal_links):
                            
                            for link in result.internal_links[:self.config.max_links_per_page]:
                                urls_to_crawl.append(({
                                    'url': link.url,
                                    'search_query': result.search_query
                                }, depth + 1))
                        
                        # Store internal links for future crawling
                        if result.internal_links:
                            link_dicts = [{'url': link.url, 'text': link.text, 'context': link.context} 
                                        for link in result.internal_links]
                            self.urls_collection.add_urls_from_crawl(
                                result.search_query, 
                                link_dicts, 
                                depth + 1
                            )
                        
                        print(f"Crawled: {result.url} (Status: {result.status})")
                        
                    except Exception as e:
                        print(f"Error crawling {url_info['url']}: {e}")
            
            all_crawled_data.extend(crawled_batch)
            
            # Add delay between depth levels
            if current_depth < self.config.max_depth:
                time.sleep(self.config.depth_delay)
        
        print(f"Crawling completed. Total URLs processed: {len(all_crawled_data)}")
        return all_crawled_data
    
    def get_crawling_stats(self, crawled_data: List[CrawlResult]) -> Dict:
        """
        Generate crawling statistics.
        
        Args:
            crawled_data: List of CrawlResult objects
            
        Returns:
            Statistics dictionary
        """
        if not crawled_data:
            return {}
        
        stats = {
            'total_urls': len(crawled_data),
            'successful_crawls': sum(1 for item in crawled_data if item.status == 'success'),
            'failed_crawls': sum(1 for item in crawled_data if item.status == 'error'),
            'skipped_crawls': sum(1 for item in crawled_data if item.status in ['skipped', 'blocked']),
            'domains': len(set(urlparse(item.url).netloc for item in crawled_data)),
            'avg_content_length': 0,
            'avg_response_time': 0,
            'depth_distribution': {}
        }
        
        successful_crawls = [item for item in crawled_data if item.status == 'success']
        if successful_crawls:
            stats['avg_content_length'] = sum(item.content_length for item in successful_crawls) / len(successful_crawls)
            stats['avg_response_time'] = sum(item.response_time for item in successful_crawls) / len(successful_crawls)
        
        # Depth distribution
        for item in crawled_data:
            stats['depth_distribution'][item.depth] = stats['depth_distribution'].get(item.depth, 0) + 1
        
        return stats
    
    def close(self):
        """Close database connections and session."""
        self.db_manager.close()
        self.session.close()


def main():
    """Example usage of the web crawler."""
    config = CrawlerConfig(max_depth=2, max_workers=3)
    crawler = WebCrawler(config=config)
    
    try:
        print("Starting web crawling from search results...")
        
        # Perform crawling
        crawled_data = crawler.crawl_from_search_results(max_urls=50, max_pages_per_domain=3)
        
        if crawled_data:            
            # Print statistics
            stats = crawler.get_crawling_stats(crawled_data)
            print("\nCrawling Statistics:")
            print(f"- Total URLs processed: {stats.get('total_urls', 0)}")
            print(f"- Successful crawls: {stats.get('successful_crawls', 0)}")
            print(f"- Failed crawls: {stats.get('failed_crawls', 0)}")
            print(f"- Skipped crawls: {stats.get('skipped_crawls', 0)}")
            print(f"- Domains crawled: {stats.get('domains', 0)}")
            print(f"- Average content length: {stats.get('avg_content_length', 0):.0f} bytes")
            print(f"- Average response time: {stats.get('avg_response_time', 0):.2f} seconds")
            print(f"- Depth distribution: {stats.get('depth_distribution', {})}")
        else:
            print("No data was crawled")
            
    except Exception as e:
        print(f"Error during crawling: {e}")
    finally:
        crawler.close()


if __name__ == "__main__":
    main()