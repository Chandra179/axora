#!/usr/bin/env python3
"""
Search module using DuckDuckGo Lite for web searches.
"""

import sys
import urllib.parse
import urllib.request
from typing import List, Dict
import re
from storage.database import SearchResultsDB


class DDGLiteSearch:
    """DuckDuckGo Lite search implementation."""
    
    def __init__(self, max_urls: int = 10):
        self.max_urls = max_urls
        self.base_url = "https://lite.duckduckgo.com/lite/"
        self.db = SearchResultsDB()
        
    def search(self, query: str) -> List[Dict[str, str]]:
        """
        Search using DuckDuckGo Lite and return results.
        
        Args:
            query: Search query string
            
        Returns:
            List of dictionaries containing search results with keys:
            - title: Page title
            - url: URL
            - snippet: Description snippet
        """
        try:
            # Prepare search parameters
            params = {
                'q': query,
                'kl': 'wt-wt'  # No region restriction
            }
            
            # Build URL
            url = self.base_url + '?' + urllib.parse.urlencode(params)
            
            # Make request
            headers = {
                'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
            }
            
            req = urllib.request.Request(url, headers=headers)
            
            with urllib.request.urlopen(req) as response:
                html_content = response.read().decode('utf-8')
                
            # Parse results from HTML
            results = self._parse_results(html_content)
            
            # Limit results to max_urls
            limited_results = results[:self.max_urls]
            
            # Store results
            if limited_results:
                self._store_search_results(query, limited_results)
            
            return limited_results
            
        except Exception as e:
            print(f"Error performing search: {e}", file=sys.stderr)
            return []
    
    def _parse_results(self, html_content: str) -> List[Dict[str, str]]:
        """Parse search results from DuckDuckGo Lite HTML."""
        results = []
        
        # Updated patterns for current DuckDuckGo Lite structure
        # Look for links with specific patterns
        link_pattern = r'<a[^>]*href="([^"]*)"[^>]*>([^<]+)</a>'
        
        # Find result table rows or divs
        result_blocks = re.findall(r'<tr[^>]*>(.*?)</tr>', html_content, re.DOTALL | re.IGNORECASE)
        
        for block in result_blocks:
            # Skip header rows and non-result rows
            if 'result' not in block.lower() and 'http' not in block:
                continue
                
            # Find links in this block
            links = re.findall(link_pattern, block, re.IGNORECASE)
            
            for url, title in links:
                # Skip internal DuckDuckGo links
                if url.startswith('/') and not url.startswith('//'):
                    continue
                    
                # Clean up URL
                if url.startswith('/l/?uddg='):
                    actual_url = urllib.parse.unquote(url.split('uddg=')[1])
                elif url.startswith('//'):
                    actual_url = 'https:' + url
                else:
                    actual_url = url
                
                # Extract snippet if available
                snippet_match = re.search(r'<span[^>]*>(.*?)</span>', block, re.DOTALL | re.IGNORECASE)
                snippet = snippet_match.group(1) if snippet_match else ""
                
                # Clean up title and snippet
                title = re.sub(r'<[^>]+>', '', title).strip()
                snippet = re.sub(r'<[^>]+>', '', snippet).strip()
                
                if actual_url.startswith('http') and title:
                    results.append({
                        'title': title,
                        'url': actual_url,
                        'snippet': snippet
                    })
        
        # If no results found with table parsing, try alternative method
        if not results:
            # Simple link extraction as fallback
            all_links = re.findall(r'<a[^>]*href="([^"]*)"[^>]*>([^<]+)</a>', html_content, re.IGNORECASE)
            
            for url, title in all_links:
                if url.startswith('http') and len(title.strip()) > 3:
                    results.append({
                        'title': title.strip(),
                        'url': url,
                        'snippet': ""
                    })
                    
        return results
    
    def _store_search_results(self, query: str, results: List[Dict[str, str]]):
        """Store search results directly in database."""
        try:
            result_id = self.db.store_search_results(query, results)
            print(f"Successfully stored {len(results)} search results for query: {query} (ID: {result_id})")
        except Exception as e:
            print(f"Error storing search results: {e}", file=sys.stderr)