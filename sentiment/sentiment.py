#!/usr/bin/env python3
"""
Modular sentiment analysis system for scraped data.
"""

import sys
import os
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from typing import List, Dict
from datetime import datetime, timezone
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
        
        # Collection to store sentiment results
        self.sentiment_collection = self.db_manager.get_collection("sentiment_analysis")
    
    def _load_model(self, model_name: str):
        """Load the specified sentiment analysis model."""
        if model_name.lower() == 'vader':
            return VaderSentimentAnalyzer()
        else:
            raise ValueError(f"Unsupported sentiment model: {model_name}")
    
    def analyze_scraped_data(self, query_id: str = None, limit: int = 50) -> List[Dict]:
        """
        Analyze sentiment of scraped data from URLs collection.
        
        Args:
            query_id: Optional query ID to filter URLs
            limit: Maximum number of URLs to process
            
        Returns:
            List of sentiment analysis results
        """
        # Get processed URLs with content
        if query_id:
            processed_urls = self.urls_collection.get_urls_by_query(query_id, status="processed", limit=limit)
        else:
            # Get all processed URLs
            processed_urls = self.urls_collection.collection.find({"status": "processed", "content": {"$ne": ""}}).limit(limit)
            processed_urls = list(processed_urls)
            # Convert ObjectIds to strings for consistency
            for url_doc in processed_urls:
                url_doc["_id"] = str(url_doc["_id"])
                url_doc["query_id"] = str(url_doc["query_id"])
        
        if not processed_urls:
            return []
        
        results = []
        
        # Group URLs by query_id for analysis
        query_groups = {}
        for url_doc in processed_urls:
            q_id = url_doc['query_id']
            if q_id not in query_groups:
                query_groups[q_id] = []
            query_groups[q_id].append(url_doc)
        
        for query_id, urls in query_groups.items():
            doc_results = {
                'query_id': query_id,
                'timestamp': datetime.now(timezone.utc),
                'model': self.model_name,
                'scraped_items_analyzed': [],
                'summary': {
                    'total_items': 0,
                    'positive': 0,
                    'negative': 0,
                    'neutral': 0
                }
            }
            
            # Analyze each URL's scraped content
            for url_doc in urls:
                if not url_doc.get('content'):
                    continue
                
                # Create scraped item format for analysis
                scraped_item = {
                    'url': url_doc.get('url'),
                    'content': url_doc.get('content'),
                    'title': url_doc.get('title', ''),
                    'status': 'success'
                }
                
                item_sentiment = self.model.analyze_scraped_item(scraped_item)
                
                analyzed_item = {
                    'url': url_doc.get('url'),
                    'title': url_doc.get('title', ''),
                    'sentiment_analysis': item_sentiment
                }
                
                doc_results['scraped_items_analyzed'].append(analyzed_item)
                
                # Update summary based on overall sentiment
                overall_sentiment = item_sentiment.get('overall_sentiment', {}).get('sentiment', 'neutral')
                doc_results['summary']['total_items'] += 1
                doc_results['summary'][overall_sentiment] += 1
                
                # Update the URL document with sentiment data
                self.urls_collection.update_url_sentiment(
                    url_doc['_id'],
                    item_sentiment.get('overall_sentiment', {}).get('compound', 0.0),
                    overall_sentiment
                )
            
            if doc_results['summary']['total_items'] > 0:
                results.append(doc_results)
        
        return results
    
    def save_sentiment_results(self, sentiment_results: List[Dict]) -> List[str]:
        """
        Save sentiment analysis results to database.
        
        Args:
            sentiment_results: List of sentiment analysis results
            
        Returns:
            List of inserted document IDs
        """
        inserted_ids = []
        
        for result in sentiment_results:
            insert_result = self.sentiment_collection.insert_one(result)
            inserted_ids.append(str(insert_result.inserted_id))
        
        return inserted_ids
    
    def get_sentiment_results(self, model_name: str = None, limit: int = 50) -> List[Dict]:
        """
        Retrieve sentiment analysis results from database.
        
        Args:
            model_name: Filter by specific model
            limit: Maximum number of results
            
        Returns:
            List of sentiment results
        """
        filter_dict = {}
        if model_name:
            filter_dict['model'] = model_name
        
        cursor = self.sentiment_collection.find(filter_dict).sort("timestamp", -1).limit(limit)
        results = []
        
        for doc in cursor:
            doc["_id"] = str(doc["_id"])
            results.append(doc)
        
        return results
    
    def analyze_and_save(self, query_id: str = None, limit: int = 50) -> List[str]:
        """
        Analyze scraped data and save results in one step.
        
        Args:
            query_id: Optional query ID to filter URLs
            limit: Maximum number of URLs to process
            
        Returns:
            List of inserted sentiment result document IDs
        """
        sentiment_results = self.analyze_scraped_data(query_id, limit)
        return self.save_sentiment_results(sentiment_results)
    
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
        sentiment_ids = analyzer.analyze_and_save(limit=100)
        
        if sentiment_ids:
            print(f"Sentiment analysis completed. Saved {len(sentiment_ids)} results.")
            
            # Display recent results
            print("\nRecent sentiment results:")
            results = analyzer.get_sentiment_results(limit=100)
            
            for result in results:
                print(f"\nQuery ID: {result['query_id']}")
                summary = result['summary']
                print(f"Items analyzed: {summary['total_items']}")
                print(f"Positive: {summary['positive']}, Negative: {summary['negative']}, Neutral: {summary['neutral']}")
                
                # Show first few items
                for item in result['scraped_items_analyzed'][:2]:
                    overall = item['sentiment_analysis'].get('overall_sentiment', {})
                    print(f"  URL: {item['url']}")
                    print(f"  Overall sentiment: {overall.get('sentiment', 'N/A')} (score: {overall.get('compound', 'N/A')})")
        else:
            print("No scraped data found to analyze.")
            
    finally:
        analyzer.close()


if __name__ == "__main__":
    main()