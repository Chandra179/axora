"""
Simple MongoDB storage for URLs
"""
from pymongo import MongoClient
from typing import Dict, Any
from datetime import datetime, timezone

class MongoStorage:
    def __init__(self, config: dict = None):
        if config and 'mongodb' in config:
            self.connection_string = config['mongodb']['uri']
            self.database_name = config['mongodb']['database']
            self.collection_names = config['mongodb']['collections']
        else:
            raise ValueError("Either config dict with mongodb section or connection_string must be provided")
        
        self.client = None
        self.db = None
        self.crawler_urls = None
        self.seed_urls = None
    
    def connect(self, database_name: str = None):
        """Connect to MongoDB and initialize collections"""
        try:
            self.client = MongoClient(self.connection_string)
            db_name = database_name or self.database_name
            self.db = self.client[db_name]
            
            self.crawler_urls = self.db[self.collection_names['crawler_urls']]
            self.seed_urls = self.db[self.collection_names['seed_urls']]
            return True
        except Exception as e:
            print(f"Failed to connect to MongoDB: {e}")
            return False
    
    def _create_indexes(self):
        """Create indexes on collections for better query performance"""
        try:
            # Index on crawler_urls collection
            crawler_urls_collection = self.db[self.collection_names['crawler_urls']]
            crawler_urls_collection.create_index("url", unique=True)
            crawler_urls_collection.create_index("crawled")
            crawler_urls_collection.create_index("depth")
            crawler_urls_collection.create_index("created_at")
            
            # Index on seed_urls collection
            seed_urls_collection = self.db[self.collection_names['seed_urls']]
            seed_urls_collection.create_index("url", unique=True)
            seed_urls_collection.create_index("processed")
            seed_urls_collection.create_index("created_at")
            
        except Exception as e:
            print(f"Failed to create indexes: {e}")
    
    def close(self):
        """Close MongoDB connection"""
        if self.client:
            self.client.close()
    
    def add_seed_url(self, url: str, metadata: Dict[str, Any] = None) -> bool:
        """Add a seed URL to the seed_urls collection"""
        try:
            doc = {
                'url': url,
                'created_at': datetime.now(timezone.utc),
                'processed': False,
                'metadata': metadata
            }
            self.seed_urls.insert_one(doc)
            return True
        except Exception as e:
            print(f"Failed to add seed URL: {e}")
            return False
    
    def add_crawl_url(self, url: str, metadata: Dict[str, Any] = None) -> bool:
        """Add URL to the crawler_urls collection"""
        try:
            doc = {
                'url': url,
                'depth': metadata['depth'],
                'created_at': datetime.now(timezone.utc),
                'crawled': metadata['crawled'],
                'metadata': metadata['metadata']
            }
            
            self.crawler_urls.insert_one(doc)
            return True
        except Exception as e:
            print(f"Failed to add crawl URL: {e}")
            return False
        
    def mark_seed_processed(self, url: str) -> bool:
        """Mark seed URL as processed"""
        try:
            self.seed_urls.update_one({'url': url}, {'$set': {'processed': True, 'processed_at': datetime.now(timezone.utc)}})
            return True
        except Exception as e:
            print(f"Failed to mark seed as processed: {e}")
            return False
    
    def get_urls_by_depth(self, depth: int) -> list:
        """Get URLs at a specific depth"""
        try:
            return list(self.crawler_urls.find({'depth': depth}))
        except Exception as e:
            print(f"Failed to get URLs by depth: {e}")
            return []