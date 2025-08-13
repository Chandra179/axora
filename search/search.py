#!/usr/bin/env python3
"""
Search module using DuckDuckGo Lite for web searches.
"""

import sys
import urllib.parse
import urllib.request
from typing import List, Dict
import re
import time
from storage.database import DatabaseManager
from storage.queries_collection import QueriesCollection
from storage.urls_collection import URLsCollection


class DDGLiteSearch:
    """DuckDuckGo Lite search implementation."""
    
    def __init__(self, max_urls: int = 10):
        self.max_urls = max_urls
        self.base_url = "https://lite.duckduckgo.com/lite/"
        self.db_manager = DatabaseManager()
        self.queries_collection = QueriesCollection(self.db_manager)
        self.urls_collection = URLsCollection(self.db_manager)
        
    def search(self, query: str) -> str:
        """
        Search using DuckDuckGo Lite and store results in new architecture.
        
        Args:
            query: Search query string
            
        Returns:
            Query ID for tracking the search and processing
        """
        try:
            # Create query entry first
            query_id = self.queries_collection.create_query(query)
            print(f"Created query with ID: {query_id}")
            
            all_results = []
            start_pos = 0
            page_num = 1
            
            # Keep fetching pages until we have enough results or no more pages
            while len(all_results) < self.max_urls:
                print(f"Fetching page {page_num}...", file=sys.stderr)
                
                # Get results for this page
                page_results = self._fetch_page(query, start_pos)
                
                if not page_results:
                    print(f"No more results found after {page_num-1} pages", file=sys.stderr)
                    break
                
                # Add new results (avoid duplicates)
                existing_urls = {r['url'] for r in all_results}
                new_results = [r for r in page_results if r['url'] not in existing_urls]
                all_results.extend(new_results)
                
                print(f"Page {page_num}: Found {len(new_results)} new results (total: {len(all_results)})", file=sys.stderr)
                
                # If we didn't get any new results, no point in continuing
                if not new_results:
                    print("No new results on this page, stopping", file=sys.stderr)
                    break
                
                start_pos += 10  # DDG Lite uses increments of 10
                page_num += 1
                
                # Small delay between requests to be respectful
                time.sleep(1)
                
                # Safety limit to prevent infinite loops
                if page_num > 20:  # Max 200 results
                    print("Reached maximum page limit (20 pages)", file=sys.stderr)
                    break
            
            # Limit results to max_urls
            limited_results = all_results[:self.max_urls]
            
            if limited_results:
                url_ids = self.urls_collection.add_urls_from_search(query_id, limited_results)
                print(f"Stored {len(url_ids)} URLs for processing")
                
                # Update query with initial stats
                self.queries_collection.update_query_stats(query_id, len(url_ids), 0)
            
            return query_id
            
        except Exception as e:
            print(f"Error performing search: {e}", file=sys.stderr)
            # If we created a query, mark it as failed
            if 'query_id' in locals():
                self.queries_collection.update_query_status(query_id, "failed")
            return None
    
    def _fetch_page(self, query: str, start_pos: int = 0) -> List[Dict[str, str]]:
        """
        Fetch a single page of search results.
        
        Args:
            query: Search query string
            start_pos: Starting position for pagination (0, 10, 20, etc.)
            
        Returns:
            List of search results for this page
        """
        try:
            if start_pos == 0:
                # First page uses GET request
                params = {
                    'q': query,
                    'kl': 'wt-wt'  # No region restriction
                }
                url = self.base_url + '?' + urllib.parse.urlencode(params)
                
                headers = {
                    'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
                }
                
                req = urllib.request.Request(url, headers=headers)
                
                with urllib.request.urlopen(req) as response:
                    html_content = response.read().decode('utf-8')
            else:
                # Subsequent pages use POST request
                data = {
                    'q': query,
                    's': str(start_pos),
                    'kl': 'wt-wt'
                }
                
                headers = {
                    'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36',
                    'Content-Type': 'application/x-www-form-urlencoded'
                }
                
                post_data = urllib.parse.urlencode(data).encode('utf-8')
                req = urllib.request.Request(self.base_url, data=post_data, headers=headers)
                    
                with urllib.request.urlopen(req) as response:
                    html_content = response.read().decode('utf-8')
            
            # Parse results from HTML
            return self._parse_results(html_content)
            
        except Exception as e:
            print(f"Error fetching page (start_pos={start_pos}): {e}", file=sys.stderr)
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
    
    def get_query_status(self, query_id: str) -> Dict:
        """
        Get the status of a query.
        
        Args:
            query_id: Query document ID
            
        Returns:
            Dictionary with query status and statistics
        """
        query = self.queries_collection.get_query(query_id)
        if not query:
            return None
        
        # Get current URL statistics
        stats = self.urls_collection.get_query_stats(query_id)
        
        return {
            'query_id': query_id,
            'question': query['question'],
            'status': query['status'],
            'timestamp': query['timestamp'],
            'total_urls': stats['total_urls'],
            'pending_urls': stats['pending'],
            'processed_urls': stats['processed'],
            'failed_urls': stats['failed'],
            'avg_sentiment': stats['avg_sentiment'],
            'summary': query.get('summary', '')
        }
    
    def list_recent_queries(self, limit: int = 10) -> List[Dict]:
        """
        List recent queries with their status.
        
        Args:
            limit: Maximum number of queries to return
            
        Returns:
            List of query status dictionaries
        """
        queries = self.queries_collection.get_queries(limit=limit)
        result = []
        
        for query in queries:
            status_info = self.get_query_status(query['_id'])
            if status_info:
                result.append(status_info)
        
        return result