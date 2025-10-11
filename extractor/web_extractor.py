"""
Web Content Extraction Module
Extracts main content from HTML pages using trafilatura
"""

import logging
import trafilatura
import json
from typing import Optional

logger = logging.getLogger(__name__)


def extract_content(url: str) -> Optional[dict]:
    """
    Extract main content and metadata from URL
    
    Args:
        url: URL to extract content from
        
    Returns:
        dict or None: {
            "url": str,
            "title": str,
            "author": str or None,
            "date": str or None,
            "text": str,              # Clean main content
            "raw_text": str,          # Content with structure markers
            "language": str,
            "word_count": int,
            "char_count": int,
            "html_length": int,       # Original HTML length
            "extraction_success": bool
        }
    """
    try:
        logger.info(f"Downloading content from: {url}")
        
        # Download HTML
        downloaded = trafilatura.fetch_url(url)
        
        if not downloaded:
            logger.warning(f"Failed to download content from {url}")
            return None
        
        html_length = len(downloaded)
        logger.info(f"Downloaded {html_length} bytes of HTML")
        
        # Extract content with all metadata
        result = trafilatura.extract(
            downloaded,
            output_format='json',
            include_comments=False,      # Skip comment sections
            include_tables=True,         # Keep tables
            with_metadata=True,          # Extract metadata
            url=url
        )
        
        if not result:
            logger.warning(f"Trafilatura extraction failed for {url}")
            return None
        
        # Parse JSON result
        data = json.loads(result)
        
        logger.info(f"data: {data}")
        
        # Get main content
        text = data.get('raw_text', '') or ''
        
        # Calculate metrics
        word_count = len(text.split())
        char_count = len(text)
        
        logger.info(f"Extracted {word_count} words, {char_count} characters")
        
        return {
            "url": url,
            "title": (data.get('title') or '').strip(),
            "author": (data.get('author') or '').strip() or None,
            "date": (data.get('date') or '').strip() or None,
            "text": text.strip(),
            "language": data.get('language', 'unknown'),
            "word_count": word_count,
            "char_count": char_count,
            "html_length": html_length,
            "extraction_success": True
        }
        
    except json.JSONDecodeError as e:
        logger.error(f"Failed to parse extraction result for {url}: {e}")
        return None
    except Exception as e:
        logger.error(f"Extraction error for {url}: {e}")
        return None