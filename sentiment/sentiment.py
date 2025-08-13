#!/usr/bin/env python3
"""
Modular sentiment analysis system for scraped data.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.append(os.path.dirname(os.path.abspath(__file__)))

from typing import List, Dict
from storage.database import DatabaseManager
from storage.urls_collection import URLsCollection
from vader import VaderSentimentAnalyzer


class SentimentAnalyzer:
    """Main sentiment analysis class that works with scraped data."""
    
    def __init__(self, model_name: str = 'vader', db_connection_string: str = None):
        """
        Initialize sentiment analyzer.
        
        Args:
            model_name: Name of sentiment model to use ('vader', etc.)
            db_connection_string: MongoDB connection string
        """
        self.model_name = model_name
        self.db_manager = DatabaseManager(db_connection_string)
        self.urls_collection = URLsCollection(self.db_manager)
        
        # Initialize sentiment model
        self.model = self._load_model(model_name)
    
    def _load_model(self, model_name: str):
        """Load the specified sentiment analysis model."""
        if model_name.lower() == 'vader':
            return VaderSentimentAnalyzer()
        else:
            raise ValueError(f"Unsupported sentiment model: {model_name}")
    
    def analyze_scraped_data(self, query_id: str = None, limit: int = 50) -> int:
        """
        Analyze sentiment of scraped data from URLs collection and update URLs directly.
        
        Args:
            query_id: Optional query ID to filter URLs
            limit: Maximum number of URLs to process
            
        Returns:
            Number of URLs processed
        """
        # Get processed URLs with content that don't have sentiment analysis yet
        filter_dict = {
            "status": "processed", 
            "content": {"$ne": ""}, 
            "$or": [
                {"sentiment_score": {"$exists": False}},
                {"sentiment_score": 0.0, "sentiment_label": "neutral"}  # Default values
            ]
        }
        
        if query_id:
            filter_dict["query_id"] = query_id
        
        processed_urls = list(self.urls_collection.collection.find(filter_dict).limit(limit))
        
        if not processed_urls:
            print("No URLs found that need sentiment analysis")
            return 0
        
        print(f"Analyzing sentiment for {len(processed_urls)} URLs...")
        processed_count = 0
        
        # Analyze each URL's scraped content
        for url_doc in processed_urls:
            if not url_doc.get('content'):
                continue
                
            # Create scraped item format for analysis
            scraped_item = {
                'url': url_doc.get('url'),
                'content': url_doc.get('content'),
                'title': url_doc.get('title', ''),
                'status': 'success'
            }
            
            try:
                item_sentiment = self.model.analyze_scraped_item(scraped_item)
                
                # Extract sentiment data
                overall_sentiment_data = item_sentiment.get('overall_sentiment', {})
                sentiment_score = overall_sentiment_data.get('compound', 0.0)
                sentiment_label = overall_sentiment_data.get('sentiment', 'neutral')
                
                # Update the URL document with sentiment data
                self.urls_collection.update_url_sentiment(
                    str(url_doc['_id']),
                    sentiment_score,
                    sentiment_label
                )
                
                processed_count += 1
                print(f"Processed {processed_count}/{len(processed_urls)}: {url_doc.get('url')} -> {sentiment_label} ({sentiment_score:.4f})")
                
            except Exception as e:
                print(f"Error analyzing sentiment for {url_doc.get('url')}: {e}")
                continue
        
        return processed_count
    
    
    def analyze_and_save(self, query_id: str = None, limit: int = 50) -> int:
        """
        Analyze scraped data and update URLs directly.
        
        Args:
            query_id: Optional query ID to filter URLs
            limit: Maximum number of URLs to process
            
        Returns:
            Number of URLs processed
        """
        return self.analyze_scraped_data(query_id, limit)
    
    def get_model_info(self) -> Dict:
        """Get information about the current sentiment model."""
        return self.model.get_model_info()
    
    def change_model(self, model_name: str):
        """Change the sentiment analysis model."""
        self.model = self._load_model(model_name)
        self.model_name = model_name
    
    def close(self):
        """Close database connections."""
        self.urls_collection.close()
        self.db_manager.close()


def main():
    """Example usage of sentiment analyzer."""
    analyzer = SentimentAnalyzer(model_name='vader')
    
    try:
        print(f"Using model: {analyzer.model_name}")
        print("Model info:", analyzer.get_model_info())
        
        # Analyze recent scraped data
        print("\nAnalyzing sentiment of recent scraped data...")
        processed_count = analyzer.analyze_and_save(limit=100)
        
        if processed_count > 0:
            print(f"\nSentiment analysis completed. Processed {processed_count} URLs.")
            print("URLs have been updated with sentiment scores in the database.")
        else:
            print("No scraped data found to analyze.")
            
    finally:
        analyzer.close()


if __name__ == "__main__":
    main()