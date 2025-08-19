"""
Simple web crawler that fetches URLs and checks robots.txt
"""

import logging
from datetime import datetime, timezone

from .storage import MongoStorage
from .fetcher import HTTPFetcher
from .robots import RobotsChecker

logger = logging.getLogger(__name__)


class Crawler:
    """Simple crawler that fetches URLs and respects robots.txt"""
    
    def __init__(self, storage: MongoStorage = None, fetcher: HTTPFetcher = None, robots_checker: RobotsChecker = None):
        self.storage = storage
        self.fetcher = fetcher
        self.robots_checker = robots_checker
        
    async def start(self):
        """Start the simple crawler"""
        logger.info("crawler started")
        try:
            while True:
                await self._process_urls()
                
        except KeyboardInterrupt:
            logger.info("Crawler stopped by user")
        except Exception as e:
            logger.error(f"Crawler error: {e}")
        finally:
            self.storage.close()
    
    async def _process_urls(self):
        """Process URLs from the database"""
        try:
            # Get seed URLs first
            seed_urls = list(self.storage.seed_urls.find({
                'processed': False
            }).limit(10))
            
            for seed_doc in seed_urls:
                url = seed_doc['url']
                await self._crawl_url(url)
                self.storage.mark_seed_processed(url)
                
            # Then process crawler URLs
            crawler_urls = list(self.storage.crawler_urls.find({
                'crawled': False
            }).limit(10))
            
            for url_doc in crawler_urls:
                url = url_doc['url']
                await self._crawl_url(url)
                
        except Exception as e:
            logger.error(f"Error processing URLs: {e}")
    
    async def _crawl_url(self, url: str):
        """Crawl a single URL"""
        try:
            logger.info(f"Crawling: {url}")
            
            # Check robots.txt
            robots_result = await self.robots_checker.check_url_allowed(url)
            if not robots_result['allowed']:
                logger.info(f"Blocked by robots.txt: {url} - {robots_result['reason']}")
                return
            
            # Fetch the URL
            result = await self.fetcher.fetch(url)
            
            # Prepare result data for storage
            result_data = {
                'url': url,
                'depth': 0,
                'created_at': datetime.now(timezone.utc),
                'crawled': True,
                'metadata': {
                    'status_code': result.status_code,
                    'success': result.success,
                    'error': result.error,
                    'content_type': result.content_type,
                    'size': result.size,
                    'fetch_time': result.fetch_time,
                    'final_url': result.final_url,
                    'timestamp': result.timestamp
                }
            }
            
            self.storage.add_crawl_url(url, result_data)
                
        except Exception as e:
            logger.error(f"Error crawling {url}: {e}")