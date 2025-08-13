#!/usr/bin/env python3
"""
Web crawling module that retrieves data from the database and performs deep crawling operations.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import requests
from bs4 import BeautifulSoup
from typing import List, Dict, Set
import time
from urllib.parse import urljoin, urlparse, urlunparse
from urllib.robotparser import RobotFileParser
from storage.database import DatabaseManager
from storage.urls_collection import URLsCollection
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed


class WebCrawler:
    """Web crawler that performs deep crawling from stored search results."""
    
    def __init__(self, db_connection_string: str = None, max_depth: int = 2, max_workers: int = 5):
        """
        Initialize the web crawler.
        
        Args:
            db_connection_string: MongoDB connection string
            max_depth: Maximum crawl depth (0 = only seed URLs)
            max_workers: Maximum number of concurrent threads
        """
        self.db_manager = DatabaseManager(db_connection_string)
        self.urls_collection = URLsCollection(self.db_manager)
        self.max_depth = max_depth
        self.max_workers = max_workers
        
        # Session for HTTP requests
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
        })
        
        # Crawling state
        self.visited_urls: Set[str] = set()
        self.robots_cache: Dict[str, RobotFileParser] = {}
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
        skip_extensions = {'.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx', 
                          '.zip', '.tar', '.gz', '.rar', '.7z', '.mp3', '.mp4', 
                          '.avi', '.mov', '.wmv', '.jpg', '.jpeg', '.png', '.gif', '.bmp'}
        
        if any(parsed.path.lower().endswith(ext) for ext in skip_extensions):
            return False
        
        # Skip common spam patterns
        spam_patterns = ['login', 'register', 'cart', 'checkout', 'admin', 'wp-admin']
        if any(pattern in parsed.path.lower() for pattern in spam_patterns):
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
                except:
                    # If robots.txt can't be read, assume crawling is allowed
                    self.robots_cache[base_url] = None
            
            robots = self.robots_cache[base_url]
            if robots is None:
                return True
            
            return robots.can_fetch(self.session.headers['User-Agent'], url)
        except:
            return True
    
    def get_search_results_from_db(self, query_id: str = None, limit: int = 100) -> List[Dict]:
        """
        Get search results from the database.
        
        Args:
            query_id: Optional query ID filter
            limit: Maximum number of results
            
        Returns:
            List of URL documents with search data
        """
        if query_id:
            return self.urls_collection.get_urls_by_query(query_id, limit=limit)
        else:
            return self.urls_collection.get_pending_urls(limit=limit)
    
    def crawl_url(self, url: str, depth: int = 0, search_query: str = "", timeout: int = 10) -> Dict:
        """
        Crawl content from a single URL.
        
        Args:
            url: URL to crawl
            depth: Current crawl depth
            search_query: Original search query
            timeout: Request timeout in seconds
            
        Returns:
            Dictionary containing crawled data
        """
        try:
            # Normalize and validate URL
            normalized_url = self._normalize_url(url)
            
            if not self._is_valid_url(normalized_url):
                return {
                    'url': url,
                    'normalized_url': normalized_url,
                    'status': 'skipped',
                    'reason': 'Invalid URL',
                    'depth': depth,
                    'search_query': search_query,
                    'crawled_at': time.time()
                }
            
            # Check if already visited
            with self.lock:
                if normalized_url in self.visited_urls:
                    return {
                        'url': url,
                        'normalized_url': normalized_url,
                        'status': 'skipped',
                        'reason': 'Already visited',
                        'depth': depth,
                        'search_query': search_query,
                        'crawled_at': time.time()
                    }
                self.visited_urls.add(normalized_url)
            
            # Check robots.txt
            if not self._check_robots_txt(normalized_url):
                return {
                    'url': url,
                    'normalized_url': normalized_url,
                    'status': 'blocked',
                    'reason': 'Blocked by robots.txt',
                    'depth': depth,
                    'search_query': search_query,
                    'crawled_at': time.time()
                }
            
            # Perform HTTP request
            response = self.session.get(normalized_url, timeout=timeout)
            response.raise_for_status()
            
            # Only process HTML content
            content_type = response.headers.get('content-type', '').lower()
            if 'text/html' not in content_type:
                return {
                    'url': url,
                    'normalized_url': normalized_url,
                    'status': 'skipped',
                    'reason': f'Non-HTML content: {content_type}',
                    'depth': depth,
                    'search_query': search_query,
                    'crawled_at': time.time()
                }
            
            soup = BeautifulSoup(response.content, 'html.parser')
            
            # Remove script and style elements
            for script in soup(["script", "style", "nav", "footer", "aside"]):
                script.decompose()
            
            # Extract content
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
            
            # Extract internal links for further crawling
            internal_links = []
            parsed_base = urlparse(normalized_url)
            base_domain = parsed_base.netloc
            
            for link in soup.find_all('a', href=True):
                href = link['href'].strip()
                if not href or href.startswith('#'):
                    continue
                
                absolute_url = urljoin(normalized_url, href)
                parsed_link = urlparse(absolute_url)
                
                # Only collect internal links from the same domain
                if parsed_link.netloc == base_domain:
                    link_text = link.get_text().strip()
                    if self._is_valid_url(absolute_url):
                        internal_links.append({
                            'url': absolute_url,
                            'text': link_text[:100],  # Limit text length
                            'context': text[:200] if text else ""
                        })
            
            # Extract external links for reference
            external_links = []
            for link in soup.find_all('a', href=True)[:10]:  # Limit external links
                href = link['href'].strip()
                if href and href.startswith(('http://', 'https://')):
                    absolute_url = urljoin(normalized_url, href)
                    parsed_link = urlparse(absolute_url)
                    
                    if parsed_link.netloc != base_domain:
                        link_text = link.get_text().strip()
                        external_links.append({
                            'url': absolute_url,
                            'text': link_text[:100],
                            'domain': parsed_link.netloc
                        })
            
            return {
                'url': url,
                'normalized_url': normalized_url,
                'status': 'success',
                'title': title_text,
                'description': description,
                'keywords': keywords,
                'content': text[:10000],  # Limit content length
                'word_count': len(text.split()),
                'internal_links': internal_links[:50],  # Limit internal links
                'external_links': external_links,
                'depth': depth,
                'search_query': search_query,
                'response_time': response.elapsed.total_seconds(),
                'content_length': len(response.content),
                'crawled_at': time.time()
            }
            
        except requests.exceptions.RequestException as e:
            return {
                'url': url,
                'normalized_url': self._normalize_url(url),
                'status': 'error',
                'error': str(e),
                'depth': depth,
                'search_query': search_query,
                'crawled_at': time.time()
            }
        except Exception as e:
            return {
                'url': url,
                'normalized_url': self._normalize_url(url),
                'status': 'error',
                'error': f"Unexpected error: {str(e)}",
                'depth': depth,
                'search_query': search_query,
                'crawled_at': time.time()
            }
    
    def crawl_from_search_results(self, query: str = None, max_urls: int = 10, 
                                max_pages_per_domain: int = 5) -> List[Dict]:
        """
        Perform deep crawling from stored search results.
        
        Args:
            query: Optional query filter for search results
            max_urls: Maximum seed URLs to start crawling from
            max_pages_per_domain: Maximum pages to crawl per domain
            
        Returns:
            List of crawled data dictionaries
        """
        print(f"Starting crawl with max_depth={self.max_depth}, max_workers={self.max_workers}")
        
        # Get search results from database
        search_results = self.get_search_results_from_db(query)
        
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
        
        for current_depth in range(self.max_depth + 1):
            print(f"Crawling depth {current_depth}...")
            
            # Filter URLs for current depth
            current_urls = [(info, depth) for info, depth in urls_to_crawl if depth == current_depth]
            
            if not current_urls:
                break
            
            # Crawl URLs concurrently
            crawled_batch = []
            
            with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
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
                        if (result['status'] == 'success' and 
                            current_depth < self.max_depth and 
                            result.get('internal_links')):
                            
                            for link in result['internal_links'][:10]:  # Limit links per page
                                urls_to_crawl.append(({
                                    'url': link['url'],
                                    'search_query': result['search_query']
                                }, depth + 1))
                        
                        # Store internal links for future crawling
                        if result.get('internal_links'):
                            self.urls_collection.add_urls_from_crawl(
                                result['search_query'], 
                                result['internal_links'], 
                                depth + 1
                            )
                        
                        print(f"Crawled: {result['url']} (Status: {result['status']})")
                        
                    except Exception as e:
                        print(f"Error crawling {url_info['url']}: {e}")
            
            all_crawled_data.extend(crawled_batch)
            
            # Add delay between depth levels
            if current_depth < self.max_depth:
                time.sleep(2)
        
        print(f"Crawling completed. Total URLs processed: {len(all_crawled_data)}")
        return all_crawled_data
    
    
    def get_crawling_stats(self, crawled_data: List[Dict]) -> Dict:
        """
        Generate crawling statistics.
        
        Args:
            crawled_data: List of crawled data
            
        Returns:
            Statistics dictionary
        """
        if not crawled_data:
            return {}
        
        stats = {
            'total_urls': len(crawled_data),
            'successful_crawls': sum(1 for item in crawled_data if item['status'] == 'success'),
            'failed_crawls': sum(1 for item in crawled_data if item['status'] == 'error'),
            'skipped_crawls': sum(1 for item in crawled_data if item['status'] in ['skipped', 'blocked']),
            'domains': len(set(urlparse(item['url']).netloc for item in crawled_data)),
            'avg_content_length': 0,
            'avg_response_time': 0,
            'depth_distribution': {}
        }
        
        successful_crawls = [item for item in crawled_data if item['status'] == 'success']
        if successful_crawls:
            stats['avg_content_length'] = sum(item.get('content_length', 0) for item in successful_crawls) / len(successful_crawls)
            stats['avg_response_time'] = sum(item.get('response_time', 0) for item in successful_crawls) / len(successful_crawls)
        
        # Depth distribution
        for item in crawled_data:
            depth = item.get('depth', 0)
            stats['depth_distribution'][depth] = stats['depth_distribution'].get(depth, 0) + 1
        
        return stats
    
    def close(self):
        """Close database connections and session."""
        self.db_manager.close()
        self.session.close()


def main():
    """Example usage of the web crawler."""
    crawler = WebCrawler(max_depth=2, max_workers=3)
    
    try:
        print("Starting web crawling from search results...")
        
        # Perform crawling
        crawled_data = crawler.crawl_from_search_results(max_urls=50, max_pages_per_domain=3)
        
        if crawled_data:            
            # Print statistics
            stats = crawler.get_crawling_stats(crawled_data)
            print("\nCrawling Statistics:")
            print(f"- Total URLs processed: {stats['total_urls']}")
            print(f"- Successful crawls: {stats['successful_crawls']}")
            print(f"- Failed crawls: {stats['failed_crawls']}")
            print(f"- Skipped crawls: {stats['skipped_crawls']}")
            print(f"- Domains crawled: {stats['domains']}")
            print(f"- Average content length: {stats['avg_content_length']:.0f} bytes")
            print(f"- Average response time: {stats['avg_response_time']:.2f} seconds")
            print(f"- Depth distribution: {stats['depth_distribution']}")
        else:
            print("No data was crawled")
            
    except Exception as e:
        print(f"Error during crawling: {e}")
    finally:
        crawler.close()


if __name__ == "__main__":
    main()