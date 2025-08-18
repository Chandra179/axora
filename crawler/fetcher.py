"""
Encapsulates actual HTTP fetch logic (using httpx, requests, or headless browser).
Handles redirects, timeouts, retries, headers.
Keeps network code separate from orchestration.
"""