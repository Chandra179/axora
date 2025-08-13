#!/usr/bin/env python3
"""
Core MongoDB database configuration and connection management.
"""

import os
from pymongo import MongoClient
from pymongo.database import Database


class DatabaseManager:
    """Core MongoDB database connection manager."""
    
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
    
    def get_collection(self, collection_name: str):
        """Get a specific collection from the database."""
        return self.db[collection_name]
    
    def close(self):
        """Close the database connection."""
        self.client.close()