#!/usr/bin/env python3
"""
URLs collection operations for the simplified architecture.
"""

from typing import List, Dict, Optional
from datetime import datetime, timezone
from bson import ObjectId
from .database import DatabaseManager


class URLsCollection:
    """Handler for urls collection operations."""
    
    def __init__(self, db_manager: DatabaseManager = None):
        """
        Initialize URLs collection handler.
        
        Args:
            db_manager: DatabaseManager instance. If None, creates a new one.
        """
        self.db_manager = db_manager or DatabaseManager()
        self.collection = self.db_manager.get_collection("urls")
        
        # Create indexes for better performance
        self._ensure_indexes()
    
    def _ensure_indexes(self):
        """Create indexes for better performance."""
        try:
            self.collection.create_index("url")
            self.collection.create_index("query_id")
            self.collection.create_index("status")
            self.collection.create_index([("url", 1), ("query_id", 1)], unique=True)  # Deduplication
            self.collection.create_index("scraped_at")
            self.collection.create_index("normalized_url", unique=True)  # Global URL deduplication
        except Exception as e:
            print(f"Warning: Could not create URLs collection indexes: {e}")
    
    def _normalize_url(self, url: str) -> str:
        """Normalize URL by removing fragments and query parameters."""
        from urllib.parse import urlparse, urlunparse
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
    
    def add_urls_from_search(self, query_id: str, search_results: List[Dict]) -> List[str]:
        """
        Add URLs from search results with deduplication.
        
        Args:
            query_id: Reference to the query document
            search_results: List of search result dictionaries
            
        Returns:
            List of inserted URL document IDs
        """
        inserted_ids = []
        
        for result in search_results:
            url = result.get('url')
            if not url:
                continue
            
            normalized_url = self._normalize_url(url)
            
            document = {
                "url": url,
                "normalized_url": normalized_url,
                "query_id": ObjectId(query_id),
                "source": "search",
                "status": "pending",
                "content": "",
                "sentiment_score": 0.0,
                "sentiment_label": "neutral",
                "scraped_at": None,
                "depth": 0,
                "title": result.get('title', ''),
                "snippet": result.get('snippet', ''),
                "added_at": datetime.now(timezone.utc)
            }
            
            try:
                # Use upsert to handle deduplication based on normalized URL
                result = self.collection.update_one(
                    {"normalized_url": normalized_url},
                    {"$setOnInsert": document},
                    upsert=True
                )
                
                if result.upserted_id:
                    inserted_ids.append(str(result.upserted_id))
                    
            except Exception as e:
                print(f"Warning: Could not insert URL {url}: {e}")
        
        return inserted_ids
    
    def add_urls_from_crawl(self, query_id: str, crawled_urls: List[Dict], depth: int = 1) -> List[str]:
        """
        Add URLs discovered during crawling.
        
        Args:
            query_id: Reference to the query document
            crawled_urls: List of URLs discovered during crawling
            depth: Crawl depth
            
        Returns:
            List of inserted URL document IDs
        """
        inserted_ids = []
        
        for url_info in crawled_urls:
            url = url_info.get('url')
            if not url:
                continue
            
            normalized_url = self._normalize_url(url)
            
            document = {
                "url": url,
                "normalized_url": normalized_url,
                "query_id": ObjectId(query_id),
                "source": "crawl",
                "status": "pending",
                "content": "",
                "sentiment_score": 0.0,
                "sentiment_label": "neutral",
                "scraped_at": None,
                "depth": depth,
                "title": url_info.get('text', ''),
                "snippet": url_info.get('context', ''),
                "added_at": datetime.now(timezone.utc)
            }
            
            try:
                # Use upsert to handle deduplication based on normalized URL
                result = self.collection.update_one(
                    {"normalized_url": normalized_url},
                    {"$setOnInsert": document},
                    upsert=True
                )
                
                if result.upserted_id:
                    inserted_ids.append(str(result.upserted_id))
                    
            except Exception as e:
                print(f"Warning: Could not insert crawled URL {url}: {e}")
        
        return inserted_ids
    
    def url_exists(self, normalized_url: str) -> bool:
        """
        Check if a normalized URL already exists in the database.
        
        Args:
            normalized_url: The normalized URL to check
            
        Returns:
            True if URL exists, False otherwise
        """
        return self.collection.count_documents({"normalized_url": normalized_url}, limit=1) > 0
    
    def batch_check_urls_exist(self, normalized_urls: List[str]) -> Dict[str, bool]:
        """
        Check multiple URLs for existence in a single database query.
        
        Args:
            normalized_urls: List of normalized URLs to check
            
        Returns:
            Dictionary mapping URL to existence status
        """
        if not normalized_urls:
            return {}
        
        # Query database for all URLs in batch
        existing_docs = self.collection.find(
            {"normalized_url": {"$in": normalized_urls}},
            {"normalized_url": 1}
        )
        
        existing_urls = {doc["normalized_url"] for doc in existing_docs}
        
        # Return mapping of all URLs to their existence status
        return {url: url in existing_urls for url in normalized_urls}
    
    def get_pending_urls(self, query_id: str = None, limit: int = 50) -> List[Dict]:
        """
        Get pending URLs for processing.
        
        Args:
            query_id: Optional filter by query ID
            limit: Maximum number of URLs to return
            
        Returns:
            List of pending URL documents
        """
        filter_dict = {"status": "pending"}
        if query_id:
            filter_dict["query_id"] = ObjectId(query_id)
        
        cursor = self.collection.find(filter_dict).sort("added_at", 1).limit(limit)
        results = []
        
        for doc in cursor:
            doc["_id"] = str(doc["_id"])
            doc["query_id"] = str(doc["query_id"])
            results.append(doc)
        
        return results
    
    def update_url_content(self, url_id: str, content: str, status: str = "processed"):
        """
        Update URL with scraped content.
        
        Args:
            url_id: URL document ID
            content: Scraped content
            status: New status (processed, failed)
        """
        self.collection.update_one(
            {"_id": ObjectId(url_id)},
            {
                "$set": {
                    "content": content,
                    "status": status,
                    "scraped_at": datetime.now(timezone.utc)
                }
            }
        )
    
    def update_url_sentiment(self, url_id: str, sentiment_score: float, sentiment_label: str):
        """
        Update URL with sentiment analysis results.
        
        Args:
            url_id: URL document ID
            sentiment_score: Sentiment score (-1 to 1)
            sentiment_label: Sentiment label (positive, negative, neutral)
        """
        self.collection.update_one(
            {"_id": ObjectId(url_id)},
            {
                "$set": {
                    "sentiment_score": sentiment_score,
                    "sentiment_label": sentiment_label
                }
            }
        )
    
    def mark_url_failed(self, url_id: str, error: str = None):
        """
        Mark URL as failed to process.
        
        Args:
            url_id: URL document ID
            error: Optional error message
        """
        update_data = {
            "status": "failed",
            "scraped_at": datetime.now(timezone.utc)
        }
        
        if error:
            update_data["error"] = error
        
        self.collection.update_one(
            {"_id": ObjectId(url_id)},
            {"$set": update_data}
        )
    
    def get_urls_by_query(self, query_id: str, status: str = None, limit: int = 100) -> List[Dict]:
        """
        Get URLs for a specific query.
        
        Args:
            query_id: Query document ID
            status: Optional status filter
            limit: Maximum number of URLs to return
            
        Returns:
            List of URL documents
        """
        filter_dict = {"query_id": ObjectId(query_id)}
        if status:
            filter_dict["status"] = status
        
        cursor = self.collection.find(filter_dict).sort("added_at", 1).limit(limit)
        results = []
        
        for doc in cursor:
            doc["_id"] = str(doc["_id"])
            doc["query_id"] = str(doc["query_id"])
            results.append(doc)
        
        return results
    
    def get_query_stats(self, query_id: str) -> Dict:
        """
        Get statistics for a query's URLs.
        
        Args:
            query_id: Query document ID
            
        Returns:
            Dictionary with statistics
        """
        pipeline = [
            {"$match": {"query_id": ObjectId(query_id)}},
            {
                "$group": {
                    "_id": None,
                    "total_urls": {"$sum": 1},
                    "pending": {"$sum": {"$cond": [{"$eq": ["$status", "pending"]}, 1, 0]}},
                    "processed": {"$sum": {"$cond": [{"$eq": ["$status", "processed"]}, 1, 0]}},
                    "failed": {"$sum": {"$cond": [{"$eq": ["$status", "failed"]}, 1, 0]}},
                    "avg_sentiment": {"$avg": "$sentiment_score"}
                }
            }
        ]
        
        result = list(self.collection.aggregate(pipeline))
        if result:
            stats = result[0]
            del stats["_id"]
            return stats
        
        return {
            "total_urls": 0,
            "pending": 0,
            "processed": 0,
            "failed": 0,
            "avg_sentiment": 0.0
        }
    
    def close(self):
        """Close the database connection."""
        if self.db_manager:
            self.db_manager.close()