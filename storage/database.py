#!/usr/bin/env python3
"""
MongoDB database connection and operations for search results storage.
"""

import os
from pymongo import MongoClient
from pymongo.database import Database
from pymongo.collection import Collection
from typing import List, Dict
from datetime import datetime, timezone


class SearchResultsDB:
    """MongoDB database handler for search results."""
    
    def __init__(self, connection_string: str = None):
        """
        Initialize MongoDB connection.
        
        Args:
            connection_string: MongoDB connection string. If None, uses default localhost.
        """
        if connection_string is None:
            connection_string = os.getenv('MONGODB_URL', 'mongodb://admin:admin@localhost:27017/axora_search?authSource=admin')
        
        self.client = MongoClient(connection_string)
        self.db: Database = self.client.axora_search
        self.collection: Collection = self.db.search_results
        
        # Create indexes for better performance (handle auth errors gracefully)
        self._ensure_indexes()
    
    def _ensure_indexes(self):
        """Create indexes for better performance (handle auth errors gracefully)."""
        try:
            self.collection.create_index("timestamp")
            self.collection.create_index("query")
        except Exception as e:
            print(f"Warning: Could not create database indexes: {e}")
            print("Database operations may be slower without indexes.")
        
    def store_search_results(self, query: str, results: List[Dict[str, str]]) -> str:
        """
        Store search results in MongoDB.
        
        Args:
            query: The search query
            results: List of search results
            
        Returns:
            The inserted document ID as string
        """
        document = {
            "timestamp": datetime.now(timezone.utc),
            "query": query,
            "result_count": len(results),
            "results": results
        }
        
        result = self.collection.insert_one(document)
        return str(result.inserted_id)
    
    def get_search_results(self, query: str = None, limit: int = 100) -> List[Dict]:
        """
        Retrieve search results from MongoDB.
        
        Args:
            query: Optional query filter. If None, returns all results.
            limit: Maximum number of results to return
            
        Returns:
            List of search result documents
        """
        filter_dict = {}
        if query:
            filter_dict["query"] = {"$regex": query, "$options": "i"}
        
        cursor = self.collection.find(filter_dict).sort("timestamp", -1).limit(limit)
        results = []
        
        for doc in cursor:
            doc["_id"] = str(doc["_id"])  # Convert ObjectId to string
            results.append(doc)
            
        return results
    
    def get_recent_searches(self, hours: int = 24, limit: int = 50) -> List[Dict]:
        """
        Get recent search results within specified hours.
        
        Args:
            hours: Number of hours to look back
            limit: Maximum number of results
            
        Returns:
            List of recent search results
        """
        from datetime import timedelta
        
        cutoff_time = datetime.now(timezone.utc) - timedelta(hours=hours)
        filter_dict = {"timestamp": {"$gte": cutoff_time}}
        
        cursor = self.collection.find(filter_dict).sort("timestamp", -1).limit(limit)
        results = []
        
        for doc in cursor:
            doc["_id"] = str(doc["_id"])
            results.append(doc)
            
        return results
    
    def get_stats(self) -> Dict:
        """
        Get database statistics.
        
        Returns:
            Dictionary with database stats
        """
        total_searches = self.collection.count_documents({})
        
        # Get top queries
        pipeline = [
            {"$group": {"_id": "$query", "count": {"$sum": 1}}},
            {"$sort": {"count": -1}},
            {"$limit": 10}
        ]
        top_queries = list(self.collection.aggregate(pipeline))
        
        return {
            "total_searches": total_searches,
            "top_queries": top_queries
        }
    
    def close(self):
        """Close the database connection."""
        self.client.close()