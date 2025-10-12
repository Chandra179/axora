import structlog
import requests
from urllib.parse import urlparse

logger = structlog.get_logger(__name__)

# File extensions to skip
SKIP_EXTENSIONS = (
    '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.svg', '.webp',  # Images
    '.mp4', '.avi', '.mov', '.wmv', '.flv', '.mkv',            # Videos
    '.mp3', '.wav', '.ogg', '.flac',                           # Audio
    '.zip', '.rar', '.tar', '.gz', '.7z',                      # Archives
    '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx'  # Documents (we only want HTML)
)


def validate_url(url: str, timeout: int = 5) -> dict:
    try:
        if not url or not isinstance(url, str):
            logger.warning("invalid_url_format", url=url)
            return {
                "valid": False,
                "reason": "Empty or invalid URL"
            }
        
        # Check URL scheme
        parsed = urlparse(url)
        if parsed.scheme not in ['http', 'https']:
            logger.warning("invalid_url_scheme", 
                          url=url,
                          scheme=parsed.scheme)
            return {
                "valid": False,
                "reason": f"Invalid scheme: {parsed.scheme}"
            }
        
        # Check for file extensions we should skip
        path_lower = parsed.path.lower()
        for ext in SKIP_EXTENSIONS:
            if path_lower.endswith(ext):
                logger.info("skipping_file_type", 
                           url=url,
                           extension=ext)
                return {
                    "valid": False,
                    "reason": f"Skipping file type: {ext}"
                }
        
        # Make HEAD request to check content type
        try:
            logger.debug("sending_head_request", url=url)
            response = requests.head(
                url,
                timeout=timeout,
                allow_redirects=True,
                headers={'User-Agent': 'Mozilla/5.0 (compatible; ContentExtractor/1.0)'}
            )
            
            # Get content type
            content_type = response.headers.get('content-type', '').lower()
            
            # Check if it's HTML
            if 'text/html' not in content_type and 'application/xhtml' not in content_type:
                logger.warning("non_html_content", 
                              url=url,
                              content_type=content_type)
                return {
                    "valid": False,
                    "reason": f"Not HTML content: {content_type}"
                }
                
            return {
                "valid": True,
                "reason": "Valid HTML URL"
            }
            
        except requests.Timeout:
            logger.warning("request_timeout", 
                          url=url,
                          timeout_seconds=timeout)
            return {
                "valid": False,
                "reason": "Request timeout"
            }
        
        except requests.ConnectionError as e:
            logger.warning("connection_error", 
                          url=url,
                          error=str(e))
            return {
                "valid": False,
                "reason": "Connection error"
            }
    
    except Exception as e:
        logger.error("url_validation_error", 
                    url=url,
                    error=str(e),
                    exc_info=True)
        return {
            "valid": False,
            "reason": f"Request failed: {str(e)}"
        }