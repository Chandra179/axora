#!/usr/bin/env python3
"""
VADER sentiment analysis implementation.
"""

from typing import Dict, List
from vaderSentiment.vaderSentiment import SentimentIntensityAnalyzer


class VaderSentimentAnalyzer:
    """VADER-based sentiment analysis implementation."""
    
    def __init__(self):
        """Initialize VADER sentiment analyzer."""
        self.analyzer = SentimentIntensityAnalyzer()
    
    def analyze_text(self, text: str) -> Dict:
        """
        Analyze sentiment of a single text.
        
        Args:
            text: Text to analyze
            
        Returns:
            Dictionary containing sentiment scores and classification
        """
        if not text or not text.strip():
            return {
                'compound': 0.0,
                'pos': 0.0,
                'neu': 1.0,
                'neg': 0.0,
                'sentiment': 'neutral'
            }
        
        scores = self.analyzer.polarity_scores(text)
        
        # Classify sentiment based on compound score
        if scores['compound'] >= 0.05:
            sentiment = 'positive'
        elif scores['compound'] <= -0.05:
            sentiment = 'negative'
        else:
            sentiment = 'neutral'
        
        return {
            'compound': scores['compound'],
            'pos': scores['pos'],
            'neu': scores['neu'], 
            'neg': scores['neg'],
            'sentiment': sentiment
        }
    
    def analyze_scraped_item(self, scraped_item: Dict) -> Dict:
        """
        Analyze sentiment of a scraped data item.
        
        Args:
            scraped_item: Dictionary containing scraped data
            
        Returns:
            Dictionary with sentiment analysis for different text fields
        """
        sentiment_results = {}
        
        # Analyze title
        title = scraped_item.get('title', '') or scraped_item.get('original_title', '')
        if title:
            sentiment_results['title_sentiment'] = self.analyze_text(title)
        
        # Analyze description/snippet
        description = scraped_item.get('description', '') or scraped_item.get('original_snippet', '')
        if description:
            sentiment_results['description_sentiment'] = self.analyze_text(description)
        
        # Analyze content
        content = scraped_item.get('content', '')
        if content:
            sentiment_results['content_sentiment'] = self.analyze_text(content)
        
        # Combined sentiment (title + description + content)
        combined_text = ' '.join([
            title,
            description,
            content[:1000]  # Limit content to avoid too much text
        ]).strip()
        
        if combined_text:
            sentiment_results['overall_sentiment'] = self.analyze_text(combined_text)
        
        return sentiment_results
    
    def get_model_info(self) -> Dict:
        """Get information about this sentiment analysis model."""
        return {
            'name': 'VADER',
            'description': 'Valence Aware Dictionary and sEntiment Reasoner',
            'version': '3.3.2',
            'suitable_for': ['social_media', 'general_text', 'mixed_sentiment'],
            'output_format': {
                'compound': 'Overall sentiment score (-1 to 1)',
                'pos': 'Positive sentiment ratio (0 to 1)',
                'neu': 'Neutral sentiment ratio (0 to 1)',
                'neg': 'Negative sentiment ratio (0 to 1)',
                'sentiment': 'Classification (positive/negative/neutral)'
            }
        }