#!/usr/bin/env python3
"""
Web scraping module that retrieves data from the database and performs scraping operations.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import requests
from bs4 import BeautifulSoup
from typing import List, Dict
import time
from urllib.parse import urljoin
from storage.database import DatabaseManager
from storage.urls_collection import URLsCollection


class WebScraper:
    """Web scraper that works with stored search results."""
    
    def __init__(self, db_connection_string: str = None):
        """
        Initialize the web scraper.
        
        Args:
            db_connection_string: MongoDB connection string
        """
        self.db_manager = DatabaseManager(db_connection_string)
        self.urls_collection = URLsCollection(self.db_manager)
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
        })
    
    def get_pending_urls_from_db(self, query_id: str = None, limit: int = 100) -> List[Dict]:
        """
        Get pending URLs from the database.
        
        Args:
            query_id: Optional query ID filter
            limit: Maximum number of results
            
        Returns:
            List of pending URL documents
        """
        return self.urls_collection.get_pending_urls(query_id, limit)
    
    def scrape_url(self, url: str, timeout: int = 10) -> Dict:
        """
        Scrape content from a single URL.
        
        Args:
            url: URL to scrape
            timeout: Request timeout in seconds
            
        Returns:
            Dictionary containing scraped data
        """
        try:
            response = self.session.get(url, timeout=timeout)
            response.raise_for_status()
            
            soup = BeautifulSoup(response.content, 'html.parser')
                    
            # Remove script and style elements
            for script in soup(["script", "style"]):
                script.decompose()
            
            # Extract basic content
            title = soup.find('title')
            title_text = title.get_text().strip() if title else ""
            
            # Extract meta description
            meta_desc = soup.find('meta', attrs={'name': 'description'})
            description = meta_desc.get('content', '').strip() if meta_desc else ""
            
            # Extract main text content (optimized)
            text_content = soup.get_text()
            # Simple whitespace normalization - much faster than nested comprehensions
            text = ' '.join(text_content.split())
            
            # Extract links
            links = []
            for link in soup.find_all('a', href=True):
                absolute_url = urljoin(url, link['href'])
                link_text = link.get_text().strip()
                if link_text and absolute_url.startswith(('http://', 'https://')):
                    links.append({
                        'url': absolute_url,
                        'text': link_text
                    })
            
            return {
                'url': url,
                'status': 'success',
                'title': title_text,
                'description': description,
                'content': text[:5000],  # Limit content length
                'links': links[:20],     # Limit number of links
                'scraped_at': time.time()
            }
            
        except requests.exceptions.RequestException as e:
            return {
                'url': url,
                'status': 'error',
                'error': str(e),
                'scraped_at': time.time()
            }
    
    def scrape_pending_urls(self, query_id: str = None, max_urls: int = 10) -> List[Dict]:
        """
        Scrape pending URLs from the database.
        
        Args:
            query_id: Optional query ID filter
            max_urls: Maximum number of URLs to scrape
            
        Returns:
            List of scraped data dictionaries with URL document IDs
        """
        # Use generator for memory efficiency
        return list(self.scrape_urls_stream(query_id, max_urls))
    
    def scrape_urls_stream(self, query_id: str = None, max_urls: int = 10):
        """
        Stream scrape pending URLs from database (generator for memory efficiency).
        
        Args:
            query_id: Optional query ID filter
            max_urls: Maximum number of URLs to scrape
            
        Yields:
            Scraped data dictionaries one at a time
        """
        # Process URLs in batches to manage memory usage
        batch_size = min(50, max_urls)
        processed_count = 0
        
        while processed_count < max_urls:
            # Get next batch of pending URLs
            batch_limit = min(batch_size, max_urls - processed_count)
            pending_urls = self.get_pending_urls_from_db(query_id, batch_limit)
            
            if not pending_urls:
                break
            
            for i, url_doc in enumerate(pending_urls):
                if processed_count >= max_urls:
                    break
                    
                print(f"Scraping {processed_count + 1}/{max_urls}: {url_doc['url']}")
                
                scraped = self.scrape_url(url_doc['url'])
                scraped.update({
                    'url_id': url_doc['_id'],
                    'query_id': url_doc['query_id'],
                    'original_title': url_doc['title'],
                    'original_snippet': url_doc['snippet']
                })
                
                # Update the URL document in the database
                if scraped['status'] == 'success':
                    self.urls_collection.update_url_content(
                        url_doc['_id'], 
                        scraped['content'], 
                        'processed'
                    )
                else:
                    self.urls_collection.mark_url_failed(
                        url_doc['_id'], 
                        scraped.get('error', 'Unknown error')
                    )
                
                processed_count += 1
                
                # Be respectful with delays
                time.sleep(1)
                
                yield scraped
            
            # Break if we got fewer URLs than requested (no more pending)
            if len(pending_urls) < batch_limit:
                break
    
    def get_scraped_urls_by_query(self, query_id: str, status: str = "processed") -> List[Dict]:
        """
        Get scraped URLs for a specific query.
        
        Args:
            query_id: Query document ID
            status: Status filter (processed, failed)
            
        Returns:
            List of URL documents
        """
        return self.urls_collection.get_urls_by_query(query_id, status)
    
    def close(self):
        """Close database connection and session."""
        self.urls_collection.close()
        self.session.close()


def main():
    """Example usage of the web scraper."""
    scraper = WebScraper()
    
    try:
        # Get pending URLs from database
        print("Fetching pending URLs from database...")
        pending_urls = scraper.get_pending_urls_from_db(limit=50)
        print(f"Found {len(pending_urls)} pending URLs")
        
        if pending_urls:
            # Scrape the pending URLs using streaming for memory efficiency
            print("\nStarting web scraping with streaming...")
            scraped_count = 0
            successful_scrapes = 0
            
            # Process URLs using generator for memory efficiency
            for scraped in scraper.scrape_urls_stream(max_urls=50):
                scraped_count += 1
                if scraped['status'] == 'success':
                    successful_scrapes += 1
                # Process results one at a time without keeping all in memory
            
            if scraped_count > 0:
                print(f"\nSuccessfully scraped {successful_scrapes}/{scraped_count} URLs")
                print("URLs have been updated in the database")
            else:
                print("No data was scraped")
        else:
            print("No pending URLs found in database")
            
    finally:
        scraper.close()


if __name__ == "__main__":
    main()