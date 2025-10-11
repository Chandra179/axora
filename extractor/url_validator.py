"""
URL Validation Module
Validates URLs before processing to prevent wasting resources
"""

import logging
import requests
from urllib.parse import urlparse

logger = logging.getLogger(__name__)

# File extensions to skip
SKIP_EXTENSIONS = (
    '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.svg', '.webp',  # Images
    '.mp4', '.avi', '.mov', '.wmv', '.flv', '.mkv',            # Videos
    '.mp3', '.wav', '.ogg', '.flac',                           # Audio
    '.zip', '.rar', '.tar', '.gz', '.7z',                      # Archives
    '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx'  # Documents (we only want HTML)
)


def validate_url(url: str, timeout: int = 5) -> dict:
    """
    Validate URL and check if it's processable HTML content
    
    Args:
        url: URL to validate
        timeout: Request timeout in seconds
        
    Returns:
        dict: {
            "valid": bool,
            "reason": str,
            "content_type": str or None,
            "final_url": str or None
        }
    """
    # Basic format check
    if not url or not isinstance(url, str):
        return {
            "valid": False,
            "reason": "Empty or invalid URL",
            "content_type": None,
            "final_url": None
        }
    
    # Check URL scheme
    parsed = urlparse(url)
    if parsed.scheme not in ['http', 'https']:
        return {
            "valid": False,
            "reason": f"Invalid scheme: {parsed.scheme}",
            "content_type": None,
            "final_url": None
        }
    
    # Check for file extensions we should skip
    path_lower = parsed.path.lower()
    for ext in SKIP_EXTENSIONS:
        if path_lower.endswith(ext):
            return {
                "valid": False,
                "reason": f"Skipping file type: {ext}",
                "content_type": None,
                "final_url": None
            }
    
    # Make HEAD request to check content type
    try:
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
            return {
                "valid": False,
                "reason": f"Not HTML content: {content_type}",
                "content_type": content_type,
                "final_url": response.url
            }
        
        # Success
        return {
            "valid": True,
            "reason": "Valid HTML URL",
            "content_type": content_type,
            "final_url": response.url
        }
        
    except requests.Timeout:
        return {
            "valid": False,
            "reason": "Request timeout",
            "content_type": None,
            "final_url": None
        }
    except requests.ConnectionError:
        return {
            "valid": False,
            "reason": "Connection error",
            "content_type": None,
            "final_url": None
        }
    except Exception as e:
        return {
            "valid": False,
            "reason": f"Request failed: {str(e)}",
            "content_type": None,
            "final_url": None
        }