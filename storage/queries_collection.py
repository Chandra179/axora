#!/usr/bin/env python3
"""
Queries collection operations for the simplified architecture.
"""

from typing import List, Dict, Optional
from datetime import datetime, timezone
from bson import ObjectId
from .database import DatabaseManager


class QueriesCollection:
    """Handler for queries collection operations."""
    
    def __init__(self, db_manager: DatabaseManager = None):
        """
        Initialize queries collection handler.
        
        Args:
            db_manager: DatabaseManager instance. If None, creates a new one.
        """
        self.db_manager = db_manager or DatabaseManager()
        self.collection = self.db_manager.get_collection("queries")
        
        # Create indexes for better performance
        self._ensure_indexes()
    
    def _ensure_indexes(self):
        """Create indexes for better performance."""
        try:
            self.collection.create_index("timestamp")
            self.collection.create_index("question")
            self.collection.create_index("status")
        except Exception as e:
            print(f"Warning: Could not create queries collection indexes: {e}")
    
    def create_query(self, question: str) -> str:
        """
        Create a new query entry.
        
        Args:
            question: The search question/query
            
        Returns:
            The inserted query ID as string
        """
        document = {
            "question": question,
            "timestamp": datetime.now(timezone.utc),
            "status": "pending",
            "total_urls": 0,
            "processed_urls": 0,
            "avg_sentiment": 0.0,
            "summary": ""
        }
        
        result = self.collection.insert_one(document)
        return str(result.inserted_id)
    
    def update_query_status(self, query_id: str, status: str):
        """
        Update query status.
        
        Args:
            query_id: Query document ID
            status: New status (pending, processing, completed)
        """
        self.collection.update_one(
            {"_id": ObjectId(query_id)},
            {"$set": {"status": status}}
        )
    
    def update_query_stats(self, query_id: str, total_urls: int, processed_urls: int, 
                          avg_sentiment: float = None, summary: str = None):
        """
        Update query statistics.
        
        Args:
            query_id: Query document ID
            total_urls: Total number of URLs found
            processed_urls: Number of URLs processed
            avg_sentiment: Average sentiment score
            summary: Summary of the query results
        """
        update_data = {
            "total_urls": total_urls,
            "processed_urls": processed_urls
        }
        
        if avg_sentiment is not None:
            update_data["avg_sentiment"] = avg_sentiment
        
        if summary is not None:
            update_data["summary"] = summary
        
        self.collection.update_one(
            {"_id": ObjectId(query_id)},
            {"$set": update_data}
        )
    
    def get_query(self, query_id: str) -> Optional[Dict]:
        """
        Get a specific query by ID.
        
        Args:
            query_id: Query document ID
            
        Returns:
            Query document or None if not found
        """
        doc = self.collection.find_one({"_id": ObjectId(query_id)})
        if doc:
            doc["_id"] = str(doc["_id"])
        return doc
    
    def get_queries(self, status: str = None, limit: int = 100) -> List[Dict]:
        """
        Get queries with optional status filter.
        
        Args:
            status: Optional status filter
            limit: Maximum number of queries to return
            
        Returns:
            List of query documents
        """
        filter_dict = {}
        if status:
            filter_dict["status"] = status
        
        cursor = self.collection.find(filter_dict).sort("timestamp", -1).limit(limit)
        results = []
        
        for doc in cursor:
            doc["_id"] = str(doc["_id"])
            results.append(doc)
        
        return results
    
    def get_pending_queries(self, limit: int = 10) -> List[Dict]:
        """
        Get pending queries for processing.
        
        Args:
            limit: Maximum number of queries to return
            
        Returns:
            List of pending query documents
        """
        return self.get_queries(status="pending", limit=limit)
    
    def close(self):
        """Close the database connection."""
        if self.db_manager:
            self.db_manager.close()