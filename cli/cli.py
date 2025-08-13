#!/usr/bin/env python3
"""
Interactive CLI for the search functionality.
"""

import sys
import os
import argparse

# Add the parent directory to the path to import search module
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from search.search import DDGLiteSearch


class SearchCLI:
    """Interactive CLI for search functionality."""
    
    def __init__(self, max_urls: int = 10):
        self.searcher = DDGLiteSearch(max_urls=max_urls)
        self.max_urls = max_urls
        
    def run_interactive(self):
        """Run the interactive CLI loop."""
        print(f"Interactive Search CLI (max URLs: {self.max_urls})")
        print("Type 'help' for commands, 'quit' to exit")
        
        while True:
            try:
                query = input("\n> ").strip()
                
                if not query:
                    continue
                    
                if query.lower() in ['quit', 'exit', 'q']:
                    print("Goodbye!")
                    break
                    
                elif query.lower() in ['help', 'h']:
                    self.show_help()
                    
                elif query.startswith('set max_urls '):
                    try:
                        new_max = int(query.split()[-1])
                        self.max_urls = new_max
                        self.searcher.max_urls = new_max
                        print(f"Max URLs set to: {new_max}")
                    except ValueError:
                        print("Invalid number. Usage: set max_urls <number>")
                        
                elif query.startswith('search '):
                    search_query = query[7:]  # Remove 'search ' prefix
                    self.perform_search(search_query)
                    
                elif query.startswith('status '):
                    query_id = query[7:]  # Remove 'status ' prefix
                    self.show_status(query_id)
                    
                elif query.lower() in ['list', 'ls']:
                    self.list_queries()
                    
                else:
                    # Treat any other input as a search query
                    self.perform_search(query)
                    
            except KeyboardInterrupt:
                print("\nUse 'quit' to exit")
            except EOFError:
                print("\nGoodbye!")
                break
    
    def show_help(self):
        """Show help information."""
        help_text = """
Available commands:
  help, h              Show this help message
  quit, exit, q        Exit the CLI
  set max_urls <num>   Set maximum number of URLs to fetch
  status <query_id>    Check the processing status of a query
  list, ls             List recent queries and their status

Examples:
  > python programming
  > search artificial intelligence
  > status 507f1f77bcf86cd799439011
  > list
  > set max_urls 20
  > help
  > quit

Note: Searches now queue URLs for background processing by the worker.
Use 'status <query_id>' to monitor progress and results.
        """
        print(help_text)
    
    def perform_search(self, query: str):
        """Perform a search and display results."""
        if not query.strip():
            print("Please enter a search query.")
            return
            
        print(f"\nSearching for: '{query}' (max URLs: {self.max_urls})...")
        
        query_id = self.searcher.search(query)
        
        if not query_id:
            print("Search failed.")
            return
        
        print(f"Search completed! Query ID: {query_id}")
        print("URLs have been queued for processing by the worker.")
        
        # Show initial status
        status = self.searcher.get_query_status(query_id)
        if status:
            print(f"Status: {status['status']}")
            print(f"Total URLs queued: {status['total_urls']}")
            print(f"Processed: {status['processed_urls']}/{status['total_urls']}")
            
            if status['processed_urls'] > 0:
                print(f"Average sentiment: {status['avg_sentiment']:.2f}")
        
        print("\nUse 'status <query_id>' to check processing progress.")
    
    def show_status(self, query_id: str):
        """Show the status of a specific query."""
        if not query_id.strip():
            print("Please provide a query ID.")
            return
        
        status = self.searcher.get_query_status(query_id)
        
        if not status:
            print(f"Query {query_id} not found.")
            return
        
        print(f"\nQuery: {status['question']}")
        print(f"ID: {status['query_id']}")
        print(f"Status: {status['status']}")
        print(f"Created: {status['timestamp']}")
        print(f"URLs: {status['processed_urls']}/{status['total_urls']} processed")
        
        if status['failed_urls'] > 0:
            print(f"Failed URLs: {status['failed_urls']}")
        
        if status['processed_urls'] > 0:
            print(f"Average sentiment: {status['avg_sentiment']:.2f}")
        
        if status['summary']:
            print(f"Summary: {status['summary']}")
        
        if status['pending_urls'] > 0:
            print(f"\n{status['pending_urls']} URLs still pending processing.")
    
    def list_queries(self):
        """List recent queries and their status."""
        queries = self.searcher.list_recent_queries(limit=10)
        
        if not queries:
            print("No queries found.")
            return
        
        print("\nRecent queries:")
        print("-" * 80)
        
        for i, query in enumerate(queries, 1):
            print(f"{i}. {query['question'][:50]}...")
            print(f"   ID: {query['query_id']}")
            print(f"   Status: {query['status']} | URLs: {query['processed_urls']}/{query['total_urls']}")
            
            if query['processed_urls'] > 0:
                print(f"   Avg sentiment: {query['avg_sentiment']:.2f}")
            
            print()


def main():
    """Main function for CLI."""
    parser = argparse.ArgumentParser(description='Interactive Search CLI')
    parser.add_argument('--max-urls', '-m', type=int, default=10,
                       help='Maximum number of URLs to fetch (default: 10)')
    
    args = parser.parse_args()
    
    cli = SearchCLI(max_urls=args.max_urls)
    cli.run_interactive()


if __name__ == '__main__':
    main()