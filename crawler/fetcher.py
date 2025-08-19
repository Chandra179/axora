
import asyncio
import logging
from typing import Optional, Dict
import httpx
from datetime import datetime, timezone
import time

logger = logging.getLogger(__name__)

class FetchResult:
    def __init__(
        self,
        url: str,
        status_code: int,
        content: bytes = b'',
        headers: Dict[str, str] = None,
        final_url: str = None,
        fetch_time: float = 0.0,
        error: str = None,
        content_type: str = None,
        encoding: str = None
    ):
        """Initialize a FetchResult with HTTP response data and metadata."""
        self.url = url
        self.status_code = status_code
        self.content = content
        self.headers = headers or {}
        self.final_url = final_url or url
        self.fetch_time = fetch_time
        self.error = error
        self.content_type = content_type
        self.encoding = encoding
        self.timestamp = datetime.now(timezone.utc)
    
    @property
    def success(self) -> bool:
        """Check if the fetch was successful (no error and 2xx status code)."""
        return self.error is None and 200 <= self.status_code < 300
    
    @property
    def text(self) -> str:
        """Decode the response content to text using detected or fallback encoding."""
        if not self.content:
            return ""
        encoding = self.encoding or 'utf-8'
        try:
            return self.content.decode(encoding)
        except UnicodeDecodeError:
            try:
                return self.content.decode('utf-8', errors='replace')
            except:
                return self.content.decode('latin-1', errors='replace')
    
    @property
    def size(self) -> int:
        """Get the size of the response content in bytes."""
        return len(self.content)


class HTTPFetcher:
    def __init__(self):
        """Initialize the HTTP fetcher with simple settings."""
        self.user_agent = 'SimpleCrawler/1.0'
        self.timeout = 30.0
        self.max_redirects = 5
        self.max_response_size = 10 * 1024 * 1024  # 10MB
        
        headers = {
                'User-Agent': self.user_agent,
                'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
                'Accept-Language': 'en-US,en;q=0.5',
                'Accept-Encoding': 'gzip, deflate',
                'Connection': 'keep-alive',
                'Upgrade-Insecure-Requests': '1',
            }   
        self._client = httpx.AsyncClient(
            timeout=httpx.Timeout(self.timeout),
            follow_redirects=True,
            max_redirects=self.max_redirects,
            headers=headers,
            limits=httpx.Limits(max_connections=20, max_keepalive_connections=10)
        )
        
    async def fetch(self, url: str, headers: Dict[str, str] = None) -> FetchResult:
        """Fetch a URL with and return a FetchResult containing response data."""
        merged_headers = {}
        if headers:
            merged_headers.update(headers)
        
        start_time = time.time()
        
        try:
            response = await self._client.get(url, headers=merged_headers)
            fetch_time = time.time() - start_time
            
            content_length = response.headers.get('content-length')
            if content_length and int(content_length) > self.max_response_size:
                return FetchResult(
                    url=url,
                    status_code=response.status_code,
                    headers=dict(response.headers),
                    final_url=str(response.url),
                    fetch_time=fetch_time,
                    error=f"Content too large: {content_length} bytes > {self.max_response_size} bytes"
                )
            
            content = b''
            content_type = response.headers.get('content-type', '').lower()
            
            if self._should_fetch_content(content_type):
                try:
                    async for chunk in response.aiter_bytes(chunk_size=8192):
                        content += chunk
                        if len(content) > self.max_response_size:
                            logger.warning(f"Response too large for {url}, truncating at {self.max_response_size} bytes")
                            break
                except Exception as e:
                    logger.error(f"Error reading response content for {url}: {e}")
                    return FetchResult(
                        url=url,
                        status_code=response.status_code,
                        headers=dict(response.headers),
                        final_url=str(response.url),
                        fetch_time=fetch_time,
                        error=f"Error reading content: {str(e)}"
                    )
            
            encoding = self._extract_encoding(response.headers, content)
            
            return FetchResult(
                url=url,
                status_code=response.status_code,
                content=content,
                headers=dict(response.headers),
                final_url=str(response.url),
                fetch_time=fetch_time,
                content_type=content_type,
                encoding=encoding
            )
            
        except httpx.TimeoutException as e:
            error = f"Timeout after {self.timeout}s: {str(e)}"
            logger.warning(f"{error} for {url}")
            
        except httpx.ConnectError as e:
            error = f"Connection error: {str(e)}"
            logger.warning(f"{error} for {url}")
            
        except httpx.HTTPStatusError as e:
            fetch_time = time.time() - start_time
            return FetchResult(
                url=url,
                status_code=e.response.status_code,
                headers=dict(e.response.headers),
                final_url=str(e.response.url),
                fetch_time=fetch_time,
                error=f"HTTP {e.response.status_code}"
            )
            
        except Exception as e:
            error = f"Unexpected error: {str(e)}"
            logger.error(f"{error} for {url}")
        
        return FetchResult(
            url=url,
            status_code=0,
            fetch_time=time.time() - start_time,
            error="Max retries exceeded"
        )
    
    def _should_fetch_content(self, content_type: str) -> bool:
        """Determine if content should be fetched based on content type."""
        if not content_type:
            return True
        
        content_type = content_type.lower()
        
        allowed_types = [
            'text/html',
            'text/xml',
            'text/plain',
            'application/xml',
            'application/xhtml+xml',
            'application/rss+xml',
            'application/atom+xml',
            'application/json',
            'application/ld+json'
        ]
        
        for allowed_type in allowed_types:
            if content_type.startswith(allowed_type):
                return True
        
        if content_type.startswith('text/'):
            return True
            
        return False
    
    def _extract_encoding(self, headers: Dict[str, str], content: bytes) -> Optional[str]:
        """Extract character encoding from HTTP headers or HTML content."""
        content_type = headers.get('content-type', '')
        if 'charset=' in content_type.lower():
            try:
                charset_part = content_type.lower().split('charset=')[1].split(';')[0].strip()
                return charset_part
            except:
                pass
        
        if content and len(content) > 100:
            content_str = content[:1024].decode('utf-8', errors='ignore').lower()
            
            if 'charset=' in content_str:
                try:
                    start = content_str.find('charset=') + 8
                    end = content_str.find('"', start)
                    if end == -1:
                        end = content_str.find("'", start)
                    if end == -1:
                        end = content_str.find('>', start)
                    if end == -1:
                        end = start + 20
                    
                    charset = content_str[start:end].strip(' \'">')
                    if charset:
                        return charset
                except:
                    pass
        
        return 'utf-8'

async def create_fetcher() -> HTTPFetcher:
    """Create and initialize an HTTPFetcher instance."""
    return HTTPFetcher()