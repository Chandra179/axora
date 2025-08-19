"""
Entrypoint: init mongodb collection, run the worker
"""

import asyncio
import logging
import yaml
import os
from dotenv import load_dotenv
from crawler.storage import MongoStorage
from crawler.robots import RobotsChecker
from crawler.fetcher import HTTPFetcher
from crawler.worker import Crawler


async def main():
    """Initialize dependencies and start the crawler"""
    logging.basicConfig(level=logging.INFO)
    logger = logging.getLogger(__name__)
    
    # Load environment variables from .env file
    load_dotenv()
    
    # Load configuration from YAML
    with open('config.yaml', 'r') as f:
        config = yaml.safe_load(f)
    
    # Add MongoDB connection details from environment variables
    config['mongodb']['uri'] = os.getenv('MONGO_URL')
    config['mongodb']['database'] = os.getenv('MONGO_DATABASE')
    
    # Initialize storage with config
    storage = MongoStorage(config=config)
    if not storage.connect():
        logger.error("Failed to connect to MongoDB")
        return
    
    # Initialize robots checker and fetcher
    robots_checker = RobotsChecker()
    fetcher = HTTPFetcher()
    
    # Create crawler with dependency injection
    crawler = Crawler(storage=storage, fetcher=fetcher, robots_checker=robots_checker)
    
    # Add some example seed URLs
    storage.add_seed_url("https://httpbin.org/")
    storage.add_seed_url("https://example.com/")
    
    # Start the crawler
    await crawler.start()


if __name__ == "__main__":
    asyncio.run(main())