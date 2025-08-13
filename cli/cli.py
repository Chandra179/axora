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

Examples:
  > python programming
  > search artificial intelligence
  > set max_urls 20
  > help
  > quit

Storage: Search results are automatically saved to MongoDB via the storage API.
        """
        print(help_text)
    
    def perform_search(self, query: str):
        """Perform a search and display results."""
        if not query.strip():
            print("Please enter a search query.")
            return
            
        print(f"\nSearching for: '{query}'...")
        
        results = self.searcher.search(query)
        
        if not results:
            print("No results found.")
            return
        
        for i, result in enumerate(results, 1):
            print(f"\n{i}. {result['title']}")
            print(f"{result['url']}")
            if result['snippet']:
                print(f"{result['snippet']}")
        
        print(f"\nFound {len(results)} results")
    


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